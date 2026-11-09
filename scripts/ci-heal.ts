#!/usr/bin/env bun
// Reruns a failed workflow run once when it failed on infrastructure
// (#2819): what `.github/workflows/ci-heal.yml` runs when a watched
// workflow completes, on trunk or on a pull request.
//
// The rerun is `--failed` and only of the first attempt, so a second
// failure is left to trunk-red and to a person. What counts as transient
// is in scripts/ci-transient.ts, which trunk-red reads too, so the alarm
// stays quiet for exactly the runs that this reruns.
//
// When the rerun cannot be made (#2822), no rerun comes and trunk-red has
// stayed quiet on the promise of one, so a trunk run opens its alarm here:
// this runs scripts/trunk-red.ts with TRUNK_RED_FORCE set, on the same
// event payload, so the alarm is the one it would have opened without the
// heal. A pull request run has no alarm to open, as trunk-red's own
// `branches: [trunk]` filter has it.
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { classifyRun } from './ci-transient.ts';

type Run = {
  id: number;
  name: string;
  conclusion: string | null;
  run_attempt?: number;
  head_branch?: string | null;
  event?: string;
  html_url: string;
};

const repo = process.env.REPO ?? process.env.GITHUB_REPOSITORY ?? '';
const replayId = process.env.REPLAY_RUN_ID ?? '';
const dryRun = replayId !== '';

function fail(message: string): never {
  console.error(`::error::ci-heal: ${message}`);
  process.exit(1);
}

function gh(args: string[]): string {
  const result = spawnSync(process.env.GH_BIN || 'gh', args, {
    encoding: 'utf8',
  });
  if (result.status !== 0) {
    fail(
      `gh ${args.join(' ')} failed (exit ${result.status}): ${result.stderr || result.error?.message || ''}`
    );
  }
  return result.stdout;
}

function readRun(): Run {
  if (replayId) {
    if (!/^\d+$/.test(replayId)) {
      fail(`REPLAY_RUN_ID must be a run id (digits only), got "${replayId}"`);
    }
    const replayed = JSON.parse(
      gh(['api', `repos/${repo}/actions/runs/${replayId}/attempts/1`])
    ) as Run;
    // A replay asks what would happen on the first attempt; a run that
    // was healed or rerun by hand has a later conclusion by now.
    return { ...replayed, run_attempt: 1 };
  }
  const event = JSON.parse(
    fs.readFileSync(process.env.GITHUB_EVENT_PATH ?? '', 'utf8')
  ) as { workflow_run?: Run };
  if (!event.workflow_run) fail('the event holds no workflow_run');
  return event.workflow_run;
}

if (!repo) fail('REPO is not set');
const run = readRun();
const attempt = run.run_attempt ?? 1;
console.log(
  `ci-heal: workflow=${JSON.stringify(run.name)} conclusion=${run.conclusion} attempt=${attempt} run=${run.html_url}${dryRun ? ' (replay: no write is made)' : ''}`
);

if (run.conclusion !== 'failure') {
  console.log(`nothing to do for the conclusion ${run.conclusion}`);
} else if (attempt !== 1) {
  console.log(
    `attempt ${attempt} is not the first, so it is left alone: one rerun only`
  );
} else {
  const verdict = classifyRun(repo, run);
  console.log(
    `verdict: ${verdict.transient ? 'transient' : 'not transient'} - ${verdict.reason}`
  );
  if (verdict.transient) {
    const endpoint = `repos/${repo}/actions/runs/${run.id}/rerun-failed-jobs`;
    if (dryRun) {
      console.log(`dry run: would POST ${endpoint}`);
    } else {
      const rerun = spawnSync(
        process.env.GH_BIN || 'gh',
        ['api', '--method', 'POST', endpoint],
        { encoding: 'utf8' }
      );
      if (rerun.status === 0) {
        console.log(`reran the failed jobs of run ${run.id}`);
      } else {
        console.error(
          `::error::ci-heal: gh api POST ${endpoint} failed (exit ${rerun.status}): ${rerun.stderr || rerun.error?.message || ''}`
        );
        if (run.head_branch === 'trunk' && run.event !== 'pull_request') {
          console.log('the rerun was refused on trunk: opening its alarm');
          const alarm = spawnSync(
            'bun',
            [path.join(import.meta.dir, 'trunk-red.ts')],
            { stdio: 'inherit', env: { ...process.env, TRUNK_RED_FORCE: '1' } }
          );
          if (alarm.status !== 0) fail('the alarm could not be opened');
        } else {
          console.log('not a trunk run, so there is no alarm to open');
        }
        process.exit(1);
      }
    }
  }
}
