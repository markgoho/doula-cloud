/**
 * `scripts/wipe-practice-data.sh` -- the production wipe (#2408).
 *
 * Every refusal is checked against the real script with every binary it
 * runs replaced by a stand-in that records its call. A refusal must come
 * before the first call: nothing is reached until the targets agree and
 * the confirmation repeats the instance.
 */
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const SCRIPT = path.join(import.meta.dir, 'wipe-practice-data.sh');
const SCRATCH = 'doula-cloud:us-central1:doula-cloud-pg-scratch';
const PRODUCTION = 'doula-cloud:us-central1:doula-cloud-pg';

let dir: string;
let calls: string;

beforeEach(() => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'wipe-practice-data-test-'));
  calls = path.join(dir, 'calls');
});

afterEach(() => {
  fs.rmSync(dir, { recursive: true, force: true });
});

function standIn(name: string): string {
  const bin = path.join(dir, name);
  fs.writeFileSync(
    bin,
    ['#!/usr/bin/env bash', `echo "${name} $*" >> "${calls}"`].join('\n')
  );
  fs.chmodSync(bin, 0o755);
  return bin;
}

const SCRATCH_TARGETS = {
  WIPE_INSTANCE: SCRATCH,
  WIPE_DSN: 'postgres://user:pass@_/doula_cloud',
  WIPE_BUCKET: 'doula-cloud-attachments-scratch',
  WIPE_AUTH_PROJECT: 'doula-cloud',
  WIPE_AUTH_EMULATOR_HOST: '127.0.0.1:9099',
};

function run(
  targets: Record<string, string | undefined>,
  typed: string
): { status: number | null; stderr: string; called: boolean } {
  const env: Record<string, string> = {
    PATH: process.env.PATH ?? '',
    CLOUD_SQL_PROXY_BIN: standIn('cloud-sql-proxy'),
    GCLOUD_BIN: standIn('gcloud'),
    PSQL_BIN: standIn('psql'),
    CURL_BIN: standIn('curl'),
    GOOSE_BIN: standIn('goose'),
  };
  for (const [key, value] of Object.entries(targets)) {
    if (value !== undefined) env[key] = value;
  }
  const result = spawnSync('bash', [SCRIPT], {
    env,
    input: typed,
    encoding: 'utf8',
  });
  return {
    status: result.status,
    stderr: result.stderr,
    called: fs.existsSync(calls),
  };
}

describe('wipe-practice-data.sh', () => {
  test.each(['WIPE_INSTANCE', 'WIPE_DSN', 'WIPE_BUCKET', 'WIPE_AUTH_PROJECT'])(
    'refuses to run without %s',
    (name) => {
      const result = run({ ...SCRATCH_TARGETS, [name]: undefined }, SCRATCH);

      expect(result.status).not.toBe(0);
      expect(result.stderr).toContain(name);
      expect(result.called).toBe(false);
    }
  );

  test('refuses a confirmation that does not repeat the instance', () => {
    const result = run(SCRATCH_TARGETS, 'doula-cloud-pg-scratch\n');

    expect(result.status).not.toBe(0);
    expect(result.stderr).toContain('did not match');
    expect(result.called).toBe(false);
  });

  test('refuses when nothing is typed', () => {
    const result = run(SCRATCH_TARGETS, '');

    expect(result.status).not.toBe(0);
    expect(result.called).toBe(false);
  });

  test('refuses an instance that is not a connection name', () => {
    const result = run(
      { ...SCRATCH_TARGETS, WIPE_INSTANCE: 'doula-cloud-pg-scratch' },
      'doula-cloud-pg-scratch\n'
    );

    expect(result.status).not.toBe(0);
    expect(result.stderr).toContain('project:region:instance');
    expect(result.called).toBe(false);
  });

  test('refuses a bucket given as a URL', () => {
    const result = run(
      {
        ...SCRATCH_TARGETS,
        WIPE_BUCKET: 'gs://doula-cloud-attachments-scratch',
      },
      `${SCRATCH}\n`
    );

    expect(result.status).not.toBe(0);
    expect(result.called).toBe(false);
  });

  test.each([
    ['the production instance', { WIPE_INSTANCE: PRODUCTION }, PRODUCTION],
    [
      'the production bucket',
      { WIPE_BUCKET: 'doula-cloud-attachments' },
      SCRATCH,
    ],
    [
      'production Identity Platform',
      { WIPE_AUTH_EMULATOR_HOST: undefined },
      SCRATCH,
    ],
    [
      'the production queue',
      {
        WIPE_TASKS_QUEUE:
          'projects/doula-cloud/locations/us-central1/queues/doula-cloud-notification-nudge',
      },
      SCRATCH,
    ],
  ])('refuses %s among scratch targets', (_, change, typed) => {
    const result = run({ ...SCRATCH_TARGETS, ...change }, `${typed}\n`);

    expect(result.status).not.toBe(0);
    expect(result.stderr).toContain('refusing a mix of production');
    expect(result.called).toBe(false);
  });

  test('starts the proxy for the confirmed instance and nothing before it', () => {
    run(SCRATCH_TARGETS, `${SCRATCH}\n`);

    const first = fs.readFileSync(calls, 'utf8').split('\n')[0];
    expect(first).toBe(`cloud-sql-proxy --port 15432 ${SCRATCH}`);
  });
});
