#!/usr/bin/env bun
// SessionStart hook: reaps the containers a killed `go test` process
// leaves behind. See docs/testing/api.md's "Reaping orphaned
// testcontainers" section for the full story; the short version is below.
//
// api/internal/testdb (and simclock's own TestMain) start one Postgres
// container per test *process* and terminate it when the process exits
// cleanly. docs/testing/api.md sets TESTCONTAINERS_RYUK_DISABLED=true for
// local Podman, so a killed process (an interrupted agent, a timeout, a
// cut-short TDD loop) leaves its container running forever. With several
// parallel Claude Code sessions on one Podman machine they accumulate
// without bound (#889, #2096).
//
// Every such container carries two labels, written by testdb.OwnerLabels
// (api/internal/testdb/labels.go): a marker, and the pid of the test
// process that started it. A container is an orphan when, and only when,
// it carries the marker and its pid is gone. No age, no guess: a live run
// keeps its container however long it takes, and a container this hook
// cannot read an owner from is kept.
//
// The reap decision (pickReapCandidates below) is pure and exported for
// direct unit testing. main() -- the engine calls, the dry run and the
// fail-open catch -- is exercised as a subprocess instead; see
// scripts/testdb-reap.test.ts. How the engine is reached lives in
// container-engine.ts, and the pid liveness test in e2e-stack-reap.ts;
// both are shared, not copied.

import {
  execEngine,
  parseContainers,
  type ReapCandidate,
} from './container-engine.ts';
import { isProcessAlive } from './e2e-stack-reap.ts';

// Must match OwnerLabel and OwnerPIDLabel in api/internal/testdb/labels.go.
export const OWNER_LABEL = 'cloud.doula.test-container';
export const OWNER_PID_LABEL = 'cloud.doula.test-container.pid';

// An e2e compose stack is never this hook's to remove (e2e-stack-reap.ts
// owns those). A stack container carries no owner marker today; this is
// the second lock on the same door.
const COMPOSE_PROJECT_LABEL = 'com.docker.compose.project';

const DRY_RUN_FLAG = '--dry-run';

// The owning pid, or undefined when the label is missing or is not a
// plain positive decimal -- which pickReapCandidates reads as "keep".
export function ownerPid(container: ReapCandidate): number | undefined {
  const raw = container.labels[OWNER_PID_LABEL];
  if (raw === undefined || !/^[1-9]\d*$/.test(raw)) return undefined;
  const pid = Number(raw);
  return Number.isSafeInteger(pid) ? pid : undefined;
}

// The one place that decides what gets `rm -f`'d. Filters on the labels
// itself rather than trusting the engine's `--filter` flag to have done
// it -- a destructive command should not depend solely on a filter
// string built elsewhere staying correct.
//
// A recycled pid -- the test process died and the OS has since handed
// its pid to something else -- reads as alive, so that container is kept
// until the pid is free again. That errs toward keeping, never toward
// removing a live run.
export function pickReapCandidates(
  containers: ReapCandidate[],
  isAlive: (pid: number) => boolean = isProcessAlive
): ReapCandidate[] {
  return containers.filter((container) => {
    if (container.labels[OWNER_LABEL] !== 'true') return false;
    if (container.labels[COMPOSE_PROJECT_LABEL] !== undefined) return false;
    const pid = ownerPid(container);
    return pid !== undefined && !isAlive(pid);
  });
}

function summarize(container: ReapCandidate): string {
  return `${container.id.slice(0, 12)} ${container.name} (pid ${container.labels[OWNER_PID_LABEL]} gone)`;
}

/*
 * Fails open, always -- deliberately, and unlike this repo's gate hooks.
 * gate-worktree-edit.ts and gate-bash-write.ts fail CLOSED on a crash:
 * they're PreToolUse gates, so a gate that cannot decide has not proven
 * an edit or command safe, and a crash gets treated as "block". This
 * hook is different in kind: it runs on SessionStart, has no decision to
 * make about the tool call that follows, and exists purely to tidy up.
 * A reaper that errors must never block or slow a session from starting
 * -- do not "fix" this into failing closed.
 *
 * `--dry-run` lists what a real run would remove and removes nothing.
 */
function main(argv: string[]): void {
  const dryRun = argv.includes(DRY_RUN_FLAG);
  try {
    const psOutput = execEngine([
      'ps',
      '-a',
      '--filter',
      `label=${OWNER_LABEL}=true`,
      '--format',
      'json',
    ]);
    const candidates = pickReapCandidates(parseContainers(psOutput));
    if (dryRun) {
      console.log(
        `testdb-reap (dry run): would remove ${candidates.length} orphaned test container(s)`
      );
      for (const container of candidates)
        console.log(`  ${summarize(container)}`);
      return;
    }
    if (candidates.length === 0) return;

    execEngine(['rm', '-f', '-t', '2', ...candidates.map((c) => c.id)]);
    console.log(
      `testdb-reap: removed ${candidates.length} orphaned test container(s) whose go test process is gone (${OWNER_LABEL}=true)`
    );
  } catch {
    // Engine unreachable, binary missing, malformed output, rm failed --
    // none of it may ever surface as a blocked or slowed SessionStart.
    return;
  }
}

if (import.meta.main) main(process.argv.slice(2));
