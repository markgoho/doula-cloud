#!/usr/bin/env bun
// Machine-wide admission control for the heavy step of
// `scripts/hooks/pre-commit` (#936). Wraps a command in an exclusive
// lock so that only one session at a time runs it:
//
//   bun scripts/gate-lock.ts -- bun run --cwd app test:unit:coverage
//
// Why this exists. `app/vite.config.ts` runs its `client` project in
// browser mode, so the gate's `test:unit:coverage` step starts headless
// Chromium and peaks at ~4.9 GB after #935 capped the browser pool.
// Several Claude Code sessions work this repo at once (up to 9
// worktrees, see docs/agents/worktree-flow.md), each free to commit
// whenever it likes. Two of those gates fit on a 24 GB machine whose
// non-repo residents already come to ~10.5 GB; three do not, and about
// 3.5 GB of each gate is fixed cost that no worker count removes. So
// capping one run cannot buy the third concurrent run -- only admission
// control can. See docs/testing.md, "The memory this gate costs".
//
// The rule this is built around: a gate that wedges every commit is
// worse than the memory pressure it prevents. Every path through the
// lock bookkeeping therefore fails OPEN -- an unreadable lock, an
// unreachable git dir, an owner that was killed, a lock older than any
// real run -- and the wrapped command runs anyway. The only thing this
// script is allowed to do to a commit is delay it.
//
// The reclaim decision (`inspectLock`) is pure and exported for direct
// unit testing; the fail-open paths are exercised as a subprocess, the
// same way scripts/testdb-reap.test.ts covers .claude/hooks/testdb-reap.ts.

import { execFileSync, spawn } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

// How long a held lock may go unrefreshed before a waiter takes it. The
// step this guards is ~16s on an idle 14-CPU machine (docs/testing.md),
// and the whole reason the lock exists is that the machine is not idle
// -- a gate queued behind two others, on a box under memory pressure,
// can run several times that. Five minutes is roughly 15x the idle run,
// the same "generously above the slowest plausible real run" reasoning
// as REAP_THRESHOLD_MS in .claude/hooks/testdb-reap.ts. Erring long is
// the cheap direction: reclaiming too early lets two gates run at once,
// which is the exact condition this exists to prevent, while reclaiming
// too late only delays a commit that was going to wait anyway. A killed
// session is usually caught long before this by the liveness check
// below, which needs no threshold at all.
export const STALE_LOCK_MS = 5 * 60 * 1000;

// A commit is an interactive act, so the wait has to be legible while it
// happens. First notice goes out as soon as we know we are queued; after
// that, one line per interval so the session reads as waiting rather
// than hung.
const POLL_INTERVAL_MS = 500;
const WAIT_NOTICE_INTERVAL_MS = 15 * 1000;

const LOCK_DIR_NAME = 'pre-commit-gate.lock';
const OWNER_FILE_NAME = 'owner.json';

// Set this to any non-empty value to run the command with no lock at
// all. Documented in docs/testing.md next to the memory section.
const SKIP_ENV_VAR = 'SKIP_GATE_LOCK';

// Test seam only: relocates the lock so a spec never touches the real
// `.git`. Not documented as a user-facing knob.
const LOCK_DIR_ENV_VAR = 'GATE_LOCK_DIR';

export interface LockOwner {
	pid: number;
	label: string;
	startedAtMs: number;
}

export type LockVerdict = { action: 'wait' } | { action: 'reclaim'; reason: string };

// The one place that decides whether another session's lock may be taken
// away. `heldForMs` comes from the lock directory's own mtime, which
// mkdir sets atomically at creation -- not from the owner file, which is
// written a moment later and is therefore absent during a live holder's
// startup. A missing or unreadable owner (`owner === null`) must never
// be reclaim-on-sight for that reason; it falls through to the age gate,
// which a freshly created lock cannot fail.
export function inspectLock(
	heldForMs: number,
	owner: LockOwner | null,
	isAlive: (pid: number) => boolean,
	staleMs: number = STALE_LOCK_MS
): LockVerdict {
	if (owner && !isAlive(owner.pid)) {
		return { action: 'reclaim', reason: `owner process ${owner.pid} is gone` };
	}
	if (heldForMs > staleMs) {
		return {
			action: 'reclaim',
			reason: `held for ${Math.round(heldForMs / 1000)}s, past the ${Math.round(staleMs / 1000)}s staleness gate`
		};
	}
	return { action: 'wait' };
}

// EPERM means the pid exists and belongs to someone else -- alive. Only
// ESRCH proves it is gone. Anything else (a pid of 0, a malformed owner
// file) is treated as alive, because the age gate will clear it anyway
// and "assume alive" is the side that never runs two gates at once.
export function isProcessAlive(pid: number): boolean {
	if (!Number.isInteger(pid) || pid <= 0) return true;
	try {
		process.kill(pid, 0);
		return true;
	} catch (error) {
		return (error as NodeJS.ErrnoException).code !== 'ESRCH';
	}
}

// Every worktree of this checkout has to contend for the SAME lock, so
// it goes in the git *common* directory (the main checkout's `.git`),
// which `git rev-parse --git-common-dir` answers identically from any
// worktree. A per-worktree `.git` file would give each session its own
// private lock and coordinate nothing.
export function resolveLockDir(): string {
	const override = process.env[LOCK_DIR_ENV_VAR];
	if (override) return override;
	const commonDir = execFileSync('git', ['rev-parse', '--git-common-dir'], {
		encoding: 'utf8',
		stdio: ['ignore', 'pipe', 'pipe']
	}).trim();
	return path.join(path.resolve(commonDir), LOCK_DIR_NAME);
}

function readOwner(lockDir: string): LockOwner | null {
	try {
		const parsed = JSON.parse(fs.readFileSync(path.join(lockDir, OWNER_FILE_NAME), 'utf8')) as LockOwner;
		return typeof parsed?.pid === 'number' ? parsed : null;
	} catch {
		return null; // not written yet, or unreadable -- the age gate decides
	}
}

function describeSelf(): string {
	try {
		return path.basename(execFileSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim());
	} catch {
		return path.basename(process.cwd());
	}
}

// `mkdir` without `recursive` is the atomic test-and-set: exactly one
// caller creates the directory, everyone else gets EEXIST. macOS ships
// no `flock` binary, which is why this is a bun script rather than a
// shell one-liner.
function tryAcquire(lockDir: string): boolean {
	try {
		fs.mkdirSync(lockDir);
	} catch (error) {
		if ((error as NodeJS.ErrnoException).code === 'EEXIST') return false;
		throw error;
	}
	const owner: LockOwner = { pid: process.pid, label: describeSelf(), startedAtMs: Date.now() };
	try {
		fs.writeFileSync(path.join(lockDir, OWNER_FILE_NAME), `${JSON.stringify(owner)}\n`);
	} catch {
		// Held either way -- the owner file is for the waiter's message
		// and its liveness check, not for the mutual exclusion itself.
	}
	return true;
}

// Reclaiming by `rm -rf` on the lock directory is a race: two waiters
// can both judge the same lock stale, the first removes it and takes a
// fresh one, and the second then deletes THAT and takes one too, leaving
// two gates running. Renaming is atomic and single-winner -- the loser
// gets ENOENT and simply goes round the loop again.
function reclaim(lockDir: string): void {
	const parked = `${lockDir}.stale-${Date.now()}-${process.pid}`;
	fs.renameSync(lockDir, parked);
	fs.rmSync(parked, { recursive: true, force: true });
}

function release(lockDir: string): void {
	try {
		const owner = readOwner(lockDir);
		if (owner && owner.pid !== process.pid) return; // already reclaimed; not ours to remove
		fs.rmSync(lockDir, { recursive: true, force: true });
	} catch {
		// A lock we cannot remove is cleared by the next session's age
		// gate. Never let cleanup fail a commit that already passed.
	}
}

const sleep = (ms: number): Promise<void> => new Promise(resolve => setTimeout(resolve, ms));

// Blocks until the lock is ours. Throws only on something genuinely
// unexpected, which main() turns into an unlocked run.
async function acquire(lockDir: string, notify: (message: string) => void): Promise<void> {
	let announcedAtMs = 0;
	for (;;) {
		if (tryAcquire(lockDir)) return;

		let heldForMs: number;
		try {
			heldForMs = Date.now() - fs.statSync(lockDir).mtimeMs;
		} catch {
			continue; // vanished between the mkdir and the stat -- try again
		}
		const owner = readOwner(lockDir);
		const verdict = inspectLock(heldForMs, owner, isProcessAlive);
		if (verdict.action === 'reclaim') {
			try {
				reclaim(lockDir);
				notify(`gate-lock: reclaimed a stale lock (${verdict.reason})`);
			} catch {
				// Another waiter reclaimed it first; go round again.
			}
			continue;
		}

		const now = Date.now();
		if (now - announcedAtMs >= WAIT_NOTICE_INTERVAL_MS) {
			const held = Math.round(heldForMs / 1000);
			const who = owner ? `${owner.label} (pid ${owner.pid})` : 'another session';
			notify(`gate-lock: waiting on ${who}'s test run, running for ${held}s. Nothing is hung. Set ${SKIP_ENV_VAR}=1 to skip the lock.`);
			announcedAtMs = now;
		}
		await sleep(POLL_INTERVAL_MS);
	}
}

function runCommand(argv: string[]): Promise<number> {
	return new Promise((resolve, reject) => {
		const child = spawn(argv[0], argv.slice(1), { stdio: 'inherit' });
		// Forward the interrupt rather than dying under it, so the
		// `finally` in main() still gets to release the lock. Without
		// this, a Ctrl-C during the gate leaves a held lock behind for
		// the age gate to clear five minutes later.
		const forward = (signal: NodeJS.Signals) => () => child.kill(signal);
		const onInt = forward('SIGINT');
		const onTerm = forward('SIGTERM');
		process.on('SIGINT', onInt);
		process.on('SIGTERM', onTerm);
		child.on('error', reject);
		child.on('close', (code, signal) => {
			process.off('SIGINT', onInt);
			process.off('SIGTERM', onTerm);
			resolve(signal ? 1 : (code ?? 1));
		});
	});
}

async function main(argv: string[]): Promise<number> {
	const command = argv[0] === '--' ? argv.slice(1) : argv;
	if (command.length === 0) {
		console.error(`gate-lock: usage: bun scripts/gate-lock.ts -- <command> [args...]`);
		return 2;
	}
	if (process.env[SKIP_ENV_VAR]) return runCommand(command);

	let lockDir: string | null = null;
	try {
		lockDir = resolveLockDir();
		fs.mkdirSync(path.dirname(lockDir), { recursive: true });
		await acquire(lockDir, message => console.error(message));
	} catch (error) {
		// No git dir, an unwritable parent, a lock directory we cannot
		// reason about -- none of it may stop a commit. Say so and run.
		console.error(`gate-lock: running without the lock (${(error as Error).message})`);
		return runCommand(command);
	}

	try {
		return await runCommand(command);
	} finally {
		release(lockDir);
	}
}

if (import.meta.main) {
	process.exitCode = await main(process.argv.slice(2));
}
