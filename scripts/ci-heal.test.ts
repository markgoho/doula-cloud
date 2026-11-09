/**
 * `scripts/ci-heal.ts` -- what `.github/workflows/ci-heal.yml` runs when a
 * watched workflow completes (#2819). `workflow_run` runs from the default
 * branch only, so these tests are the proof in place of a pull request:
 * each spawns the real script against a stand-in `gh` (the `GH_BIN` seam)
 * that answers from real log excerpts and records every call.
 */
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {
  BUN_504,
  COVCHECK_FAILED,
  TERRAFORM_504,
} from './ci-transient.fixtures.ts';

const ROOT = path.join(import.meta.dir, '..');
const SCRIPT = path.join(import.meta.dir, 'ci-heal.ts');
const WORKFLOWS = path.join(ROOT, '.github', 'workflows');
const REPO = 'markgoho/doula-cloud';

let dir: string;

beforeEach(() => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ci-heal-test-'));
});

afterEach(() => {
  fs.rmSync(dir, { recursive: true, force: true });
});

type Call = { method: string; endpoint: string };

/** A stand-in `gh api`: jobs and logs from the fixtures, every call recorded. */
function standIn(jobs: object[], logs: Record<number, string>, run = {}) {
  const bin = path.join(dir, 'gh');
  fs.writeFileSync(
    path.join(dir, 'fixtures.json'),
    JSON.stringify({ jobs, logs, run })
  );
  fs.writeFileSync(
    bin,
    [
      '#!/usr/bin/env bun',
      "import fs from 'node:fs';",
      "import path from 'node:path';",
      'const args = process.argv.slice(2);',
      "const method = args.includes('--method') ? args[args.indexOf('--method') + 1] : 'GET';",
      "const endpoint = args.filter((a) => a.startsWith('repos/'))[0];",
      "const f = JSON.parse(fs.readFileSync(path.join(import.meta.dir, 'fixtures.json'), 'utf8'));",
      "fs.appendFileSync(path.join(import.meta.dir, 'calls.jsonl'), JSON.stringify({ method, endpoint }) + '\\n');",
      'const log = endpoint.match(/\\/jobs\\/(\\d+)\\/logs$/);',
      "if (log) console.log(f.logs[log[1]] ?? '');",
      'else if (/\\/attempts\\/\\d+\\/jobs/.test(endpoint)) console.log(JSON.stringify({ jobs: f.jobs }));',
      'else if (/\\/actions\\/runs\\/\\d+$/.test(endpoint)) console.log(JSON.stringify(f.run));',
      "else console.log('{}');",
      '',
    ].join('\n'),
    { mode: 0o755 }
  );
  return bin;
}

const job = (id: number, name: string, step: string) => ({
  id,
  name,
  conclusion: 'failure',
  steps: [{ name: step, conclusion: 'failure' }],
});

function heal(
  run: object,
  jobs: object[],
  logs: Record<number, string>,
  env: Record<string, string> = {}
) {
  const eventPath = path.join(dir, 'event.json');
  fs.writeFileSync(eventPath, JSON.stringify({ workflow_run: run }));
  const result = spawnSync('bun', [SCRIPT], {
    encoding: 'utf8',
    cwd: dir,
    env: {
      ...process.env,
      GH_BIN: standIn(jobs, logs, run),
      GITHUB_EVENT_PATH: eventPath,
      REPO,
      REPLAY_RUN_ID: '',
      ...env,
    },
  });
  const callsFile = path.join(dir, 'calls.jsonl');
  const calls: Call[] = fs.existsSync(callsFile)
    ? fs
        .readFileSync(callsFile, 'utf8')
        .split('\n')
        .filter((line) => line !== '')
        .map((line) => JSON.parse(line) as Call)
    : [];
  return {
    status: result.status,
    stdout: result.stdout,
    reruns: calls.filter((call) => call.method === 'POST'),
  };
}

const run = (conclusion: string, attempt = 1) => ({
  id: 7001,
  name: 'CI',
  conclusion,
  run_attempt: attempt,
  html_url: 'https://github.com/markgoho/doula-cloud/actions/runs/7001',
});

describe('a failed run', () => {
  test('of setup-bun 504s is rerun once, failed jobs only', () => {
    const result = heal(
      run('failure'),
      [job(11, 'format', 'Run oven-sh/setup-bun@v2')],
      { 11: BUN_504 }
    );
    expect(result.status).toBe(0);
    expect(result.reruns).toEqual([
      {
        method: 'POST',
        endpoint: `repos/${REPO}/actions/runs/7001/rerun-failed-jobs`,
      },
    ]);
  });

  test('of a Terraform registry 504 is rerun', () => {
    const result = heal(
      { ...run('failure'), name: 'Terraform Plan' },
      [job(12, 'plan-stripe-sandbox', 'Validate the Stripe roots')],
      { 12: TERRAFORM_504 }
    );
    expect(result.reruns).toHaveLength(1);
  });

  test('of a failing coverage check is never rerun', () => {
    const result = heal(
      run('failure'),
      [job(13, 'api', 'Enforce 100% line coverage')],
      { 13: COVCHECK_FAILED }
    );
    expect(result.status).toBe(0);
    expect(result.reruns).toEqual([]);
    expect(result.stdout).toContain('verdict: not transient');
  });

  test('on its second attempt is left alone, though it is transient', () => {
    const result = heal(
      run('failure', 2),
      [job(11, 'format', 'Run oven-sh/setup-bun@v2')],
      { 11: BUN_504 }
    );
    expect(result.reruns).toEqual([]);
    expect(result.stdout).toContain('left alone');
  });

  test.each(['success', 'cancelled', 'timed_out'])(
    // spelling:ignore: the value GitHub's API sends
    'with the conclusion %s writes nothing',
    (conclusion) => {
      const result = heal(
        run(conclusion),
        [job(11, 'format', 'Run oven-sh/setup-bun@v2')],
        { 11: BUN_504 }
      );
      expect(result.reruns).toEqual([]);
    }
  );
});

describe('a replay', () => {
  test('reads attempt 1 of a run that was rerun by hand and writes nothing', () => {
    const result = heal(
      run('failure', 2),
      [job(11, 'format', 'Run oven-sh/setup-bun@v2')],
      { 11: BUN_504 },
      { REPLAY_RUN_ID: '7001' }
    );
    expect(result.status).toBe(0);
    expect(result.reruns).toEqual([]);
    expect(result.stdout).toContain('verdict: transient');
    expect(result.stdout).toContain(
      `dry run: would POST repos/${REPO}/actions/runs/7001/rerun-failed-jobs`
    );
  });

  test('refuses a run id that is not a number', () => {
    const result = heal(run('failure'), [], {}, { REPLAY_RUN_ID: '7001; rm' });
    expect(result.status).toBe(1);
  });
});

describe('.github/workflows/ci-heal.yml', () => {
  const load = (file: string) =>
    Bun.YAML.parse(fs.readFileSync(path.join(WORKFLOWS, file), 'utf8')) as {
      on: { workflow_run: { workflows: string[]; branches?: string[] } };
      permissions: Record<string, string>;
    };
  const heals = load('ci-heal.yml');
  const alarm = load('trunk-red.yml');

  test('watches the workflows that trunk-red watches, on every branch', () => {
    expect(heals.on.workflow_run.workflows).toEqual(
      alarm.on.workflow_run.workflows
    );
    expect(heals.on.workflow_run.branches).toBeUndefined();
  });

  test('can rerun a run, and write nothing else', () => {
    expect(heals.permissions).toEqual({ contents: 'read', actions: 'write' });
  });
});
