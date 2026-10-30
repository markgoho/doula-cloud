/**
 * `scripts/stripe-listen.sh` -- forwards Stripe Sandbox events to the
 * local BFF (#2201).
 *
 * Two things are checked against the real script run with a stand-in
 * `stripe` binary, through the `STRIPE_BIN` seam:
 *
 * - The BFF port is the worktree's own, `18080 + 100 × offset`, with the
 *   offset read from the nearest `.port-offset` above the script, the
 *   same walk `app/e2e/ports.ts` makes. No file means offset 0.
 * - Each listener names what it takes, which the CLI (1.55) requires of
 *   a `--forward-to` (`--all-snapshot`, or `--events`).
 * - Thin events are forwarded the way the CLI (1.55) asks: a listener of
 *   their own, `--events` naming the thin type and `--forward-to`,
 *   because "snapshot and thin events cannot share one destination". The
 *   deprecated `--thin-events` and `--forward-thin-to` are gone.
 */
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const SCRIPT = path.join(import.meta.dir, 'stripe-listen.sh');

const THIN_EVENT =
  'v2.core.account[configuration.merchant].capability_status_updated';

let dir: string;

beforeEach(() => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stripe-listen-test-'));
  // The script finds its offset by walking up from its own directory, so
  // each test runs a copy that sits in a worktree-shaped tree of its own.
  fs.mkdirSync(path.join(dir, 'scripts'));
  fs.copyFileSync(SCRIPT, path.join(dir, 'scripts', 'stripe-listen.sh'));
});

afterEach(() => {
  fs.rmSync(dir, { recursive: true, force: true });
});

/**
 * A stand-in `stripe` that writes the arguments of each call, one call
 * per line. Each listener is its own process, so the calls are appended
 * to one file. The snapshot listener, which the script runs in the
 * foreground, waits a moment before it exits: the script stops the thin
 * listener when the foreground one ends, and the wait lets the thin one
 * write its line first.
 */
function standIn(): { bin: string; calls: string } {
  const bin = path.join(dir, 'stripe');
  const calls = path.join(dir, 'calls');
  fs.writeFileSync(
    bin,
    [
      '#!/usr/bin/env bash',
      `echo "$*" >> "${calls}"`,
      'if [[ "$*" == *--forward-connect-to* ]]; then sleep 0.5; fi',
    ].join('\n')
  );
  fs.chmodSync(bin, 0o755);
  return { bin, calls };
}

function run(): string[] {
  const { bin, calls } = standIn();
  const result = spawnSync(
    'bash',
    [path.join(dir, 'scripts', 'stripe-listen.sh')],
    {
      env: { ...process.env, STRIPE_BIN: bin },
      encoding: 'utf8',
    }
  );
  expect(result.status).toBe(0);
  return fs.readFileSync(calls, 'utf8').trim().split('\n').sort();
}

describe('stripe-listen.sh', () => {
  test('forwards to the BFF at offset 0 when no .port-offset is found', () => {
    expect(run()).toEqual(
      [
        `listen --events ${THIN_EVENT} --forward-to http://127.0.0.1:18080/api/stripe/account-webhook`,
        'listen --all-snapshot --forward-to http://127.0.0.1:18080/api/stripe/webhook --forward-connect-to http://127.0.0.1:18080/api/stripe/connect-webhook',
      ].sort()
    );
  });

  test("forwards to the worktree's own BFF port", () => {
    fs.writeFileSync(path.join(dir, '.port-offset'), '3\n');

    const calls = run();

    expect(
      calls.every((call) => call.includes('http://127.0.0.1:18380/'))
    ).toBe(true);
    expect(calls).toHaveLength(2);
  });

  test('reads an unreadable offset as 0, as ports.ts does', () => {
    fs.writeFileSync(path.join(dir, '.port-offset'), 'not a number\n');

    expect(
      run().every((call) => call.includes('http://127.0.0.1:18080/'))
    ).toBe(true);
  });

  test('gives thin events a listener of their own, with no deprecated flag', () => {
    const calls = run();
    const thin = calls.filter((call) => call.includes(THIN_EVENT));

    expect(thin).toEqual([
      `listen --events ${THIN_EVENT} --forward-to http://127.0.0.1:18080/api/stripe/account-webhook`,
    ]);
    expect(calls.join('\n')).not.toMatch(
      /--thin-events|--forward-thin-to|--forward-thin-connect-to/
    );
  });
});
