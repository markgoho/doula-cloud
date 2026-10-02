/**
 * `scripts/firebase-tools.sh` -- the one way a workflow runs
 * `firebase-tools` (#1624): the root `bun.lock` pin, a version line, and
 * for a deploy up to three attempts.
 *
 * Everything that matters here is a property of a real process: the exit
 * status the job sees and the lines the log holds. So each test spawns the
 * real script against a stand-in binary, through the `FIREBASE_TOOLS_BIN`
 * seam, with the backoff unit set to zero.
 */
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const SCRIPT = path.join(import.meta.dir, 'firebase-tools.sh');

let dir: string;

beforeEach(() => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'firebase-tools-test-'));
});

afterEach(() => {
  fs.rmSync(dir, { recursive: true, force: true });
});

/**
 * A stand-in `firebase` binary. It answers `--version`, and for any other
 * command it fails its first `failures` calls and passes after them. It
 * counts its calls in a file, because each attempt is a new process.
 */
function standIn(failures: number): string {
  const bin = path.join(dir, 'firebase');
  const calls = path.join(dir, 'calls');
  fs.writeFileSync(
    bin,
    [
      '#!/usr/bin/env bash',
      'if [ "$1" = "--version" ]; then echo "15.30.2"; exit 0; fi',
      `n=$(( $(cat "${calls}" 2>/dev/null || echo 0) + 1 ))`,
      `echo "$n" > "${calls}"`,
      `if [ "$n" -le ${failures} ]; then`,
      '  echo "{\\"status\\":\\"error\\",\\"call\\":$n}"',
      '  echo "Error: Failed to authenticate, have you run firebase login?" >&2',
      '  exit 1',
      'fi',
      'echo "{\\"status\\":\\"success\\",\\"call\\":$n,\\"args\\":\\"$*\\"}"',
      '',
    ].join('\n'),
    { mode: 0o755 }
  );
  return bin;
}

function run(args: string[], env: Record<string, string> = {}) {
  const result = spawnSync('bash', [SCRIPT, ...args], {
    encoding: 'utf8',
    env: { ...process.env, FIREBASE_TOOLS_BACKOFF_SECONDS: '0', ...env },
  });
  return {
    status: result.status,
    stdout: result.stdout,
    stderr: result.stderr,
  };
}

const lines = (text: string): string[] =>
  text.split('\n').filter((line) => line !== '');

describe('retry', () => {
  test('prints the version before the first attempt and passes on attempt 1', () => {
    const result = run(['retry', 'deploy', '--only', 'hosting:app'], {
      FIREBASE_TOOLS_BIN: standIn(0),
    });

    expect(result.status).toBe(0);
    expect(lines(result.stdout)).toEqual([
      'firebase-tools version: 15.30.2 (the root bun.lock pin)',
      'firebase-tools deploy: attempt 1 of 3',
      '{"status":"success","call":1,"args":"deploy --only hosting:app"}',
      'firebase-tools deploy passed on attempt 1 of 3',
    ]);
  });

  test('names each failed attempt and the attempt that passed', () => {
    const result = run(['retry', 'deploy'], { FIREBASE_TOOLS_BIN: standIn(2) });

    expect(result.status).toBe(0);
    expect(result.stdout).toContain(
      '::warning::firebase-tools deploy attempt 1 of 3 failed (exit 1); retrying in 0s'
    );
    expect(result.stdout).toContain(
      '::warning::firebase-tools deploy attempt 2 of 3 failed (exit 1); retrying in 0s'
    );
    expect(result.stdout).toContain(
      'firebase-tools deploy passed on attempt 3 of 3'
    );
    expect(result.stdout).not.toContain('::error::');
  });

  test('a command that fails on every attempt fails the step', () => {
    const result = run(['retry', 'deploy'], {
      FIREBASE_TOOLS_BIN: standIn(99),
    });

    expect(result.status).toBe(1);
    expect(fs.readFileSync(path.join(dir, 'calls'), 'utf8').trim()).toBe('3');
    expect(lines(result.stdout).at(-1)).toBe(
      '::error::firebase-tools deploy failed after 3 attempts'
    );
    expect(result.stdout).toContain(
      '::warning::firebase-tools deploy attempt 3 of 3 failed (exit 1)'
    );
    expect(result.stdout).not.toContain('passed on attempt');
    // The command's own error reaches the log on each attempt.
    expect(lines(result.stderr)).toEqual(
      Array(3).fill(
        'Error: Failed to authenticate, have you run firebase login?'
      )
    );
  });

  test('the backoff is one unit, then two', () => {
    const sleeps = path.join(dir, 'sleeps');
    const fakeBin = path.join(dir, 'bin');
    fs.mkdirSync(fakeBin);
    fs.writeFileSync(
      path.join(fakeBin, 'sleep'),
      `#!/usr/bin/env bash\necho "$1" >> "${sleeps}"\n`,
      { mode: 0o755 }
    );

    const result = run(['retry', 'deploy'], {
      FIREBASE_TOOLS_BIN: standIn(99),
      FIREBASE_TOOLS_BACKOFF_SECONDS: '10',
      PATH: `${fakeBin}:${process.env.PATH}`,
    });

    expect(result.status).toBe(1);
    expect(lines(fs.readFileSync(sleeps, 'utf8'))).toEqual(['10', '20']);
    expect(result.stdout).toContain('retrying in 10s');
    expect(result.stdout).toContain('retrying in 20s');
  });

  test('with a stdout file, the file holds the passing attempt alone', () => {
    const out = path.join(dir, 'deploy-output.json');
    const result = run(['retry', 'hosting:channel:deploy', 'pr-1', '--json'], {
      FIREBASE_TOOLS_BIN: standIn(1),
      FIREBASE_TOOLS_STDOUT_FILE: out,
    });

    expect(result.status).toBe(0);
    expect(JSON.parse(fs.readFileSync(out, 'utf8'))).toEqual({
      status: 'success',
      call: 2,
      args: 'hosting:channel:deploy pr-1 --json',
    });
    // The failed attempt's `--json` error is in the log, not lost in the file.
    expect(result.stdout).toContain("The command's output follows.");
    expect(result.stdout).toContain('{"status":"error","call":1}');
    expect(result.stdout).not.toContain('"status":"success"');
  });
});

describe('once', () => {
  test('makes one attempt and returns the command’s own status and output', () => {
    const result = run(['once', 'hosting:channel:delete', 'pr-1'], {
      FIREBASE_TOOLS_BIN: standIn(99),
    });

    expect(result.status).toBe(1);
    expect(fs.readFileSync(path.join(dir, 'calls'), 'utf8').trim()).toBe('1');
    expect(result.stdout).toBe('{"status":"error","call":1}\n');
    expect(result.stdout).not.toContain('::warning::');
  });
});

describe('version', () => {
  test('prints the version line', () => {
    const result = run(['version'], { FIREBASE_TOOLS_BIN: standIn(0) });

    expect(result.status).toBe(0);
    expect(result.stdout).toBe(
      'firebase-tools version: 15.30.2 (the root bun.lock pin)\n'
    );
  });
});

describe('a missing install', () => {
  test.each(['version', 'once', 'retry'])(
    '%s fails and does not download',
    (mode) => {
      const result = run([mode, 'deploy'], {
        FIREBASE_TOOLS_BIN: path.join(dir, 'node_modules', '.bin', 'firebase'),
      });

      expect(result.status).toBe(1);
      expect(result.stdout).toContain(
        '::error::firebase-tools is not installed at'
      );
      expect(result.stdout).toContain('bun install --frozen-lockfile');
    }
  );
});

describe('usage', () => {
  test.each([[[]], [['deploy']], [['retry']], [['once']]])(
    '%j exits 2',
    (args) => {
      const result = run(args, { FIREBASE_TOOLS_BIN: standIn(0) });

      expect(result.status).toBe(2);
      expect(result.stderr).toContain('usage: scripts/firebase-tools.sh');
    }
  );
});
