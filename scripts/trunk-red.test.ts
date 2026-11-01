/**
 * `scripts/trunk-red.ts` -- what `.github/workflows/trunk-red.yml` runs
 * when a watched workflow completes on trunk (#1021, #1639).
 *
 * That workflow fires on `workflow_run`, which GitHub runs from the
 * default branch only, so no pull request can exercise it. These tests
 * are the proof in its place: each one spawns the real script with a
 * crafted `workflow_run` payload, against a stand-in `gh` (the `GH_BIN`
 * seam) that answers from fixtures and records every call it receives.
 */
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const ROOT = path.join(import.meta.dir, '..');
const SCRIPT = path.join(import.meta.dir, 'trunk-red.ts');
const WORKFLOWS = path.join(ROOT, '.github', 'workflows');
const REPO = 'markgoho/doula-cloud';

const CI = { name: 'CI', path: '.github/workflows/ci.yml' };
const SITE = {
  name: 'Deploy the marketing site to Firebase Hosting on merge',
  path: '.github/workflows/firebase-hosting-merge.yml',
};
const TERRAFORM = {
  name: 'Terraform Plan',
  path: '.github/workflows/terraform-plan.yml',
};

const CANCELED = 'cancelled'; // spelling:ignore: the value GitHub's API sends
const SHA = '8b41853e0f6a4b0c9d2e7f1a3c5b7d9e1f2a4c6e';
const LATER_SHA = '407184f7c2d1e0b9a8f7e6d5c4b3a2f1e0d9c8b7';
const RUN_URL = 'https://github.com/markgoho/doula-cloud/actions/runs/7001';

let dir: string;

beforeEach(() => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trunk-red-test-'));
});

afterEach(() => {
  fs.rmSync(dir, { recursive: true, force: true });
});

type Issue = { number: number; body: string | null; pull_request?: object };
type Job = {
  name: string;
  conclusion: string | null;
  steps?: { name: string; conclusion: string | null }[];
};
type Fixtures = {
  issues?: Issue[];
  jobs?: Job[];
  /** The stand-in exits 1 on the jobs read. */
  jobsFail?: boolean;
  /** What `repos/<repo>/actions/runs/<id>` answers, for a replay. */
  run?: object;
  /** The comments of each issue, by its number. */
  comments?: Record<number, { body: string }[]>;
  /** The paths `repos/<repo>/commits/<sha>` lists as changed. */
  commitFiles?: string[];
  /** The stand-in exits 1 on the commit read. */
  commitFail?: boolean;
};
type Call = { method: string; endpoint: string; input: unknown };

/**
 * A stand-in `gh`. It understands `gh api [--method M] <endpoint>
 * [--input -]`, answers a read from the fixtures, and appends each call
 * to a file, because each call is a new process.
 */
function standIn(fixtures: Fixtures): string {
  const bin = path.join(dir, 'gh');
  fs.writeFileSync(path.join(dir, 'fixtures.json'), JSON.stringify(fixtures));
  fs.writeFileSync(
    bin,
    [
      '#!/usr/bin/env bun',
      "import fs from 'node:fs';",
      "import path from 'node:path';",
      'const args = process.argv.slice(2);',
      "if (args[0] !== 'api') { console.error('stand-in gh: only `gh api` is expected, got ' + args.join(' ')); process.exit(2); }",
      "let method = 'GET'; let endpoint = ''; let readsInput = false;",
      'for (let i = 1; i < args.length; i++) {',
      "  if (args[i] === '--method') method = args[++i];",
      "  else if (args[i] === '--input') { readsInput = true; i++; }",
      '  else endpoint = args[i];',
      '}',
      "const input = readsInput ? JSON.parse(fs.readFileSync(0, 'utf8')) : null;",
      "const fixtures = JSON.parse(fs.readFileSync(path.join(import.meta.dir, 'fixtures.json'), 'utf8'));",
      "fs.appendFileSync(path.join(import.meta.dir, 'calls.jsonl'), JSON.stringify({ method, endpoint, input }) + '\\n');",
      "if (method === 'GET' && /\\/actions\\/runs\\/\\d+\\/attempts\\/\\d+\\/jobs/.test(endpoint)) {",
      "  if (fixtures.jobsFail) { console.error('HTTP 502'); process.exit(1); }",
      '  console.log(JSON.stringify({ jobs: fixtures.jobs ?? [] }));',
      "} else if (method === 'GET' && /\\/commits\\/[0-9a-f]+$/.test(endpoint)) {",
      "  if (fixtures.commitFail) { console.error('HTTP 502'); process.exit(1); }",
      '  console.log(JSON.stringify({ files: (fixtures.commitFiles ?? []).map((filename) => ({ filename })) }));',
      "} else if (method === 'GET' && /\\/issues\\/\\d+\\/comments/.test(endpoint)) {",
      '  const number = Number(endpoint.match(/\\/issues\\/(\\d+)\\/comments/)[1]);',
      '  console.log(JSON.stringify((fixtures.comments ?? {})[number] ?? []));',
      "} else if (method === 'GET' && /\\/actions\\/runs\\/\\d+$/.test(endpoint)) {",
      '  console.log(JSON.stringify(fixtures.run ?? {}));',
      "} else if (method === 'GET' && /\\/issues\\?/.test(endpoint)) {",
      '  console.log(JSON.stringify(fixtures.issues ?? []));',
      "} else if (method === 'POST' && /\\/issues$/.test(endpoint)) {",
      "  console.log(JSON.stringify({ number: 4242, html_url: 'https://github.com/markgoho/doula-cloud/issues/4242' }));",
      '} else {',
      "  console.log('{}');",
      '}',
      '',
    ].join('\n'),
    { mode: 0o755 }
  );
  return bin;
}

type RunFields = {
  conclusion: string;
  event?: string;
  message?: string | null;
  sha?: string;
  attempt?: number;
};

/** The fields of a `workflow_run` object that the script reads. */
function workflowRun(
  workflow: { name: string; path: string },
  { conclusion, event = 'push', message, sha = SHA, attempt = 1 }: RunFields
) {
  return {
    id: 7001,
    run_attempt: attempt,
    name: workflow.name,
    path: workflow.path,
    event,
    status: 'completed',
    conclusion,
    head_branch: 'trunk',
    head_sha: sha,
    head_commit:
      message === null
        ? null
        : {
            message:
              message ??
              'The three SvelteKit packages move to 3.0.0 stable (#1657) (#1660)\n\nA body line that names (#9999).',
          },
    html_url: RUN_URL,
  };
}

function alarm(
  run: object | null,
  fixtures: Fixtures = {},
  env: Record<string, string> = {}
) {
  const eventPath = path.join(dir, 'event.json');
  fs.writeFileSync(
    eventPath,
    JSON.stringify(run ? { action: 'completed', workflow_run: run } : {})
  );
  const result = spawnSync('bun', [SCRIPT], {
    encoding: 'utf8',
    cwd: dir,
    env: {
      ...process.env,
      GH_BIN: standIn(fixtures),
      GITHUB_EVENT_PATH: eventPath,
      REPO,
      REPLAY_RUN_ID: '',
      ...env,
    },
  });
  const log = path.join(dir, 'calls.jsonl');
  const calls: Call[] = fs.existsSync(log)
    ? fs
        .readFileSync(log, 'utf8')
        .split('\n')
        .filter((line) => line !== '')
        .map((line) => JSON.parse(line) as Call)
    : [];
  return {
    status: result.status,
    stdout: result.stdout,
    stderr: result.stderr,
    calls,
    writes: calls.filter((call) => call.method !== 'GET'),
  };
}

const marker = (workflow: { path: string }) =>
  `<!-- trunk-red: ${workflow.path} -->`;

const shaMarker = (sha: string) => `<!-- trunk-red-sha: ${sha} -->`;

const openAlarm = (
  number: number,
  workflow: { path: string },
  sha = SHA
): Issue => ({
  number,
  body: `It failed.\n\n${shaMarker(sha)}\n\n${marker(workflow)}\n`,
});

const DEPLOY_FAILED: Job[] = [
  { name: 'build', conclusion: 'success', steps: [] },
  {
    name: 'deploy',
    conclusion: 'failure',
    steps: [
      { name: 'Checkout repository', conclusion: 'success' },
      { name: 'Deploy to Firebase', conclusion: 'failure' },
      { name: 'Confirm every published page resolves', conclusion: 'skipped' },
    ],
  },
];

type IssueInput = {
  title: string;
  body: string;
  labels: string[];
  assignees: string[];
};
type CommentInput = { body: string };

describe('a red run', () => {
  test('of the site deploy on a push opens an assigned, labeled issue that names the workflow, the job, the step and the run', () => {
    const result = alarm(workflowRun(SITE, { conclusion: 'failure' }), {
      jobs: DEPLOY_FAILED,
    });

    expect(result.status).toBe(0);
    expect(result.writes).toHaveLength(1);
    const [created] = result.writes;
    expect(created?.method).toBe('POST');
    expect(created?.endpoint).toBe(`repos/${REPO}/issues`);
    const input = created?.input as IssueInput;
    expect(input.title).toBe(
      'Trunk is red: Deploy the marketing site to Firebase Hosting on merge failed on 8b41853e'
    );
    expect(input.labels).toEqual(['trunk-red']);
    expect(input.assignees).toEqual(['markgoho']);
    expect(input.body).toContain(
      '**Deploy the marketing site to Firebase Hosting on merge** failed on trunk at **8b41853e**.'
    );
    expect(input.body).toContain(
      '- **deploy** - failed at step _Deploy to Firebase_'
    );
    expect(input.body).not.toContain('**build**');
    expect(input.body).toContain(
      '**Commit:** The three SvelteKit packages move to 3.0.0 stable (#1657) (#1660)'
    );
    expect(input.body).not.toContain('#9999');
    expect(input.body).toContain(`(${SHA})`);
    expect(input.body).toContain(`**Run:** ${RUN_URL}`);
    expect(input.body).toContain('marketing site');
    expect(input.body).toContain(marker(SITE));
  });

  test('of the site deploy on a dispatch says that the run has no commit and no pull request of its own', () => {
    const result = alarm(
      workflowRun(SITE, {
        conclusion: 'failure',
        event: 'repository_dispatch',
      }),
      { jobs: DEPLOY_FAILED }
    );

    expect(result.status).toBe(0);
    expect(result.writes).toHaveLength(1);
    const input = result.writes[0]?.input as IssueInput;
    expect(input.title).toBe(
      'Trunk is red: Deploy the marketing site to Firebase Hosting on merge failed on a dispatched rebuild'
    );
    expect(input.body).toContain('`repository_dispatch`');
    expect(input.body).toContain('no commit and no pull request of its own');
    expect(input.body).toContain('**8b41853e**');
    // The trunk head's subject belongs to a pull request that has
    // nothing to do with a dispatched run.
    expect(input.body).not.toContain('**Commit:**');
    expect(input.body).not.toContain('#1660');
    expect(input.body).toContain(
      '- **deploy** - failed at step _Deploy to Firebase_'
    );
    expect(input.body).toContain(marker(SITE));
  });

  test('of a dispatch whose payload has no head commit is not an error', () => {
    const result = alarm(
      workflowRun(SITE, {
        conclusion: 'failure',
        event: 'repository_dispatch',
        message: null,
      }),
      { jobs: DEPLOY_FAILED }
    );

    expect(result.status).toBe(0);
    expect(result.writes).toHaveLength(1);
  });

  test('of a push whose payload has no head commit is not an error', () => {
    const result = alarm(
      workflowRun(SITE, { conclusion: 'failure', message: null }),
      { jobs: DEPLOY_FAILED }
    );

    expect(result.status).toBe(0);
    const input = result.writes[0]?.input as IssueInput;
    expect(input.body).toContain('**Commit:** (no commit message');
  });

  test('of CI keeps the title and the trunk-only jobs line it always had', () => {
    const result = alarm(workflowRun(CI, { conclusion: 'failure' }), {
      jobs: [
        {
          name: 'migrate',
          conclusion: 'failure',
          steps: [{ name: 'Run migrations', conclusion: 'failure' }],
        },
      ],
    });

    expect(result.status).toBe(0);
    const input = result.writes[0]?.input as IssueInput;
    expect(input.title).toBe('Trunk is red: CI failed on 8b41853e');
    expect(input.body).toContain(
      '- **migrate** - failed at step _Run migrations_'
    );
    expect(input.body).toContain(
      'The migrate, deploy-api, deploy-app, deploy-ops and deploy-ops-site jobs run ONLY on trunk'
    );
    expect(input.body).toContain(marker(CI));
  });

  test('comments on the open issue of the same workflow', () => {
    const result = alarm(workflowRun(SITE, { conclusion: 'failure' }), {
      jobs: DEPLOY_FAILED,
      issues: [openAlarm(1700, SITE)],
    });

    expect(result.status).toBe(0);
    expect(result.writes).toHaveLength(1);
    expect(result.writes[0]?.method).toBe('POST');
    expect(result.writes[0]?.endpoint).toBe(
      `repos/${REPO}/issues/1700/comments`
    );
    expect((result.writes[0]?.input as CommentInput).body).toContain(
      '- **deploy** - failed at step _Deploy to Firebase_'
    );
  });

  test('of the site deploy opens its own issue while a CI alarm is open', () => {
    const result = alarm(workflowRun(SITE, { conclusion: 'failure' }), {
      jobs: DEPLOY_FAILED,
      issues: [openAlarm(1622, CI)],
    });

    expect(result.status).toBe(0);
    expect(result.writes.map((call) => call.endpoint)).toEqual([
      `repos/${REPO}/issues`,
    ]);
  });

  test('of CI opens its own issue while a site alarm is open', () => {
    const result = alarm(workflowRun(CI, { conclusion: 'failure' }), {
      issues: [openAlarm(1700, SITE)],
    });

    expect(result.status).toBe(0);
    expect(result.writes.map((call) => call.endpoint)).toEqual([
      `repos/${REPO}/issues`,
    ]);
  });

  test('of CI comments on an open issue from before the marker existed', () => {
    const result = alarm(workflowRun(CI, { conclusion: 'failure' }), {
      issues: [{ number: 1622, body: 'Trunk CI failed on **462d3b94**.' }],
    });

    expect(result.writes.map((call) => call.endpoint)).toEqual([
      `repos/${REPO}/issues/1622/comments`,
    ]);
  });

  test('of the site deploy does not take an issue from before the marker existed', () => {
    const result = alarm(workflowRun(SITE, { conclusion: 'failure' }), {
      jobs: DEPLOY_FAILED,
      issues: [{ number: 1622, body: 'Trunk CI failed on **462d3b94**.' }],
    });

    expect(result.writes.map((call) => call.endpoint)).toEqual([
      `repos/${REPO}/issues`,
    ]);
  });

  test('does not take a pull request that carries the label', () => {
    const result = alarm(workflowRun(SITE, { conclusion: 'failure' }), {
      jobs: DEPLOY_FAILED,
      issues: [{ ...openAlarm(1701, SITE), pull_request: {} }],
    });

    expect(result.writes.map((call) => call.endpoint)).toEqual([
      `repos/${REPO}/issues`,
    ]);
  });

  test.each(['timed_out', 'startup_failure'])(
    'with the conclusion %s opens the alarm and names the conclusion',
    (conclusion) => {
      const result = alarm(workflowRun(SITE, { conclusion }), { jobs: [] });

      expect(result.status).toBe(0);
      expect(result.writes).toHaveLength(1);
      const input = result.writes[0]?.input as IssueInput;
      expect(input.body).toContain(`\`${conclusion}\``);
      expect(input.body).toContain('no job reported a failure');
    }
  );

  test('names a failed job that has no failed step', () => {
    const result = alarm(workflowRun(SITE, { conclusion: 'failure' }), {
      jobs: [{ name: 'build', conclusion: 'timed_out', steps: [] }],
    });

    const input = result.writes[0]?.input as IssueInput;
    expect(input.body).toContain(
      '- **build** - `timed_out`, with no failed step'
    );
  });

  test.each([1, 2])(
    'reads the jobs of the attempt that failed (%i), not of a rerun that started since (#2504)',
    (attempt) => {
      const result = alarm(
        workflowRun(CI, { conclusion: 'failure', attempt }),
        {
          jobs: [
            {
              name: 'api-image',
              conclusion: 'failure',
              steps: [{ name: 'Build and push', conclusion: 'failure' }],
            },
          ],
        }
      );

      expect(result.status).toBe(0);
      expect(result.calls.map((call) => call.endpoint)).toContain(
        `repos/${REPO}/actions/runs/7001/attempts/${attempt}/jobs?per_page=100`
      );
      expect((result.writes[0]?.input as IssueInput).body).toContain(
        '- **api-image** - failed at step _Build and push_'
      );
    }
  );

  test('records the failing commit, and says that only a pass on that commit closes the issue', () => {
    const result = alarm(workflowRun(CI, { conclusion: 'failure' }));

    const input = result.writes[0]?.input as IssueInput;
    expect(input.body).toContain(shaMarker(SHA));
    expect(input.body).toContain('gh run rerun 7001 --failed');
    expect(input.body).toContain('passes on this same commit');
  });

  test('on a later commit records that commit in its comment on the open issue', () => {
    const result = alarm(
      workflowRun(CI, { conclusion: 'failure', sha: LATER_SHA }),
      { issues: [openAlarm(1622, CI)] }
    );

    expect(result.writes.map((call) => call.endpoint)).toEqual([
      `repos/${REPO}/issues/1622/comments`,
    ]);
    expect((result.writes[0]?.input as CommentInput).body).toContain(
      shaMarker(LATER_SHA)
    );
  });

  test('of Terraform Plan on a push says what a red plan means, where the drift table is, and the fix', () => {
    const result = alarm(workflowRun(TERRAFORM, { conclusion: 'failure' }), {
      jobs: [
        {
          name: 'plan',
          conclusion: 'failure',
          steps: [{ name: 'Terraform plan', conclusion: 'failure' }],
        },
      ],
    });

    expect(result.status).toBe(0);
    const input = result.writes[0]?.input as IssueInput;
    expect(input.title).toBe('Trunk is red: Terraform Plan failed on 8b41853e');
    expect(input.body).toContain(
      '- **plan** - failed at step _Terraform plan_'
    );
    expect(input.body).not.toContain('no deploy is reaching the environment');
    expect(input.body).not.toContain('not reaching the environment');
    expect(input.body).toContain('drift');
    expect(input.body).toContain(
      'gh api repos/markgoho/doula-cloud/actions/jobs/<id>/logs --allow-escape-sequences'
    );
    expect(input.body).toContain('laptop `apply`');
    expect(input.body).toContain('docs/infrastructure.md');
    expect(input.body).toContain(marker(TERRAFORM));
  });

  describe('of Terraform Plan on a commit that changed what its job plans (#2575)', () => {
    const planJob = (name: string): Job => ({
      name,
      conclusion: 'failure',
      steps: [{ name: 'Terraform plan', conclusion: 'failure' }],
    });
    const bodyOf = (jobName: string, commitFiles: string[]) =>
      (
        alarm(workflowRun(TERRAFORM, { conclusion: 'failure' }), {
          jobs: [planJob(jobName)],
          commitFiles,
        }).writes[0]?.input as IssueInput
      ).body;

    test('leads with waiting for its apply, the files, and the commands', () => {
      const result = alarm(workflowRun(TERRAFORM, { conclusion: 'failure' }), {
        jobs: [planJob('plan')],
        commitFiles: ['terraform/cloud_run.tf', 'README.md'],
      });
      const body = (result.writes[0]?.input as IssueInput).body;

      expect(result.calls.map((call) => call.endpoint)).toContain(
        `repos/${REPO}/commits/${SHA}`
      );
      expect(body.startsWith('**This commit changed Terraform')).toBe(true);
      expect(body).toContain('waiting for its apply');
      expect(body).toContain('- `terraform/cloud_run.tf`');
      expect(body).not.toContain('README.md');
      expect(body).toContain('git pull');
      expect(body).toContain('cd terraform && terraform apply');
      expect(body).toContain('gh run rerun 7001 --failed');
      expect(body).toContain('- **plan** - failed at step _Terraform plan_');
      expect(body).toContain(marker(TERRAFORM));
    });

    test('the root plan does not count a Stripe folder', () => {
      const body = bodyOf('plan', [
        'terraform/stripe-sandbox/main.tf',
        'terraform/stripe-live/main.tf',
        'terraform/stripe/products.tf',
      ]);
      expect(body).not.toContain('waiting for its apply');
      expect(body).toContain('drift');
    });

    test('the Sandbox plan applies from terraform/stripe-sandbox', () => {
      const body = bodyOf('plan-stripe-sandbox', [
        'terraform/stripe/products.tf',
        'terraform/stripe-sandbox/main.tf',
        'terraform/cloud_run.tf',
      ]);
      expect(body).toContain('waiting for its apply');
      expect(body).toContain('- `terraform/stripe/products.tf`');
      expect(body).toContain('- `terraform/stripe-sandbox/main.tf`');
      expect(body).not.toContain('cloud_run.tf');
      expect(body).toContain('cd terraform/stripe-sandbox && terraform apply');
    });

    test('the Sandbox plan ignores a change to the root only', () => {
      const body = bodyOf('plan-stripe-sandbox', ['terraform/cloud_run.tf']);
      expect(body).not.toContain('waiting for its apply');
    });

    test('a job the workflow does not plan gets the drift text', () => {
      const body = bodyOf('something-else', ['terraform/cloud_run.tf']);
      expect(body).not.toContain('waiting for its apply');
    });

    test('a commit with no Terraform change gets the drift text unchanged', () => {
      const body = bodyOf('plan', ['api/main.go', 'docs/infrastructure.md']);
      expect(body).not.toContain('waiting for its apply');
      expect(body.startsWith('**Terraform Plan** failed on trunk')).toBe(true);
      expect(body).toContain('drift');
    });

    test('a scheduled run reads no commit', () => {
      const result = alarm(
        workflowRun(TERRAFORM, { conclusion: 'failure', event: 'schedule' }),
        { jobs: [planJob('plan')], commitFiles: ['terraform/cloud_run.tf'] }
      );
      expect(
        result.calls.some((call) => call.endpoint.includes('/commits/'))
      ).toBe(false);
      expect((result.writes[0]?.input as IssueInput).body).not.toContain(
        'waiting for its apply'
      );
    });

    test('another workflow reads no commit', () => {
      const result = alarm(workflowRun(CI, { conclusion: 'failure' }), {
        jobs: [planJob('plan')],
        commitFiles: ['terraform/cloud_run.tf'],
      });
      expect(
        result.calls.some((call) => call.endpoint.includes('/commits/'))
      ).toBe(false);
    });

    test('an unreadable commit still opens the alarm with the drift text', () => {
      const result = alarm(workflowRun(TERRAFORM, { conclusion: 'failure' }), {
        jobs: [planJob('plan')],
        commitFail: true,
      });
      expect(result.status).toBe(0);
      expect(result.stdout).toContain('::warning::');
      expect((result.writes[0]?.input as IssueInput).body).not.toContain(
        'waiting for its apply'
      );
    });

    test('unreadable jobs mean no apply text', () => {
      const result = alarm(workflowRun(TERRAFORM, { conclusion: 'failure' }), {
        jobsFail: true,
        commitFiles: ['terraform/cloud_run.tf'],
      });
      expect((result.writes[0]?.input as IssueInput).body).not.toContain(
        'waiting for its apply'
      );
    });
  });

  test('of the daily Terraform Plan says it was not caused by the trunk head', () => {
    const result = alarm(
      workflowRun(TERRAFORM, { conclusion: 'failure', event: 'schedule' })
    );

    const input = result.writes[0]?.input as IssueInput;
    expect(input.title).toBe(
      'Trunk is red: Terraform Plan failed on a schedule run'
    );
    expect(input.body).toContain('a `schedule` event, not a push');
    expect(input.body).not.toContain('**Commit:**');
  });

  test('still opens the alarm when the jobs cannot be read', () => {
    const result = alarm(workflowRun(SITE, { conclusion: 'failure' }), {
      jobsFail: true,
    });

    expect(result.status).toBe(0);
    expect(result.writes).toHaveLength(1);
    const input = result.writes[0]?.input as IssueInput;
    expect(input.body).toContain('the jobs of this run could not be read');
    expect(input.body).toContain(`**Run:** ${RUN_URL}`);
  });
});

describe('a passing run', () => {
  test('of the site deploy closes the site alarm with a comment, and leaves the CI alarm open', () => {
    const result = alarm(workflowRun(SITE, { conclusion: 'success' }), {
      issues: [openAlarm(1622, CI), openAlarm(1700, SITE)],
    });

    expect(result.status).toBe(0);
    expect(
      result.writes.map((call) => `${call.method} ${call.endpoint}`)
    ).toEqual([
      `POST repos/${REPO}/issues/1700/comments`,
      `PATCH repos/${REPO}/issues/1700`,
    ]);
    expect((result.writes[0]?.input as CommentInput).body).toContain(
      `Deploy the marketing site to Firebase Hosting on merge passed on trunk at 8b41853e, the commit that failed - ${RUN_URL}`
    );
    expect(result.writes[1]?.input).toEqual({
      state: 'closed',
      state_reason: 'completed',
    });
  });

  test('of CI closes the CI alarm, and leaves the site alarm open', () => {
    const result = alarm(workflowRun(CI, { conclusion: 'success' }), {
      issues: [openAlarm(1622, CI), openAlarm(1700, SITE)],
    });

    expect(result.status).toBe(0);
    expect(
      result.writes.map((call) => `${call.method} ${call.endpoint}`)
    ).toEqual([
      `POST repos/${REPO}/issues/1622/comments`,
      `PATCH repos/${REPO}/issues/1622`,
    ]);
  });

  test('of CI closes nothing when the site alarm is the only one open', () => {
    const result = alarm(workflowRun(CI, { conclusion: 'success' }), {
      issues: [openAlarm(1700, SITE)],
    });

    expect(result.status).toBe(0);
    expect(result.writes).toEqual([]);
  });

  test('of a dispatched site deploy closes the alarm that a push run opened', () => {
    const result = alarm(
      workflowRun(SITE, {
        conclusion: 'success',
        event: 'repository_dispatch',
        message: null,
      }),
      { issues: [openAlarm(1700, SITE)] }
    );

    expect(result.status).toBe(0);
    expect(result.writes.map((call) => call.endpoint)).toEqual([
      `repos/${REPO}/issues/1700/comments`,
      `repos/${REPO}/issues/1700`,
    ]);
  });

  test('with no alarm open writes nothing', () => {
    const result = alarm(workflowRun(SITE, { conclusion: 'success' }));

    expect(result.status).toBe(0);
    expect(result.writes).toEqual([]);
  });

  test('on a different commit leaves the alarm open, with a comment that names both commits (#2504)', () => {
    const result = alarm(
      workflowRun(CI, { conclusion: 'success', sha: LATER_SHA }),
      { issues: [openAlarm(1622, CI)] }
    );

    expect(result.status).toBe(0);
    expect(
      result.writes.map((call) => `${call.method} ${call.endpoint}`)
    ).toEqual([`POST repos/${REPO}/issues/1622/comments`]);
    const body = (result.writes[0]?.input as CommentInput).body;
    expect(body).toContain(LATER_SHA.slice(0, 8));
    expect(body).toContain(SHA.slice(0, 8));
    expect(body).toContain('never reproduced');
    expect(body).toContain('gh run rerun');
  });

  test('on a different commit says so only once', () => {
    const first = alarm(
      workflowRun(CI, { conclusion: 'success', sha: LATER_SHA }),
      { issues: [openAlarm(1622, CI)] }
    );
    const said = (first.writes[0]?.input as CommentInput).body;
    fs.rmSync(path.join(dir, 'calls.jsonl'));

    const result = alarm(
      workflowRun(CI, {
        conclusion: 'success',
        sha: 'c0ffee00c0ffee00c0ffee00c0ffee00c0ffee00',
      }),
      { issues: [openAlarm(1622, CI)], comments: { 1622: [{ body: said }] } }
    );

    expect(result.status).toBe(0);
    expect(result.writes).toEqual([]);
  });

  test('on a later commit that also failed closes the alarm, because that commit is recorded in a comment', () => {
    const result = alarm(
      workflowRun(CI, { conclusion: 'success', sha: LATER_SHA }),
      {
        issues: [openAlarm(1622, CI)],
        comments: {
          1622: [{ body: `It failed again.\n\n${shaMarker(LATER_SHA)}` }],
        },
      }
    );

    expect(
      result.writes.map((call) => `${call.method} ${call.endpoint}`)
    ).toEqual([
      `POST repos/${REPO}/issues/1622/comments`,
      `PATCH repos/${REPO}/issues/1622`,
    ]);
  });

  test('does not close an issue that records no commit', () => {
    const result = alarm(workflowRun(CI, { conclusion: 'success' }), {
      issues: [{ number: 1622, body: 'Trunk CI failed on **462d3b94**.' }],
    });

    expect(
      result.writes.map((call) => `${call.method} ${call.endpoint}`)
    ).toEqual([`POST repos/${REPO}/issues/1622/comments`]);
    expect((result.writes[0]?.input as CommentInput).body).toContain(
      'never reproduced'
    );
  });
});

describe('a canceled run', () => {
  test('of the site deploy on a push opens no alarm, and comments on the pull request in the commit subject', () => {
    const result = alarm(workflowRun(SITE, { conclusion: CANCELED }), {
      issues: [openAlarm(1700, SITE)],
    });

    expect(result.status).toBe(0);
    expect(result.writes).toHaveLength(1);
    expect(result.writes[0]?.endpoint).toBe(
      `repos/${REPO}/issues/1660/comments`
    );
    const body = (result.writes[0]?.input as CommentInput).body;
    expect(body).toContain(
      'Deploy the marketing site to Firebase Hosting on merge'
    );
    expect(body).toContain('was canceled');
    expect(body).toContain(`git merge-base --is-ancestor ${SHA} <later-sha>`);
    expect(body).toContain(`**Canceled run:** ${RUN_URL}`);
  });

  test('of CI comments on the pull request and names the three trunk-only jobs', () => {
    const result = alarm(workflowRun(CI, { conclusion: CANCELED }));

    expect(result.status).toBe(0);
    expect(result.writes.map((call) => call.endpoint)).toEqual([
      `repos/${REPO}/issues/1660/comments`,
    ]);
    expect((result.writes[0]?.input as CommentInput).body).toContain(
      'migrate, deploy-api, deploy-app, deploy-ops and deploy-ops-site have no evidence for this commit'
    );
  });

  test('on a dispatch writes nothing: the trunk head names a pull request that the run does not belong to', () => {
    const result = alarm(
      workflowRun(SITE, {
        conclusion: CANCELED,
        event: 'repository_dispatch',
      })
    );

    expect(result.status).toBe(0);
    expect(result.calls).toEqual([]);
    expect(result.stdout).toContain('::notice::');
  });

  test('on a push whose subject names no pull request writes nothing and warns', () => {
    const result = alarm(
      workflowRun(SITE, { conclusion: CANCELED, message: 'A direct push' })
    );

    expect(result.status).toBe(0);
    expect(result.calls).toEqual([]);
    expect(result.stdout).toContain('::warning::');
  });
});

describe('any run', () => {
  test('prints one line that names the workflow, the event, the conclusion and the head', () => {
    const result = alarm(workflowRun(SITE, { conclusion: 'success' }));

    expect(result.stdout).toContain(
      'trunk-red: workflow="Deploy the marketing site to Firebase Hosting on merge" path=.github/workflows/firebase-hosting-merge.yml event=push conclusion=success head=8b41853e'
    );
  });

  test.each(['skipped', 'neutral', 'action_required', 'stale'])(
    'with the conclusion %s touches nothing',
    (conclusion) => {
      const result = alarm(workflowRun(SITE, { conclusion }), {
        issues: [openAlarm(1700, SITE)],
      });

      expect(result.status).toBe(0);
      expect(result.calls).toEqual([]);
    }
  );

  test('fails when a write is refused, so that the alarm is not silently lost', () => {
    const result = alarm(
      workflowRun(SITE, { conclusion: 'failure' }),
      {},
      { GH_BIN: '/usr/bin/false' }
    );

    expect(result.status).not.toBe(0);
  });

  test('fails on an event that holds no workflow run', () => {
    const result = alarm(null);

    expect(result.status).not.toBe(0);
    expect(result.stderr).toContain('no workflow_run');
  });
});

describe('a replay', () => {
  test('reads the run from the API and prints what the alarm would do, without a write', () => {
    const result = alarm(
      null,
      {
        run: workflowRun(SITE, { conclusion: 'failure' }),
        jobs: DEPLOY_FAILED,
        issues: [openAlarm(1622, CI)],
      },
      { REPLAY_RUN_ID: '7001' }
    );

    expect(result.status).toBe(0);
    expect(result.calls[0]?.endpoint).toBe(`repos/${REPO}/actions/runs/7001`);
    expect(result.writes).toEqual([]);
    expect(result.stdout).toContain(`dry run: POST repos/${REPO}/issues`);
    expect(result.stdout).toContain(
      'Trunk is red: Deploy the marketing site to Firebase Hosting on merge failed on 8b41853e'
    );
    expect(result.stdout).toContain(
      '- **deploy** - failed at step _Deploy to Firebase_'
    );
  });

  test('of a passing run prints the close it would make', () => {
    const result = alarm(
      null,
      {
        run: workflowRun(CI, { conclusion: 'success' }),
        issues: [openAlarm(1622, CI)],
      },
      { REPLAY_RUN_ID: '7001' }
    );

    expect(result.status).toBe(0);
    expect(result.writes).toEqual([]);
    expect(result.stdout).toContain(`dry run: PATCH repos/${REPO}/issues/1622`);
  });

  test('refuses a run id that is not a number', () => {
    const result = alarm(null, {}, { REPLAY_RUN_ID: '7001/jobs' });

    expect(result.status).not.toBe(0);
    expect(result.calls).toEqual([]);
  });
});

describe('.github/workflows/trunk-red.yml', () => {
  const source = fs.readFileSync(path.join(WORKFLOWS, 'trunk-red.yml'), 'utf8');
  const workflow = Bun.YAML.parse(source) as {
    on: { workflow_run: { workflows: string[]; branches: string[] } };
  };
  const watched = workflow.on.workflow_run.workflows;

  const names = new Map(
    fs
      .readdirSync(WORKFLOWS)
      .filter((file) => file.endsWith('.yml'))
      .map((file) => {
        const parsed = Bun.YAML.parse(
          fs.readFileSync(path.join(WORKFLOWS, file), 'utf8')
        ) as { name?: string };
        return [parsed.name, file] as const;
      })
  );

  // `workflow_run` matches a workflow by its `name:`. A name here that no
  // file holds matches nothing, and nothing says so: the alarm for that
  // workflow is silently off.
  test('each watched name is the name of a workflow file', () => {
    expect(watched.filter((name) => !names.has(name))).toEqual([]);
  });

  test('watches CI and the marketing site deploy', () => {
    const files = watched.map((name) => names.get(name));
    expect(files).toContain('ci.yml');
    expect(files).toContain('firebase-hosting-merge.yml');
  });

  test('watches the daily Hosting edge check (#1867)', () => {
    expect(watched.map((name) => names.get(name))).toContain(
      'hosting-edge-check.yml'
    );
  });

  test('watches Terraform Plan (#2504)', () => {
    expect(watched.map((name) => names.get(name))).toContain(
      'terraform-plan.yml'
    );
  });

  test('watches trunk only', () => {
    expect(workflow.on.workflow_run.branches).toEqual(['trunk']);
  });
});
