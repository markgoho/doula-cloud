/**
 * `scripts/install-playwright-deps.sh` -- the retry loop for Playwright's OS
 * dependencies (#1655).
 *
 * The defect: after `timeout` stopped `bunx`, the root `apt-get` it had
 * started kept the dpkg lock, so every retry died in about a second on the
 * lock. These specs model that with a fake install command that fails the
 * way a locked dpkg does for as long as a "lock" file exists, and a fake
 * apt-busy check that watches the same file. The last describe block runs the
 * real `apt_busy` probe instead (#1661), on Linux only.
 */
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import {
  chmodSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  symlinkSync,
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
      DEB_CACHE_DIR: join(dir, 'deb-cache'),
      APT_ARCHIVES: join(dir, 'archives'),
      // No spec may run the real `apt-get install --fix-broken`.
      REPAIR_CMD: 'true',
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

  test('stops before a retry when the wait ends with apt still held, and names the holder', () => {
    // SETTLE_TIMEOUT=0 ends the wait at once while the lock is held. A retry
    // then would only meet the lock (#1667), so the step stops instead.
    const lock = join(dir, 'lock');
    const { code, out } = run(lockedInstall(), {
      SETTLE_TIMEOUT: '0',
      ATTEMPTS: '2',
      APT_BUSY_CMD: `test -e ${lock} && echo "5138 apt-get update"`,
    });
    expect(code).toBe(1);
    expect(out).toContain('still busy after 0s. It is held by:');
    expect(out).toContain('5138 apt-get update');
    expect(out).not.toContain('Could not get lock');
    expect(
      readFileSync(join(dir, 'count'), 'utf8').trim().split('\n')
    ).toHaveLength(1);
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
    expect(conf).toContain('APT::Keep-Downloaded-Packages "true"');
  });
});

// The package cache (#1667): a cache hit installs with no mirror, and a
// mirror install leaves its packages for the next run.
describe('install-playwright-deps.sh package cache', () => {
  function cacheHolds(...names: string[]) {
    mkdirSync(join(dir, 'deb-cache'), { recursive: true });
    for (const name of names) writeFileSync(join(dir, 'deb-cache', name), '');
  }

  test('installs from cached packages and never runs the mirror install', () => {
    cacheHolds('libnss3.deb', 'libgbm1.deb');
    const mirror = join(dir, 'mirror-ran');
    const cached = join(dir, 'cached-ran');
    const { code, out } = run(`touch ${mirror}`, {
      CACHED_INSTALL_CMD: `touch ${cached}`,
    });
    expect(code).toBe(0);
    expect(out).toContain('Installing 2 cached packages');
    expect(existsSync(cached)).toBe(true);
    expect(existsSync(mirror)).toBe(false);
  });

  test('repairs dpkg, then goes on to the mirror, when the cached install fails', () => {
    // A failed `dpkg --install` leaves packages half-configured, and the
    // mirror install refuses that state, so the repair comes first.
    cacheHolds('libnss3.deb');
    const order = join(dir, 'order');
    const { code, out } = run(`echo mirror >> ${order}`, {
      CACHED_INSTALL_CMD: 'exit 1',
      REPAIR_CMD: `echo repair >> ${order}`,
    });
    expect(code).toBe(0);
    expect(out).toContain(
      'The cached install failed; repairing dpkg, then installing from the mirror.'
    );
    expect(readFileSync(order, 'utf8').trim().split('\n')).toEqual([
      'repair',
      'mirror',
    ]);
  });

  test('still tries the mirror when the repair fails', () => {
    cacheHolds('libnss3.deb');
    const mirror = join(dir, 'mirror-ran');
    const { code, out } = run(`touch ${mirror}`, {
      CACHED_INSTALL_CMD: 'exit 1',
      REPAIR_CMD: 'exit 100',
    });
    expect(code).toBe(0);
    expect(out).toContain('The dpkg repair failed');
    expect(existsSync(mirror)).toBe(true);
  });

  test('keeps the packages a mirror install downloaded', () => {
    mkdirSync(join(dir, 'archives'));
    writeFileSync(join(dir, 'archives', 'libnss3.deb'), 'nss');
    writeFileSync(join(dir, 'archives', 'libgbm1.deb'), 'gbm');
    const { code, out } = run('true');
    expect(code).toBe(0);
    expect(out).toContain('Kept 2 packages');
    expect(readdirSync(join(dir, 'deb-cache')).sort()).toEqual([
      'libgbm1.deb',
      'libnss3.deb',
    ]);
  });

  test('makes no cache directory when apt left no packages', () => {
    // The Actions cache saves an empty directory, which would hold the key
    // with nothing in it until the next runner image.
    mkdirSync(join(dir, 'archives'));
    const { code, out } = run('true');
    expect(code).toBe(0);
    expect(out).toContain('nothing to cache');
    expect(existsSync(join(dir, 'deb-cache'))).toBe(false);
  });
});

// The real `apt_busy` path (#1661): the specs above always replace it with
// APT_BUSY_CMD. These two run it for real, against a stand-in "apt-get" and a
// held lock file, with the same first-attempt-fails-and-leaves-an-orphan
// shape as `lockedInstall`. They need Linux's `pgrep -x` and `fuser`, the
// tools the CI runner has; macOS has other versions of both.
describe.skipIf(process.platform !== 'linux')(
  'install-playwright-deps.sh real apt_busy path',
  () => {
    // The first run leaves $orphan running for two seconds and fails. Later
    // runs fail for as long as $alive succeeds, and succeed once it does not.
    function orphanInstall(orphan: string, alive: string): string {
      const counter = join(dir, 'count');
      const fake = join(dir, 'install');
      writeFileSync(
        fake,
        `#!/usr/bin/env bash
echo x >> ${counter}
if [ "$(wc -l < ${counter})" -eq 1 ]; then
  ( ${orphan} ) >/dev/null 2>&1 &
  exit 124
fi
${alive} && { echo "E: Could not get lock"; exit 100; }
exit 0
`
      );
      chmodSync(fake, 0o755);
      return fake;
    }

    const realProbe = { APT_BUSY_CMD: '', ATTEMPTS: '2' };

    test('pgrep sees a process named apt-get', () => {
      // comm is the name the process was started under, so a symlink named
      // apt-get to sleep is "apt-get" to pgrep -x.
      const aptGet = join(dir, 'apt-get');
      symlinkSync('/bin/sleep', aptGet);
      const { code, out } = run(
        orphanInstall(`${aptGet} 2`, 'pgrep -x apt-get >/dev/null'),
        { ...realProbe, APT_LOCK_FILES: join(dir, 'no-such-lock') }
      );
      expect(out).toContain('still running; waiting for it');
      expect(code).toBe(0);
    });

    test('fuser sees a process holding the lock file', () => {
      const lock = join(dir, 'lock-frontend');
      writeFileSync(lock, '');
      const { code, out } = run(
        orphanInstall(`exec 3<${lock}; sleep 2`, `fuser -s ${lock}`),
        { ...realProbe, APT_LOCK_FILES: lock }
      );
      expect(out).toContain('still running; waiting for it');
      expect(code).toBe(0);
    });

    // Run 37031423643 (#1667): the leftover `apt-get update` held
    // /var/lib/apt/lists/lock past the wait, and the retry died on it in a
    // second, because DPkg::Lock::Timeout covers only the dpkg lock. Now the
    // step stops before that retry and names the holder.
    test('a process holding the apt lists lock past the wait stops the step before a retry', () => {
      const lock = join(dir, 'lists-lock');
      writeFileSync(lock, '');
      const { code, out } = run(
        orphanInstall(`exec 3<${lock}; sleep 5`, `fuser -s ${lock}`),
        { ...realProbe, APT_LOCK_FILES: lock, SETTLE_TIMEOUT: '1' }
      );
      expect(code).toBe(1);
      expect(out).toContain('It is held by:');
      expect(out).toContain(lock);
      expect(out).not.toContain('Could not get lock');
    });
  }
);
