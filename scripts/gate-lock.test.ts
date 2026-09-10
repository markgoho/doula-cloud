/**
 * `scripts/gate-lock.ts` -- the exclusive lock the pre-commit gate's
 * heavy step runs under, so that several concurrent sessions cannot all
 * start headless Chromium at once (#936). See that file's header and
 * docs/testing.md's "The memory this gate costs" section.
 *
 * `inspectLock` and `isProcessAlive` are pure enough to import and test
 * directly. Everything that matters about the wrapper, though, is a
 * property of real concurrent processes -- mutual exclusion, reclaim,
 * fail-open -- so those are exercised by spawning the real script, the
 * same way scripts/testdb-reap.test.ts covers the reaper's main().
 */
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { inspectLock, isProcessAlive, STALE_LOCK_MS, type LockOwner, type LockVerdict } from './gate-lock.ts';

// Narrows the verdict union so a reason can be asserted on directly.
const reclaimReason = (verdict: LockVerdict): string => (verdict.action === 'reclaim' ? verdict.reason : '');

const SCRIPT = path.join(import.meta.dir, 'gate-lock.ts');

const alwaysAlive = () => true;
const neverAlive = () => false;

function owner(pid: number): LockOwner {
	return { pid, label: 'some-worktree', startedAtMs: 0 };
}

describe('inspectLock', () => {
	test('waits on a fresh lock whose owner is alive', () => {
		expect(inspectLock(1000, owner(4242), alwaysAlive)).toEqual({ action: 'wait' });
	});

	test('reclaims as soon as the owner process is gone, however fresh the lock', () => {
		expect(reclaimReason(inspectLock(0, owner(4242), neverAlive))).toContain('4242');
	});

	test('reclaims a lock held past the staleness gate even while its owner lives', () => {
		expect(reclaimReason(inspectLock(STALE_LOCK_MS + 1000, owner(4242), alwaysAlive))).toContain('staleness gate');
	});

	test('does not reclaim a lock held exactly at the staleness gate', () => {
		expect(inspectLock(STALE_LOCK_MS, owner(4242), alwaysAlive)).toEqual({ action: 'wait' });
	});

	test('waits on a fresh lock with no owner file yet, rather than reclaiming the gap between mkdir and the owner write', () => {
		expect(inspectLock(5, null, neverAlive)).toEqual({ action: 'wait' });
	});

	test('falls back to the age gate for a lock with no readable owner', () => {
		expect(inspectLock(STALE_LOCK_MS + 1, null, alwaysAlive).action).toBe('reclaim');
	});

	test('accepts a custom staleness threshold', () => {
		expect(inspectLock(2000, owner(4242), alwaysAlive, 1000).action).toBe('reclaim');
	});
});

describe('isProcessAlive', () => {
	test('reports this very process as alive', () => {
		expect(isProcessAlive(process.pid)).toBe(true);
	});

	test('reports a process that has exited as gone', () => {
		// spawnSync has already reaped it by the time it returns, so this
		// pid is genuinely dead rather than merely unlikely to exist.
		const finished = spawnSync('sh', ['-c', 'exit 0']);
		expect(isProcessAlive(finished.pid as number)).toBe(false);
	});

	test('treats a nonsense pid as alive, because the age gate clears it and assuming dead would run two gates at once', () => {
		expect(isProcessAlive(0)).toBe(true);
		expect(isProcessAlive(Number.NaN)).toBe(true);
	});
});

/*
 * The wrapper end to end. Each case runs the real script as a subprocess
 * with GATE_LOCK_DIR pointed at a scratch directory, so no test ever
 * touches the repo's own `.git`.
 */
describe('the wrapper', () => {
	let scratch: string;
	let lockDir: string;
	let journal: string;
	let marker: string;

	beforeEach(() => {
		scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'gate-lock-test-'));
		lockDir = path.join(scratch, 'pre-commit-gate.lock');
		journal = path.join(scratch, 'journal.txt');
		// Stands in for the gate's heavy step: brackets its own run in a
		// shared append-only journal, so overlap between two runs is
		// visible as an interleaving rather than having to be timed.
		marker = path.join(scratch, 'marker.ts');
		fs.writeFileSync(
			marker,
			[
				`import fs from 'node:fs';`,
				`const journal = process.argv[2];`,
				`fs.appendFileSync(journal, \`start \${process.pid}\\n\`);`,
				`await new Promise(r => setTimeout(r, 300));`,
				`fs.appendFileSync(journal, \`end \${process.pid}\\n\`);`,
				`process.exit(Number(process.argv[3] ?? 0));`
			].join('\n')
		);
	});

	afterEach(() => {
		fs.rmSync(scratch, { recursive: true, force: true });
	});

	function invoke(args: string[], env: Record<string, string | undefined> = {}): Promise<{ exitCode: number; stderr: string }> {
		return new Promise((resolve, reject) => {
			const child = spawn('bun', [SCRIPT, '--', ...args], {
				stdio: ['ignore', 'ignore', 'pipe'],
				env: { ...process.env, GATE_LOCK_DIR: lockDir, ...env }
			});
			let stderr = '';
			child.stderr.on('data', chunk => {
				stderr += chunk;
			});
			child.on('error', reject);
			child.on('close', code => resolve({ exitCode: code ?? 1, stderr }));
		});
	}

	const runMarker = (env?: Record<string, string | undefined>, exitCode = 0) => invoke(['bun', marker, journal, String(exitCode)], env);

	const journalLines = () => fs.readFileSync(journal, 'utf8').trim().split('\n');

	// The whole point of the ticket: three sessions committing at once.
	test('serializes three concurrent runs so their journals never interleave', async () => {
		const results = await Promise.all([runMarker(), runMarker(), runMarker()]);
		expect(results.map(r => r.exitCode)).toEqual([0, 0, 0]);

		const lines = journalLines();
		expect(lines).toHaveLength(6);
		for (let i = 0; i < lines.length; i += 2) {
			const [startWord, startPid] = lines[i].split(' ');
			const [endWord, endPid] = lines[i + 1].split(' ');
			expect(startWord).toBe('start');
			expect(endWord).toBe('end');
			expect(endPid).toBe(startPid);
		}
	}, 30000);

	test('says what it is waiting on instead of looking hung', async () => {
		const [first, second] = await Promise.all([runMarker(), runMarker()]);
		const notices = `${first.stderr}${second.stderr}`;
		expect(notices).toContain("'s test run");
		expect(notices).toContain('Nothing is hung');
		expect(notices).toContain('SKIP_GATE_LOCK=1');
	}, 30000);

	test('releases the lock when the run finishes', async () => {
		await runMarker();
		expect(fs.existsSync(lockDir)).toBe(false);
	}, 30000);

	test('propagates the wrapped command exit status', async () => {
		expect((await runMarker(undefined, 3)).exitCode).toBe(3);
	}, 30000);

	// A lock a killed session left behind must never block a commit.
	test('reclaims a lock whose owner process is gone and runs anyway', async () => {
		const dead = spawnSync('sh', ['-c', 'exit 0']).pid as number;
		fs.mkdirSync(lockDir);
		fs.writeFileSync(path.join(lockDir, 'owner.json'), JSON.stringify({ pid: dead, label: 'killed-session', startedAtMs: Date.now() }));

		const result = await runMarker();
		expect(result.exitCode).toBe(0);
		expect(result.stderr).toContain('reclaimed a stale lock');
		expect(journalLines()).toHaveLength(2);
	}, 30000);

	// The age gate, proved without waiting five real minutes: a lock
	// whose owner file claims a live pid (this test process) but whose
	// directory mtime is old.
	test('reclaims a lock held past the staleness gate and runs anyway', async () => {
		fs.mkdirSync(lockDir);
		fs.writeFileSync(path.join(lockDir, 'owner.json'), JSON.stringify({ pid: process.pid, label: 'wedged-session', startedAtMs: 0 }));
		const longAgo = new Date(Date.now() - (STALE_LOCK_MS + 60_000));
		fs.utimesSync(lockDir, longAgo, longAgo);

		const result = await runMarker();
		expect(result.exitCode).toBe(0);
		expect(result.stderr).toContain('staleness gate');
		expect(journalLines()).toHaveLength(2);
	}, 30000);

	test('runs without the lock when the lock directory cannot be created', async () => {
		// A regular file where the lock's parent directory should be:
		// nothing under it can ever be made.
		const blocker = path.join(scratch, 'not-a-directory');
		fs.writeFileSync(blocker, 'x');
		lockDir = path.join(blocker, 'pre-commit-gate.lock');

		const result = await runMarker();
		expect(result.exitCode).toBe(0);
		expect(result.stderr).toContain('running without the lock');
		expect(journalLines()).toHaveLength(2);
	}, 30000);

	test('skips the lock entirely under SKIP_GATE_LOCK, even while another session holds it', async () => {
		fs.mkdirSync(lockDir);
		fs.writeFileSync(path.join(lockDir, 'owner.json'), JSON.stringify({ pid: process.pid, label: 'live-session', startedAtMs: Date.now() }));

		const result = await runMarker({ SKIP_GATE_LOCK: '1' });
		expect(result.exitCode).toBe(0);
		expect(journalLines()).toHaveLength(2);
		expect(fs.existsSync(lockDir)).toBe(true); // the other session's lock is left alone
	}, 30000);

	// The other half of the reclaim race: a run whose lock was taken away
	// mid-flight must not delete its successor's lock on the way out, or
	// a third session walks straight in alongside the second.
	test('leaves a successor lock alone when its own was reclaimed mid-run', async () => {
		const ownerFile = path.join(lockDir, 'owner.json');
		const running = runMarker();
		// Wait for the run to have fully taken the lock, owner file and
		// all, so what follows is a reclaim and not a torn acquire.
		while (!fs.existsSync(ownerFile)) await new Promise(resolve => setTimeout(resolve, 10));

		fs.rmSync(lockDir, { recursive: true, force: true });
		fs.mkdirSync(lockDir);
		fs.writeFileSync(ownerFile, JSON.stringify({ pid: process.pid, label: 'successor', startedAtMs: Date.now() }));

		expect((await running).exitCode).toBe(0);
		expect(fs.existsSync(lockDir)).toBe(true);
		expect(JSON.parse(fs.readFileSync(ownerFile, 'utf8')).label).toBe('successor');
	}, 30000);

	// Without this the staleness gate would be a run-time limit, and a
	// gate that legitimately ran past it would have its lock taken while
	// it was still working -- two gates at once, the thing this prevents.
	test('keeps the lock fresh while the wrapped command runs', async () => {
		const running = runMarker({ GATE_LOCK_HEARTBEAT_MS: '50' });
		while (!fs.existsSync(lockDir)) await new Promise(resolve => setTimeout(resolve, 5));
		const atStart = fs.statSync(lockDir).mtimeMs;
		await new Promise(resolve => setTimeout(resolve, 150));
		const laterMs = fs.statSync(lockDir).mtimeMs;

		expect(laterMs).toBeGreaterThan(atStart);
		expect((await running).exitCode).toBe(0);
	}, 30000);

	// A Ctrl-C during the gate must not leave a lock for the staleness
	// gate to clear five minutes later.
	test('releases the lock when it is interrupted', async () => {
		const child = spawn('bun', [SCRIPT, '--', 'bun', marker, journal, '0'], {
			stdio: ['ignore', 'ignore', 'ignore'],
			env: { ...process.env, GATE_LOCK_DIR: lockDir }
		});
		while (!fs.existsSync(path.join(lockDir, 'owner.json'))) await new Promise(resolve => setTimeout(resolve, 5));
		child.kill('SIGTERM');
		await new Promise(resolve => child.on('close', resolve));

		expect(fs.existsSync(lockDir)).toBe(false);
	}, 30000);

	test('refuses an empty command rather than silently succeeding', async () => {
		const result = await invoke([]);
		expect(result.exitCode).toBe(2);
		expect(result.stderr).toContain('usage');
	}, 30000);
});
