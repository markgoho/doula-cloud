/**
 * `scripts/install-playwright-deps.sh` -- the retry loop for Playwright's OS
 * dependencies (#1655).
 *
 * The defect: after `timeout` stopped `bunx`, the root `apt-get` it had
 * started kept the dpkg lock, so every retry died in about a second on the
 * lock. These specs model that with a fake install command that fails the
 * way a locked dpkg does for as long as a "lock" file exists, and a fake
 * apt-busy check that watches the same file. The real lock is exercised by a
 * temporary CI run recorded on #1655.
 */
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import {
  chmodSync,
  existsSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const script = join(import.meta.dir, 'install-playwright-deps.sh');

let dir: string;
beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), 'playwright-deps-'));
  // A stand-in for coreutils `timeout`, which macOS lacks: runs the command.
  const shim = join(dir, 'timeout-shim');
  writeFileSync(shim, '#!/usr/bin/env bash\nshift\nexec "$@"\n');
  chmodSync(shim, 0o755);
});
afterEach(() => rmSync(dir, { recursive: true, force: true }));

function run(installCmd: string, extra: Record<string, string> = {}) {
  const result = Bun.spawnSync(['bash', script], {
    env: {
      ...process.env,
      INSTALL_CMD: installCmd,
      AS_ROOT: '',
      APT_CONF: join(dir, 'apt.conf'),
      TIMEOUT_CMD: join(dir, 'timeout-shim'),
      RETRY_SLEEP: '0',
      POLL_INTERVAL: '1',
      APT_BUSY_CMD: `test -e ${join(dir, 'lock')}`,
      ...extra,
    },
  });
  return { code: result.exitCode, out: result.stdout.toString() };
}

// The first run starts a background "apt-get" that holds the lock for two
// seconds, then fails (as a timed-out attempt does). Later runs fail while
// the lock file exists and succeed once it is gone. The counter file records
// how many times the install command ran.
function lockedInstall(): string {
  const lock = join(dir, 'lock');
  const counter = join(dir, 'count');
  const fake = join(dir, 'install');
  writeFileSync(
    fake,
    `#!/usr/bin/env bash
echo x >> ${counter}
if [ "$(wc -l < ${counter})" -eq 1 ]; then
  touch ${lock}
  ( sleep 2; rm -f ${lock} ) >/dev/null 2>&1 &
  exit 124
fi
[ -e ${lock} ] && { echo "E: Could not get lock"; exit 100; }
exit 0
`
  );
  chmodSync(fake, 0o755);
  return fake;
}

describe('install-playwright-deps.sh', () => {
  test('a retry after a timed-out attempt waits for the orphan apt-get and succeeds', () => {
    const { code, out } = run(lockedInstall());
    expect(code).toBe(0);
    expect(out).toContain('still running; waiting for it');
    expect(
      readFileSync(join(dir, 'count'), 'utf8').trim().split('\n')
    ).toHaveLength(2);
    expect(existsSync(join(dir, 'lock'))).toBe(false);
  });

  test('without the wait the same retry sequence fails on the lock', () => {
    // SETTLE_TIMEOUT=0 turns the wait off: the retry meets the held lock.
    const { code, out } = run(lockedInstall(), {
      SETTLE_TIMEOUT: '0',
      ATTEMPTS: '2',
    });
    expect(code).toBe(1);
    expect(out).toContain('Could not get lock');
  });

  test('exits 0 on the first success and does not retry', () => {
    const counter = join(dir, 'count');
    const { code } = run(`echo x >> ${counter}`);
    expect(code).toBe(0);
    expect(readFileSync(counter, 'utf8').trim().split('\n')).toHaveLength(1);
  });

  test('stops retrying when the total budget is spent', () => {
    const { code, out } = run('sleep 2; exit 1', {
      TOTAL_BUDGET: '1',
      ATTEMPTS: '5',
    });
    expect(code).toBe(1);
    expect(out).toContain('budget for this step is spent');
  });

  test('exits 1 after the last failed attempt', () => {
    const { code, out } = run('exit 1', { ATTEMPTS: '2' });
    expect(code).toBe(1);
    expect(out).toContain('attempt 2 failed');
  });

  test('writes the apt bounds', () => {
    run('true');
    const conf = readFileSync(join(dir, 'apt.conf'), 'utf8');
    expect(conf).toContain('Acquire::http::Timeout "30"');
    expect(conf).toContain('Acquire::https::Timeout "30"');
    expect(conf).toContain('Acquire::Retries "3"');
    expect(conf).toContain('DPkg::Lock::Timeout "120"');
  });
});
