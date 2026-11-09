/**
 * `scripts/ci-transient.ts` -- the verdict that `ci-heal.ts` reruns on
 * and `trunk-red.ts` stays quiet on (#2819). The logs below are excerpts
 * of real runs; see ci-transient.fixtures.ts.
 */
import { describe, expect, test } from 'bun:test';
import { classify, signatureBeforeError } from './ci-transient.ts';
import {
  BUN_504,
  COVCHECK_FAILED,
  TERRAFORM_504,
} from './ci-transient.fixtures.ts';

const failedAt = (name: string, step: string, log: string, id = 1) => ({
  id,
  name,
  conclusion: 'failure',
  steps: [
    { name: 'Checkout', conclusion: 'success' },
    { name: step, conclusion: 'failure' },
  ],
  log,
});

describe('classify', () => {
  test('setup-bun 504s in the format job are transient', () => {
    const verdict = classify([
      failedAt('format', 'Run oven-sh/setup-bun@v2', BUN_504),
    ]);
    expect(verdict.transient).toBe(true);
    expect(verdict.reason).toContain('Unexpected HTTP response: 504');
  });

  test('a registry 504 in the Validate step of the Sandbox plan is transient', () => {
    const verdict = classify([
      failedAt(
        'plan-stripe-sandbox',
        'Validate the Stripe roots',
        TERRAFORM_504
      ),
    ]);
    expect(verdict.transient).toBe(true);
    expect(verdict.reason).toContain('504 Gateway Timeout');
  });

  test('the covcheck failure of a Go job is not transient', () => {
    const verdict = classify([
      failedAt('api', 'Enforce 100% line coverage', COVCHECK_FAILED),
    ]);
    expect(verdict.transient).toBe(false);
    expect(verdict.reason).toContain('not a setup or download step');
  });

  test('a test step is never transient, even when its log names ECONNRESET', () => {
    const log = `read tcp 10.0.0.1:5432: ECONNRESET\n${COVCHECK_FAILED}`;
    for (const step of ['Test', 'Lint', 'Go tests', 'Check formatting']) {
      expect(classify([failedAt('api', step, log)]).transient).toBe(false);
    }
  });

  test('a setup step with no signature in its log is not transient', () => {
    const verdict = classify([
      failedAt('app', 'Install dependencies', COVCHECK_FAILED),
    ]);
    expect(verdict.transient).toBe(false);
    expect(verdict.reason).toContain('no transient signature');
  });

  test('a signature in the output of a step that passed is not evidence', () => {
    const log = `Unexpected HTTP response: 504\n${'filler\n'.repeat(40)}${COVCHECK_FAILED}`;
    expect(
      classify([failedAt('app', 'Install dependencies', log)]).transient
    ).toBe(false);
  });

  test('one real failure among transient ones makes the run not transient', () => {
    const verdict = classify([
      failedAt('format', 'Run oven-sh/setup-bun@v2', BUN_504, 1),
      failedAt('api', 'Enforce 100% line coverage', COVCHECK_FAILED, 2),
    ]);
    expect(verdict.transient).toBe(false);
  });

  test('a failed Set up job step is transient with no signature', () => {
    expect(
      classify([failedAt('api', 'Set up job', '##[error]The runner lost')])
        .transient
    ).toBe(true);
  });

  test('a run with no failed job is not transient', () => {
    expect(classify([]).transient).toBe(false);
  });
});

describe('signatureBeforeError', () => {
  test.each([
    'Error response from daemon: received unexpected HTTP status: 503 Service Unavailable',
    'npm error code ECONNRESET',
    'curl: (22) The requested URL returned error: 502',
    'http.client.RemoteDisconnected: Remote end closed connection',
    'dial tcp 1.2.3.4:443: i/o timeout',
  ])('finds %s', (line) => {
    expect(signatureBeforeError(`${line}\n##[error]exit 1`)).toBe(
      `${line}`.trim()
    );
  });

  test('strips terminal color codes before matching', () => {
    expect(signatureBeforeError('\x1b[31mETIMEDOUT\x1b[0m\n##[error]x')).toBe(
      'ETIMEDOUT'
    );
  });
});
