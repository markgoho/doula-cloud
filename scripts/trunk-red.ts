#!/usr/bin/env bun
// The trunk-red alarm (#1021, #1639): what `.github/workflows/trunk-red.yml`
// runs when a workflow it watches completes on trunk.
//
// - A red run opens an assigned `trunk-red` issue, or comments on the one
//   that is open for the same workflow.
// - A passing run closes the open issue of the same workflow.
// - A canceled run opens nothing (#1207); for a push it says so on the
//   pull request that the commit came from.
//
// **One alarm per watched workflow.** An alarm issue belongs to the
// workflow whose file path is in the marker in its body. Every lookup
// filters by that marker, so `CI` going green cannot close the issue a
// failed site deploy opened, and the reverse. The path and not the name,
// because a change to a workflow's `name:` must not orphan an open alarm.
//
// The logic is here and not in the workflow file because `workflow_run`
// runs from the default branch only: no pull request can exercise that
// workflow. `scripts/trunk-red.test.ts` spawns this file with crafted
// payloads against a stand-in `gh` (the `GH_BIN` seam), and the `scripts`
// job is a required check.
//
// Every call is `gh api` (REST). `gh issue ...` spends the GraphQL budget
// that every session shares.
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';

const LABEL = 'trunk-red';
const ASSIGNEE = 'markgoho';
const ALARM_WORKFLOW = '.github/workflows/trunk-red.yml';

/** The fields of a `workflow_run` object that the alarm reads. */
type Run = {
  id: number;
  name: string;
  path: string;
  event: string;
  conclusion: string | null;
  head_sha: string;
  // Absent or null on some payloads; a dispatched run's is the trunk
  // head's, which is not a commit that caused the run.
  head_commit?: { message?: string | null } | null;
  html_url: string;
};

type Job = {
  name: string;
  conclusion: string | null;
  steps?: { name: string; conclusion: string | null }[];
};

type Issue = { number: number; body: string | null; pull_request?: unknown };

/**
 * What a watched workflow means, for the text of its alarm. A workflow
 * with no entry here gets `GENERIC`, so watching one more workflow needs
 * only its name in the `workflows:` list of the alarm workflow.
 */
type Meaning = {
  /** What a canceled run leaves without evidence. */
  unproven: string;
  /** What a person must know while the alarm is open. */
  whileOpen: string;
};

const CI_PATH = '.github/workflows/ci.yml';

const MEANING: Record<string, Meaning> = {
  [CI_PATH]: {
    unproven: 'migrate, deploy-api and deploy-app have',
    whileOpen:
      'The migrate, deploy-api and deploy-app jobs run ONLY on trunk, so a green pull request says nothing about them. While this issue is open, **no deploy is reaching the environment** and trunk is not a safe base to branch from.',
  },
  '.github/workflows/firebase-hosting-merge.yml': {
    unproven: 'the build and the deploy of the marketing site have',
    whileOpen:
      'This workflow builds and deploys the marketing site. It runs ONLY on a push to trunk and on a `practice-page-published` dispatch, so a green pull request says nothing about it. While this issue is open, **the live marketing site can be behind trunk**: a merged change to the site, or a Practice page published or changed after the last passing run, is possibly not live.',
  },
};

const GENERIC: Meaning = {
  unproven: 'the jobs of this workflow have',
  whileOpen:
    'This workflow runs on trunk, so a green pull request says nothing about this failure. While this issue is open, what this workflow builds or deploys is possibly not reaching the environment.',
};

/** A conclusion that opens the alarm. */
const RED = new Set(['failure', 'timed_out', 'startup_failure']);
const CANCELED = 'cancelled'; // spelling:ignore: the value GitHub's API sends

const repo = process.env.REPO ?? process.env.GITHUB_REPOSITORY ?? '';
const replayId = process.env.REPLAY_RUN_ID ?? '';
// A replay never writes: it prints each write it would make.
const dryRun = replayId !== '';

function fail(message: string): never {
  console.error(`::error::trunk-red: ${message}`);
  process.exit(1);
}

/** `gh api`, with the JSON answer parsed. A failed call ends the run. */
function api(endpoint: string, method = 'GET', input?: object): unknown {
  if (method !== 'GET' && dryRun) {
    console.log(`dry run: ${method} ${endpoint}`);
    console.log(JSON.stringify(input, null, 2));
    return {};
  }
  const args = ['api'];
  if (method !== 'GET') args.push('--method', method);
  args.push(endpoint);
  if (input) args.push('--input', '-');
  const result = spawnSync(process.env.GH_BIN || 'gh', args, {
    encoding: 'utf8',
    input: input ? JSON.stringify(input) : undefined,
  });
  if (result.status !== 0) {
    throw new Error(
      `gh api ${method} ${endpoint} failed (exit ${result.status}): ${result.stderr || result.error?.message || ''}`
    );
  }
  return JSON.parse(result.stdout || 'null');
}

/** Says what was written. A replay writes nothing, so it says nothing. */
function wrote(message: string): void {
  if (!dryRun) console.log(message);
}

function readRun(): Run {
  if (replayId) {
    if (!/^\d+$/.test(replayId)) {
      fail(`REPLAY_RUN_ID must be a run id (digits only), got "${replayId}"`);
    }
    return api(`repos/${repo}/actions/runs/${replayId}`) as Run;
  }
  const event = JSON.parse(
    fs.readFileSync(process.env.GITHUB_EVENT_PATH ?? '', 'utf8')
  ) as { workflow_run?: Run };
  if (!event.workflow_run) fail('the event holds no workflow_run');
  return event.workflow_run;
}

const marker = (path: string) => `<!-- ${LABEL}: ${path} -->`;
const ANY_MARKER = `<!-- ${LABEL}: `;

/**
 * The open alarm issues of one workflow, newest first. An open issue
 * with the label and no marker is from before #1639, when `CI` was the
 * only workflow watched, so it is `CI`'s.
 */
function openAlarms(path: string): Issue[] {
  const issues = api(
    `repos/${repo}/issues?labels=${LABEL}&state=open&per_page=100`
  ) as Issue[];
  return issues.filter((issue) => {
    // The issues endpoint lists pull requests too.
    if (issue.pull_request) return false;
    const body = issue.body ?? '';
    if (body.includes(marker(path))) return true;
    return path === CI_PATH && !body.includes(ANY_MARKER);
  });
}

/** One line per failed job, with the step that failed inside it. */
function failedJobs(run: Run): string {
  let jobs: Job[];
  try {
    const answer = api(
      `repos/${repo}/actions/runs/${run.id}/jobs?per_page=100`
    ) as { jobs?: Job[] };
    jobs = answer.jobs ?? [];
  } catch (error) {
    // The alarm matters more than its detail: open it with the run link.
    console.log(`::warning::${(error as Error).message}`);
    return '- (the jobs of this run could not be read; see the run)';
  }
  const lines = jobs
    .filter((job) => job.conclusion !== null && RED.has(job.conclusion))
    .flatMap((job) => {
      const steps = (job.steps ?? []).filter(
        (step) => step.conclusion !== null && RED.has(step.conclusion)
      );
      return steps.length > 0
        ? steps.map(
            (step) => `- **${job.name}** - failed at step _${step.name}_`
          )
        : [`- **${job.name}** - \`${job.conclusion}\`, with no failed step`];
    });
  return lines.length > 0
    ? [...new Set(lines)].sort().join('\n')
    : '- (no job reported a failure conclusion; see the run)';
}

const run = readRun();
// `path` can carry an `@ref` suffix for a workflow that another calls.
const path = (run.path ?? '').split('@')[0] ?? '';
const short = (run.head_sha ?? '').slice(0, 8);
const subject = (run.head_commit?.message ?? '').split('\n')[0] ?? '';
const meaning = MEANING[path] ?? GENERIC;
// Only a push run is the run of a commit. A dispatched run takes the
// trunk head as its `head_sha` and `head_commit`, so the subject there
// names a pull request that has nothing to do with the run.
const isPush = run.event === 'push';

console.log(
  `trunk-red: workflow=${JSON.stringify(run.name)} path=${path} event=${run.event} conclusion=${run.conclusion} head=${short} run=${run.html_url}${dryRun ? ' (replay: no write is made)' : ''}`
);

if (!repo) fail('REPO is not set');
if (!path) fail('the run has no workflow path, so its alarm has no identity');

function openOrUpdate(): void {
  const how =
    run.conclusion === 'failure'
      ? 'failed'
      : `ended with \`${run.conclusion}\``;
  const trigger = isPush
    ? `**Commit:** ${subject || '(no commit message in the event)'} (${run.head_sha})`
    : `**Trigger:** a \`${run.event}\` event, not a push. This run has no commit and no pull request of its own. It built trunk at **${short}**, and that commit did not cause the run.`;
  const body = [
    `**${run.name}** ${how} on trunk at **${short}**.`,
    failedJobs(run),
    trigger,
    `**Run:** ${run.html_url}`,
    meaning.whileOpen,
    `This issue is opened automatically by ${ALARM_WORKFLOW} and closes itself when ${run.name} next passes on trunk. Do not close it by hand while that workflow still fails - the next failed run opens a new one.`,
    marker(path),
  ].join('\n\n');

  const existing = openAlarms(path)[0];
  if (existing) {
    api(`repos/${repo}/issues/${existing.number}/comments`, 'POST', { body });
    wrote(`commented on existing #${existing.number}`);
    return;
  }
  const where = isPush
    ? short
    : run.event === 'repository_dispatch'
      ? 'a dispatched rebuild'
      : `a ${run.event} run`;
  const created = api(`repos/${repo}/issues`, 'POST', {
    title: `Trunk is red: ${run.name} failed on ${where}`,
    body,
    labels: [LABEL],
    assignees: [ASSIGNEE],
  }) as { number?: number };
  wrote(`opened #${created.number}`);
}

function close(): void {
  for (const issue of openAlarms(path)) {
    api(`repos/${repo}/issues/${issue.number}/comments`, 'POST', {
      body: `${run.name} is green again on trunk as of ${short} - ${run.html_url}. Closed automatically by ${ALARM_WORKFLOW}.`,
    });
    api(`repos/${repo}/issues/${issue.number}`, 'PATCH', {
      state: 'closed',
      state_reason: 'completed',
    });
    wrote(`closed #${issue.number}`);
  }
}

/**
 * A canceled run is not a failure (#1207): a run queued behind two
 * others in the same concurrency group is canceled before any job
 * starts, which is routine under a burst of trunk merges. It opens no
 * alarm. The next run of the same workflow that completes is evidence
 * for this commit too, so this only names the gap, on the pull request
 * the commit came from.
 */
function flagCanceled(): void {
  if (!isPush) {
    console.log(
      `::notice::Canceled ${run.event} run ${run.html_url} of ${run.name}: it has no pull request of its own to comment on. The next run of this workflow builds the same trunk.`
    );
    return;
  }
  const pr = [...subject.matchAll(/\(#(\d+)\)/g)].at(-1)?.[1];
  if (!pr) {
    console.log(
      `::warning::Canceled trunk run ${run.html_url} for ${short} has no (#NNNN) in its commit subject to comment on: ${subject}`
    );
    return;
  }
  const body = [
    `The trunk run of **${run.name}** on **${short}** was canceled, so **${meaning.unproven} no evidence for this commit.**`,
    'This is expected under a burst of trunk merges (see the concurrency comment in ci.yml), not a failure -- it does not open a trunk-red issue.',
    `What to check instead: the next trunk run of ${run.name} whose head has this commit as an ancestor (git merge-base --is-ancestor ${run.head_sha} <later-sha>), read by its conclusion field. "gh run watch --exit-status" exits 0 on a canceled run, so it cannot tell a canceled run from a passing one -- only conclusion can.`,
    `**Commit:** ${subject}`,
    `**Canceled run:** ${run.html_url}`,
  ].join('\n\n');
  api(`repos/${repo}/issues/${pr}/comments`, 'POST', { body });
  wrote(`commented on #${pr}`);
}

try {
  if (run.conclusion !== null && RED.has(run.conclusion)) openOrUpdate();
  else if (run.conclusion === 'success') close();
  else if (run.conclusion === CANCELED) flagCanceled();
  else console.log(`nothing to do for the conclusion ${run.conclusion}`);
} catch (error) {
  fail((error as Error).message);
}
