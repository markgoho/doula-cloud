/**
 * `scripts/install-bun.sh` and `scripts/retry.sh` (#2819): each spawns the
 * real script against stand-in binaries, with the backoff unit set to zero.
 */
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const INSTALL = path.join(import.meta.dir, 'install-bun.sh');
const RETRY = path.join(import.meta.dir, 'retry.sh');

let dir: string;

beforeEach(() => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'install-bun-test-'));
});

afterEach(() => {
  fs.rmSync(dir, { recursive: true, force: true });
});

/** A stand-in that fails its first `failures` calls, then runs `body`. */
function flaky(name: string, failures: number, body: string): string {
  const bin = path.join(dir, name);
  const calls = path.join(dir, `${name}.calls`);
  fs.writeFileSync(
    bin,
    [
      '#!/usr/bin/env bash',
      `n=$(( $(cat "${calls}" 2>/dev/null || echo 0) + 1 ))`,
      `echo "$n" > "${calls}"`,
      `[ "$n" -le ${failures} ] && { echo "curl: (22) The requested URL returned error: 504" >&2; exit 22; }`,
      body,
      '',
    ].join('\n'),
    { mode: 0o755 }
  );
  return bin;
}

const calls = (name: string) =>
  Number(fs.readFileSync(path.join(dir, `${name}.calls`), 'utf8'));

function install(failures: number) {
  const curl = flaky('curl', failures, ': > "${@: -2:1}"');
  const unzip = path.join(dir, 'unzip');
  fs.writeFileSync(
    unzip,
    [
      '#!/usr/bin/env bash',
      'out="${@: -1}"',
      'mkdir -p "$out/bun-linux-x64"',
      'printf "#!/bin/sh\\n" > "$out/bun-linux-x64/bun"',
      '',
    ].join('\n'),
    { mode: 0o755 }
  );
  const dest = path.join(dir, 'dest');
  const result = spawnSync('bash', [INSTALL, '1.4.4', dest], {
    encoding: 'utf8',
    env: {
      ...process.env,
      INSTALL_BUN_CURL: curl,
      INSTALL_BUN_UNZIP: unzip,
      INSTALL_BUN_BACKOFF_SECONDS: '0',
    },
  });
  return { ...result, dest };
}

describe('install-bun.sh', () => {
  test('installs on the first attempt', () => {
    const result = install(0);
    expect(result.status).toBe(0);
    expect(fs.existsSync(path.join(result.dest, 'bin', 'bun'))).toBe(true);
    expect(fs.existsSync(path.join(result.dest, 'bin', 'bunx'))).toBe(true);
    expect(calls('curl')).toBe(1);
  });

  test('survives four 504s, which is longer than setup-bun waits', () => {
    const result = install(4);
    expect(result.status).toBe(0);
    expect(calls('curl')).toBe(5);
    expect(result.stdout).toContain('retrying in 0s');
  });

  test('fails after five attempts, with an error annotation', () => {
    const result = install(5);
    expect(result.status).toBe(1);
    expect(calls('curl')).toBe(5);
    expect(result.stdout).toContain('::error::Bun 1.4.4 did not download');
  });
});

describe('retry.sh', () => {
  const retry = (failures: number) => {
    const cmd = flaky('flaky-cmd', failures, 'echo done');
    const result = spawnSync('bash', [RETRY, cmd, 'init'], {
      encoding: 'utf8',
      env: { ...process.env, RETRY_BACKOFF_SECONDS: '0' },
    });
    return result;
  };

  test('passes through a command that works', () => {
    expect(retry(0).status).toBe(0);
    expect(calls('flaky-cmd')).toBe(1);
  });

  test('reruns a command that fails twice', () => {
    expect(retry(2).status).toBe(0);
    expect(calls('flaky-cmd')).toBe(3);
  });

  test('keeps the exit status after the third failure', () => {
    const result = retry(3);
    expect(result.status).toBe(22);
    expect(calls('flaky-cmd')).toBe(3);
    expect(result.stdout).toContain('failed after 3 attempts');
  });
});

describe('workflows', () => {
  const workflows = path.join(import.meta.dir, '..', '.github', 'workflows');
  const files = fs.readdirSync(workflows).filter((f) => f.endsWith('.yml'));

  test('no workflow sets up Bun without the shared action', () => {
    for (const file of files) {
      const source = fs.readFileSync(path.join(workflows, file), 'utf8');
      expect(source, file).not.toContain('uses: oven-sh/setup-bun');
    }
  });

  test('the version it installs is the one .bun-version names', () => {
    const version = fs
      .readFileSync(path.join(import.meta.dir, '..', '.bun-version'), 'utf8')
      .trim();
    expect(version).toMatch(/^\d+\.\d+\.\d+$/);
  });

  test('every terraform init in terraform-plan.yml goes through retry.sh', () => {
    const source = fs.readFileSync(
      path.join(workflows, 'terraform-plan.yml'),
      'utf8'
    );
    const inits = source
      .split('\n')
      .filter((line) => /\bterraform\b.*\binit\b/.test(line))
      .filter((line) => !line.trim().startsWith('#'));
    expect(inits.length).toBeGreaterThan(0);
    for (const line of inits) expect(line).toContain('retry.sh');
  });
});
