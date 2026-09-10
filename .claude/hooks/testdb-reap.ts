#!/usr/bin/env bun
// SessionStart hook: reaps orphaned testcontainers Postgres containers
// left behind by a killed `go test` process. See docs/testing.md's
// "Reaping orphaned testcontainers" section for the full story; the
// short version is below.
//
// api/internal/testdb starts one Postgres container per test *process*
// and tears it down via testdb.Main at process exit (see
// api/internal/testdb/setup.go). That teardown only runs on a clean
// exit -- docs/testing.md sets TESTCONTAINERS_RYUK_DISABLED=true for
// local Podman, so there is no other reaper. A killed process (an
// interrupted agent, a timeout, a cut-short TDD loop) leaves its
// container running forever. With several parallel Claude Code sessions
// on one machine, these accumulate without bound -- 116 containers, 4GB
// Podman machine down to 133MB free, observed live (#889).
//
// The reap decision (pickReapCandidates below) is pure and exported for
// direct unit testing. main() itself -- the engine invocation and the
// fail-open catch -- is exercised as a subprocess instead, the same way
// scripts/gate-bash-write.test.ts covers gate-bash-write.ts; see
// scripts/testdb-reap.test.ts. How the engine is reached at all lives in
// container-engine.ts, shared with e2e-stack-reap.ts.

import { execFileSync } from 'node:child_process';
import { engineInvocation, parseContainers, type ReapCandidate } from './container-engine.ts';

// A container backs one `go test` process for one package, so it lives
// minutes at most -- anything materially older is an orphan, not a slow
// test. Measured directly on this machine (`go test ./...`, api/, idle
// otherwise): every package finished in under 91s, the slowest being
// internal/payments at 90.4s. This threshold is deliberately generous
// rather than tight to that number -- the brief this shipped under put it
// plainly: a wrong reap kills a live run, reaping too late only delays
// cleanup. 15 minutes is roughly 10x the slowest observed single-package
// run, enough headroom for the exact condition that causes the leak in
// the first place (several parallel agent sessions competing for the
// same 4GB Podman machine, which can slow a test process well below its
// idle-machine time) without ever mistaking a live run for an orphan.
export const REAP_THRESHOLD_MS = 15 * 60 * 1000;

const TESTCONTAINERS_LABEL = 'org.testcontainers';
const TESTCONTAINERS_LABEL_VALUE = 'true';

// The one place that decides what gets `rm -f`'d. Filters on the label
// itself rather than trusting the engine's own `--filter` flag to have
// done it -- a destructive command should not depend solely on a filter
// string built elsewhere staying correct.
export function pickReapCandidates(
	containers: ReapCandidate[],
	nowMs: number,
	thresholdMs: number = REAP_THRESHOLD_MS
): ReapCandidate[] {
	return containers.filter(
		container =>
			container.labels[TESTCONTAINERS_LABEL] === TESTCONTAINERS_LABEL_VALUE && nowMs - container.createdAtMs > thresholdMs
	);
}

/*
 * Fails open, always -- deliberately, and unlike this repo's gate hooks.
 * gate-worktree-edit.ts and gate-bash-write.ts fail CLOSED on a crash:
 * they're PreToolUse gates, so a gate that cannot decide has not proven
 * an edit or command safe, and a crash gets treated as "block". This
 * hook is different in kind: it runs on SessionStart, has no decision to
 * make about the tool call that follows, and exists purely to tidy up.
 * A reaper that errors must never block or slow a session from starting
 * -- do not "fix" this into failing closed. gate-shared-index.sh is the
 * existing fail-open precedent, for the same reason: it fires on every
 * Bash command, so a broken instance of it must let the command through
 * rather than halt all shell work.
 */
function main(): void {
	try {
		const ps = engineInvocation([
			'ps',
			'-a',
			'--filter',
			`label=${TESTCONTAINERS_LABEL}=${TESTCONTAINERS_LABEL_VALUE}`,
			'--format',
			'json'
		]);
		const psOutput = execFileSync(ps.binary, ps.argv, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
		const candidates = pickReapCandidates(parseContainers(psOutput), Date.now());
		if (candidates.length === 0) return;

		const remove = engineInvocation(['rm', '-f', '-t', '2', ...candidates.map(c => c.id)]);
		execFileSync(remove.binary, remove.argv, { stdio: ['ignore', 'pipe', 'pipe'] });
		const minutes = Math.round(REAP_THRESHOLD_MS / 60000);
		console.log(
			`testdb-reap: removed ${candidates.length} orphaned testcontainers Postgres container(s) older than ${minutes}m (org.testcontainers=true, no process attached)`
		);
	} catch {
		// Engine unreachable, binary missing, malformed output, rm failed --
		// none of it may ever surface as a blocked or slowed SessionStart.
		return;
	}
}

if (import.meta.main) main();
