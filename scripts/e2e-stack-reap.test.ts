/**
 * `.claude/hooks/e2e-stack-reap.ts` -- the SessionStart hook that tears
 * down the podman-compose e2e stack, and the host-process pidfiles beside
 * it, that a killed worktree session leaves running (#1066, #1194). See
 * that file's header and docs/testing.md's "Reaping orphaned e2e stacks"
 * section for the full story.
 *
 * The reap decisions (groupStackProjects/pickReapProjects for the
 * compose stack, discoverPidfiles/pickReapPidfiles/looksLikeOurProcess
 * for host-process pidfiles) and the filesystem readers behind them
 * (recentlyTouched/liveWorktreeOffsets, plus isHeartbeatFresh/
 * heartbeatLiveOffsets/liveOffsets, #1193) are imported and tested
 * directly. main()'s fail-open behavior is exercised as a subprocess, the
 * same way scripts/testdb-reap.test.ts covers testdb-reap.ts.
 *
 * The container fixtures below are the labels podman-compose 1.6.0
 * actually writes, read back off this machine after starting
 * app/compose.e2e.yaml under `-p doula-cloud-e2e-9`.
 */
import { afterEach, describe, expect, test } from 'bun:test';
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { e2eApiBinaryPath, e2eHeartbeatPath } from '../app/e2e/ports.ts';
import {
  HEARTBEAT_STALE_MS,
  QUIET_MS,
  REAP_THRESHOLD_MS,
  discoverPidfiles,
  groupStackProjects,
  heartbeatLiveOffsets,
  isHeartbeatFresh,
  isProcessAlive,
  liveOffsets,
  liveWorktreeOffsets,
  looksLikeOurProcess,
  pickReapPidfiles,
  pickReapProjects,
  recentlyTouched,
  type StackProject,
} from '../.claude/hooks/e2e-stack-reap.ts';
import type { ReapCandidate } from '../.claude/hooks/container-engine.ts';

const REPO_ROOT = path.resolve(import.meta.dir, '..');
const HOOK = path.join(REPO_ROOT, '.claude', 'hooks', 'e2e-stack-reap.ts');

function invoke(
  env: Record<string, string | undefined>
): Promise<{ exitCode: number; stdout: string }> {
  return new Promise((resolve, reject) => {
    const child = spawn('bun', [HOOK], {
      stdio: ['ignore', 'pipe', 'inherit'],
      env,
    });
    let stdout = '';
    child.stdout.on('data', (chunk) => {
      stdout += chunk;
    });
    child.on('error', reject);
    child.on('close', (code) => resolve({ exitCode: code ?? 1, stdout }));
  });
}

function composeContainer(
  project: string,
  service: string,
  createdAtMs: number
): ReapCandidate {
  return {
    id: `${project}_${service}`,
    name: `${project}_${service}_1`,
    labels: {
      'com.docker.compose.project': project,
      'com.docker.compose.service': service,
      'io.podman.compose.project': project,
      'io.podman.compose.version': '1.6.0',
    },
    createdAtMs,
  };
}

function stack(
  project: string,
  offset: number,
  newestCreatedAtMs: number
): StackProject {
  return {
    project,
    offset,
    newestCreatedAtMs,
    containerIds: [`${project}_db`],
  };
}

const temporaryRoots: string[] = [];

function temporaryWorktreesRoot(): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'e2e-stack-reap-test-'));
  temporaryRoots.push(root);
  return root;
}

// discoverPidfiles/pickReapPidfiles/main()'s own pidfile sweep never run
// against the real os.tmpdir() from a test -- other agents' real
// worktrees hold real pidfiles there right now. This is the fixture
// directory their own tests point discoverPidfiles at instead.
function temporaryPidfileDir(): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'e2e-pidfile-reap-test-'));
  temporaryRoots.push(root);
  return root;
}

// Writes a fixture pidfile named exactly the way app/e2e/ports.ts's
// e2ePidfilePath would, backdated by ageMs so pickReapPidfiles' age check
// has something to compare against.
function writePidfile(
  dir: string,
  kind: string,
  offset: number,
  pid: number,
  ageMs: number,
  now: number
): string {
  const filePath = path.join(dir, `doula-cloud-e2e-${kind}-${offset}.pid`);
  fs.writeFileSync(filePath, String(pid));
  const stamp = new Date(now - ageMs);
  fs.utimesSync(filePath, stamp, stamp);
  return filePath;
}

// Shared by liveWorktreeOffsets's and heartbeatLiveOffsets's own describe
// blocks below -- both need a worktree directory carrying a `.port-offset`
// to scan with readWorktreeOffsets.
function worktreeWithOffset(
  root: string,
  name: string,
  offset: string,
  ageMs: number,
  now: number
): string {
  const worktree = path.join(root, name);
  fs.mkdirSync(worktree);
  fs.writeFileSync(path.join(worktree, '.port-offset'), `${offset}\n`);
  const stamp = new Date(now - ageMs);
  fs.utimesSync(worktree, stamp, stamp);
  return worktree;
}

// Writing the heartbeat file bumps its parent directory's own mtime, on
// every POSIX filesystem, as any new directory entry does -- which would
// make recentlyTouched see the worktree as freshly touched no matter
// what age worktreeWithOffset gave it. Call this after writing or
// re-stamping a heartbeat file whenever a test needs the worktree
// directory itself to still read as quiet.
function restampWorktreeQuiet(worktree: string, now: number): void {
  const stamp = new Date(now - QUIET_MS - 60_000);
  fs.utimesSync(worktree, stamp, stamp);
}

afterEach(() => {
  while (temporaryRoots.length > 0)
    fs.rmSync(temporaryRoots.pop()!, { recursive: true, force: true });
});

describe('groupStackProjects', () => {
  test("collapses a project's containers into one entry, dated by the newest", () => {
    const containers = [
      composeContainer('doula-cloud-e2e-3', 'db', 1000),
      composeContainer('doula-cloud-e2e-3', 'gcs', 5000),
    ];
    expect(groupStackProjects(containers)).toEqual([
      {
        project: 'doula-cloud-e2e-3',
        offset: 3,
        newestCreatedAtMs: 5000,
        containerIds: ['doula-cloud-e2e-3_db', 'doula-cloud-e2e-3_gcs'],
      },
    ]);
  });

  test("keeps each worktree's stack separate", () => {
    const containers = [
      composeContainer('doula-cloud-e2e-1', 'db', 1000),
      composeContainer('doula-cloud-e2e-2', 'db', 2000),
    ];
    expect(
      groupStackProjects(containers).map((project) => project.offset)
    ).toEqual([1, 2]);
  });

  test("ignores the main checkout's unsuffixed stack", () => {
    expect(
      groupStackProjects([composeContainer('doula-cloud-e2e', 'db', 1000)])
    ).toEqual([]);
  });

  test("ignores another project's compose containers", () => {
    expect(
      groupStackProjects([composeContainer('some-other-project-4', 'db', 1000)])
    ).toEqual([]);
  });

  test('ignores a name that only starts like ours', () => {
    expect(
      groupStackProjects([
        composeContainer('doula-cloud-e2e-attachments', 'db', 1000),
      ])
    ).toEqual([]);
  });

  test('ignores a container with no compose project label at all', () => {
    const testcontainer: ReapCandidate = {
      id: 'abc123',
      name: 'stupefied_keller',
      labels: { 'org.testcontainers': 'true' },
      createdAtMs: 1000,
    };
    expect(groupStackProjects([testcontainer])).toEqual([]);
  });
});

describe('pickReapProjects', () => {
  const now = Date.now();
  const none = new Set<number>();

  test('reaps a stack older than the threshold whose offset no live worktree claims', () => {
    const orphan = stack(
      'doula-cloud-e2e-4',
      4,
      now - REAP_THRESHOLD_MS - 1000
    );
    expect(pickReapProjects([orphan], none, now)).toEqual([orphan]);
  });

  test('never reaps a stack whose offset a live worktree claims, however old', () => {
    const held = stack('doula-cloud-e2e-4', 4, now - REAP_THRESHOLD_MS * 100);
    expect(pickReapProjects([held], new Set([4]), now)).toEqual([]);
  });

  test('does not reap a stack younger than the threshold', () => {
    const fresh = stack('doula-cloud-e2e-4', 4, now - REAP_THRESHOLD_MS + 1000);
    expect(pickReapProjects([fresh], none, now)).toEqual([]);
  });

  test('does not reap a stack exactly at the threshold', () => {
    const borderline = stack('doula-cloud-e2e-4', 4, now - REAP_THRESHOLD_MS);
    expect(pickReapProjects([borderline], none, now)).toEqual([]);
  });

  test('accepts a custom threshold', () => {
    const fresh = stack('doula-cloud-e2e-4', 4, now - 5000);
    expect(pickReapProjects([fresh], none, now, 1000)).toEqual([fresh]);
  });

  test('only reaps the stacks that qualify out of a mixed list', () => {
    const orphan = stack(
      'doula-cloud-e2e-1',
      1,
      now - REAP_THRESHOLD_MS - 1000
    );
    const held = stack('doula-cloud-e2e-2', 2, now - REAP_THRESHOLD_MS - 1000);
    const fresh = stack('doula-cloud-e2e-3', 3, now - 1000);
    expect(pickReapProjects([orphan, held, fresh], new Set([2]), now)).toEqual([
      orphan,
    ]);
  });
});

describe('recentlyTouched', () => {
  const now = Date.now();

  test('a worktree directory just written is touched', () => {
    const worktree = temporaryWorktreesRoot();
    expect(recentlyTouched(worktree, now)).toBe(true);
  });

  test('a worktree gone quiet is not touched', () => {
    const worktree = temporaryWorktreesRoot();
    const old = new Date(now - QUIET_MS - 60_000);
    fs.utimesSync(worktree, old, old);
    expect(recentlyTouched(worktree, now)).toBe(false);
  });

  /*
   * The case the directory's own mtime cannot answer: editing a file in
   * a subdirectory never changes the worktree root's mtime, but a
   * session's own hooks rewrite the git index on nearly every command.
   * worktree-prune.ts follows the `.git` pointer file for exactly this
   * reason, and so does this.
   */
  test('a quiet directory whose git index is fresh is still touched', () => {
    const worktree = temporaryWorktreesRoot();
    const gitDir = path.join(worktree, 'gitdir');
    fs.mkdirSync(gitDir);
    fs.writeFileSync(path.join(gitDir, 'index'), '');
    fs.writeFileSync(path.join(worktree, '.git'), `gitdir: ${gitDir}\n`);

    const old = new Date(now - QUIET_MS - 60_000);
    fs.utimesSync(gitDir, old, old);
    fs.utimesSync(worktree, old, old);
    expect(recentlyTouched(worktree, now)).toBe(true);
  });

  /*
   * The only "no" this reader will give without reading a timestamp.
   * Everything else fails closed, the way worktree-prune.ts's own copy
   * does: a worktree that is there but cannot be measured counts as
   * touched. A directory that is gone is not a guess — it is the
   * session's own directory having been removed.
   */
  test('a directory that is not there is not touched', () => {
    expect(
      recentlyTouched(path.join(temporaryWorktreesRoot(), 'gone'), now)
    ).toBe(false);
  });
});

describe('liveWorktreeOffsets', () => {
  const now = Date.now();

  test('claims the offset of a worktree that has been touched', () => {
    const root = temporaryWorktreesRoot();
    worktreeWithOffset(root, 'agent-live', '5', 0, now);
    expect(liveWorktreeOffsets(root, now)).toEqual(new Set([5]));
  });

  test('claims nothing for a worktree that has gone quiet', () => {
    const root = temporaryWorktreesRoot();
    worktreeWithOffset(root, 'agent-quiet', '5', QUIET_MS + 60_000, now);
    expect(liveWorktreeOffsets(root, now)).toEqual(new Set());
  });

  test('claims nothing for a worktree with no offset assigned yet', () => {
    const root = temporaryWorktreesRoot();
    fs.mkdirSync(path.join(root, 'agent-unprovisioned'));
    expect(liveWorktreeOffsets(root, now)).toEqual(new Set());
  });

  test('claims nothing for an unreadable offset file', () => {
    const root = temporaryWorktreesRoot();
    const worktree = worktreeWithOffset(
      root,
      'agent-garbled',
      'not-a-number',
      0,
      now
    );
    expect(fs.existsSync(path.join(worktree, '.port-offset'))).toBe(true);
    expect(liveWorktreeOffsets(root, now)).toEqual(new Set());
  });

  test('returns nothing when the worktrees directory does not exist', () => {
    expect(
      liveWorktreeOffsets(path.join(temporaryWorktreesRoot(), 'absent'), now)
    ).toEqual(new Set());
  });

  test("collects every live worktree's offset at once", () => {
    const root = temporaryWorktreesRoot();
    worktreeWithOffset(root, 'agent-one', '1', 0, now);
    worktreeWithOffset(root, 'agent-two', '2', 0, now);
    worktreeWithOffset(root, 'agent-three', '3', QUIET_MS + 60_000, now);
    expect(liveWorktreeOffsets(root, now)).toEqual(new Set([1, 2]));
  });
});

describe('isHeartbeatFresh', () => {
  const now = Date.now();

  test('fresh when the heartbeat file was just touched', () => {
    const root = temporaryWorktreesRoot();
    const heartbeat = e2eHeartbeatPath(root);
    fs.writeFileSync(heartbeat, String(now));
    expect(isHeartbeatFresh(heartbeat, now)).toBe(true);
  });

  test('not fresh once the heartbeat has gone stale', () => {
    const root = temporaryWorktreesRoot();
    const heartbeat = e2eHeartbeatPath(root);
    fs.writeFileSync(heartbeat, String(now));
    const stale = new Date(now - HEARTBEAT_STALE_MS - 1000);
    fs.utimesSync(heartbeat, stale, stale);
    expect(isHeartbeatFresh(heartbeat, now)).toBe(false);
  });

  test('not fresh exactly at the staleness threshold', () => {
    const root = temporaryWorktreesRoot();
    const heartbeat = e2eHeartbeatPath(root);
    fs.writeFileSync(heartbeat, String(now));
    const stale = new Date(now - HEARTBEAT_STALE_MS);
    fs.utimesSync(heartbeat, stale, stale);
    expect(isHeartbeatFresh(heartbeat, now)).toBe(false);
  });

  // The deliberate asymmetry with recentlyTouched: a missing file means
  // this offset isn't using the signal, not "can't tell, so protect it".
  test('not fresh -- not an error -- when no heartbeat file exists', () => {
    const root = temporaryWorktreesRoot();
    expect(isHeartbeatFresh(e2eHeartbeatPath(root), now)).toBe(false);
  });

  test('accepts a custom staleness threshold', () => {
    const root = temporaryWorktreesRoot();
    const heartbeat = e2eHeartbeatPath(root);
    fs.writeFileSync(heartbeat, String(now));
    const stamp = new Date(now - 5000);
    fs.utimesSync(heartbeat, stamp, stamp);
    expect(isHeartbeatFresh(heartbeat, now, 1000)).toBe(false);
  });
});

describe('heartbeatLiveOffsets', () => {
  const now = Date.now();

  test("claims a worktree's offset when its heartbeat is fresh, however quiet the worktree itself is", () => {
    const root = temporaryWorktreesRoot();
    const worktree = worktreeWithOffset(
      root,
      'agent-quiet-but-live',
      '7',
      QUIET_MS + 60_000,
      now
    );
    fs.writeFileSync(e2eHeartbeatPath(worktree), String(now));
    expect(heartbeatLiveOffsets(root, now)).toEqual(new Set([7]));
  });

  test('claims nothing for a worktree with no heartbeat file', () => {
    const root = temporaryWorktreesRoot();
    worktreeWithOffset(root, 'agent-no-heartbeat', '7', 0, now);
    expect(heartbeatLiveOffsets(root, now)).toEqual(new Set());
  });

  test('claims nothing once the heartbeat has gone stale', () => {
    const root = temporaryWorktreesRoot();
    const worktree = worktreeWithOffset(root, 'agent-stale', '7', 0, now);
    const heartbeat = e2eHeartbeatPath(worktree);
    fs.writeFileSync(heartbeat, String(now));
    const stale = new Date(now - HEARTBEAT_STALE_MS - 1000);
    fs.utimesSync(heartbeat, stale, stale);
    expect(heartbeatLiveOffsets(root, now)).toEqual(new Set());
  });
});

describe('liveOffsets', () => {
  const now = Date.now();

  test('live: worktree touched, no heartbeat at all', () => {
    const root = temporaryWorktreesRoot();
    worktreeWithOffset(root, 'agent-touched', '1', 0, now);
    expect(liveOffsets(root, now)).toEqual(new Set([1]));
  });

  test('live: worktree quiet, but heartbeat fresh -- the case #1193 is about', () => {
    const root = temporaryWorktreesRoot();
    const worktree = worktreeWithOffset(
      root,
      'agent-quiet-but-live',
      '2',
      QUIET_MS + 60_000,
      now
    );
    fs.writeFileSync(e2eHeartbeatPath(worktree), String(now));
    restampWorktreeQuiet(worktree, now);
    expect(recentlyTouched(worktree, now)).toBe(false); // the worktree itself really is quiet
    expect(liveOffsets(root, now)).toEqual(new Set([2]));
  });

  test('not live: worktree quiet and heartbeat stale -- augments the quiet check, never replaces it', () => {
    const root = temporaryWorktreesRoot();
    const worktree = worktreeWithOffset(
      root,
      'agent-orphaned',
      '3',
      QUIET_MS + 60_000,
      now
    );
    const heartbeat = e2eHeartbeatPath(worktree);
    fs.writeFileSync(heartbeat, String(now));
    const stale = new Date(now - HEARTBEAT_STALE_MS - 1000);
    fs.utimesSync(heartbeat, stale, stale);
    restampWorktreeQuiet(worktree, now);
    expect(recentlyTouched(worktree, now)).toBe(false); // the worktree itself really is quiet
    expect(liveOffsets(root, now)).toEqual(new Set());
  });
});

describe('discoverPidfiles', () => {
  test('finds all three kinds, offset parsed out of the filename', () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 4, 111, 0, Date.now());
    writePidfile(dir, 'firebase-emulator', 4, 222, 0, Date.now());
    writePidfile(dir, 'mailbox', 4, 333, 0, Date.now());
    const found = discoverPidfiles(dir).sort((a, b) =>
      a.kind.localeCompare(b.kind)
    );
    expect(
      found.map((entry) => ({ kind: entry.kind, offset: entry.offset }))
    ).toEqual([
      { kind: 'api', offset: 4 },
      { kind: 'firebase-emulator', offset: 4 },
      { kind: 'mailbox', offset: 4 },
    ]);
  });

  test("ignores the main checkout's own unsuffixed pidfile", () => {
    const dir = temporaryPidfileDir();
    fs.writeFileSync(path.join(dir, 'doula-cloud-e2e-api.pid'), '111');
    expect(discoverPidfiles(dir)).toEqual([]);
  });

  test('ignores a file that only starts like ours', () => {
    const dir = temporaryPidfileDir();
    fs.writeFileSync(
      path.join(dir, 'doula-cloud-e2e-attachments-api-4.pid'),
      '111'
    );
    expect(discoverPidfiles(dir)).toEqual([]);
  });

  test('ignores an unrelated file in the same directory', () => {
    const dir = temporaryPidfileDir();
    fs.writeFileSync(path.join(dir, 'some-other-thing.pid'), '111');
    expect(discoverPidfiles(dir)).toEqual([]);
  });

  test('a directory that cannot be read yields nothing rather than throwing', () => {
    expect(
      discoverPidfiles(path.join(temporaryPidfileDir(), 'does-not-exist'))
    ).toEqual([]);
  });
});

describe('pickReapPidfiles', () => {
  const now = Date.now();
  const none = new Set<number>();

  test('reaps a pidfile older than the threshold whose offset no live worktree claims', () => {
    const dir = temporaryPidfileDir();
    const filePath = writePidfile(
      dir,
      'api',
      4,
      111,
      REAP_THRESHOLD_MS + 1000,
      now
    );
    expect(pickReapPidfiles(discoverPidfiles(dir), none, now)).toEqual([
      { kind: 'api', offset: 4, path: filePath, mtimeMs: expect.any(Number) },
    ]);
  });

  test('never reaps a pidfile whose offset a live worktree claims, however old', () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 4, 111, REAP_THRESHOLD_MS * 100, now);
    expect(pickReapPidfiles(discoverPidfiles(dir), new Set([4]), now)).toEqual(
      []
    );
  });

  test('does not reap a pidfile younger than the threshold', () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 4, 111, REAP_THRESHOLD_MS - 1000, now);
    expect(pickReapPidfiles(discoverPidfiles(dir), none, now)).toEqual([]);
  });

  test('accepts a custom threshold', () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 4, 111, 5000, now);
    expect(
      pickReapPidfiles(discoverPidfiles(dir), none, now, 1000)
    ).toHaveLength(1);
  });
});

describe('isProcessAlive', () => {
  test('true for a process this machine actually has running, false once it has exited', async () => {
    // A process the test itself spawns and owns -- never a real agent's.
    const child = spawn('sleep', ['5'], { stdio: 'ignore' });
    await new Promise<void>((resolve, reject) => {
      child.once('spawn', () => resolve());
      child.once('error', reject);
    });
    expect(isProcessAlive(child.pid!)).toBe(true);
    child.kill();
    await new Promise((resolve) => child.once('exit', resolve));
    expect(isProcessAlive(child.pid!)).toBe(false);
  });

  test('false for a pid nothing on this machine plausibly holds', () => {
    expect(isProcessAlive(999_999)).toBe(false);
  });
});

describe('looksLikeOurProcess', () => {
  test("api: matches only its own offset's exact binary path", () => {
    const commandLine = e2eApiBinaryPath(4);
    expect(looksLikeOurProcess('api', 4, commandLine)).toBe(true);
    expect(looksLikeOurProcess('api', 5, commandLine)).toBe(false); // a different worktree's binary
  });

  test("mailbox: matches a bun invocation of any worktree's mailbox.ts", () => {
    expect(
      looksLikeOurProcess('mailbox', 4, 'bun /some/worktree/app/e2e/mailbox.ts')
    ).toBe(true);
    expect(
      looksLikeOurProcess('mailbox', 4, 'bun /some/worktree/app/e2e/stack.ts')
    ).toBe(false);
  });

  test('firebase-emulator: matches the firebase-tools emulator invocation', () => {
    expect(
      looksLikeOurProcess(
        'firebase-emulator',
        4,
        'node bunx firebase-tools emulators:start --only auth --project doula-cloud'
      )
    ).toBe(true);
    expect(
      looksLikeOurProcess(
        'firebase-emulator',
        4,
        'node some-unrelated-server.js'
      )
    ).toBe(false);
  });

  test('never matches an unreadable command line -- a pid ps could not identify', () => {
    expect(looksLikeOurProcess('api', 4, undefined)).toBe(false);
  });
});

describe('e2e-stack-reap hook (subprocess, fail-open behavior)', () => {
  test('fails open when the container engine is unreachable', async () => {
    const { exitCode, stdout } = await invoke({
      ...process.env,
      DOCKER_HOST: 'unix:///nonexistent/e2e-stack-reap-test.sock',
      CONTAINER_ENGINE: '/nonexistent-binary-e2e-stack-reap-test',
    });
    expect(exitCode).toBe(0);
    expect(stdout).toBe('');
  });

  /*
   * DOCKER_HOST unset is the ordinary case for a hook, not an exotic
   * one: docs/testing.md has it exported by hand into the shell that
   * runs the tests, never from a login profile, so a SessionStart hook
   * never inherits it. It must still reach the engine (container-engine
   * .ts drops `--url` rather than giving up) and must still fail open
   * when there is no engine to reach.
   */
  test('fails open with no DOCKER_HOST rather than giving up on the engine', async () => {
    const env: Record<string, string | undefined> = {
      ...process.env,
      CONTAINER_ENGINE: '/nonexistent-binary-e2e-stack-reap-test',
    };
    delete env.DOCKER_HOST;
    const { exitCode, stdout } = await invoke(env);
    expect(exitCode).toBe(0);
    expect(stdout).toBe('');
  });
});
