/**
 * `.claude/hooks/e2e-stack-reap.ts` -- the SessionStart hook that tears
 * down the podman-compose e2e stack, and the host-process pidfiles beside
 * it, that a killed worktree session leaves running (#1066, #1194). See
 * that file's header and docs/testing/e2e.md's "Reaping orphaned e2e stacks"
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
  attachedOffsets,
  discoverPidfiles,
  groupStackProjects,
  heartbeatLiveOffsets,
  inspectProcess,
  isHeartbeatFresh,
  isProcessAlive,
  liveOffsets,
  liveWorktreeOffsets,
  looksLikeOurProcess,
  parseProcessInfo,
  pickReapPidfiles,
  pickReapProjects,
  planPidfiles,
  planStacks,
  recentlyTouched,
  type ProcessInfo,
  type StackProject,
} from '../.claude/hooks/e2e-stack-reap.ts';
import type { ReapCandidate } from '../.claude/hooks/container-engine.ts';

const REPO_ROOT = path.resolve(import.meta.dir, '..');
const HOOK = path.join(REPO_ROOT, '.claude', 'hooks', 'e2e-stack-reap.ts');

function invoke(
  env: Record<string, string | undefined>,
  args: string[] = []
): Promise<{ exitCode: number; stdout: string }> {
  return new Promise((resolve, reject) => {
    const child = spawn('bun', [HOOK, ...args], {
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

// A `ps` stand-in: answers only for the pids it was given.
function fakeInspector(
  table: Record<number, ProcessInfo>
): (pid: number) => ProcessInfo | undefined {
  return (pid) => table[pid];
}

describe('liveOffsets', () => {
  const now = Date.now();
  // Never this machine's real os.tmpdir(): other worktrees' real pidfiles
  // are there, and would make every expected set below nondeterministic.
  const noPidfiles = () => ({
    pidfileDir: temporaryPidfileDir(),
    inspect: fakeInspector({}),
  });

  test('live: worktree touched, no heartbeat at all', () => {
    const root = temporaryWorktreesRoot();
    worktreeWithOffset(root, 'agent-touched', '1', 0, now);
    expect(liveOffsets(root, now, noPidfiles())).toEqual(new Set([1]));
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
    expect(liveOffsets(root, now, noPidfiles())).toEqual(new Set([2]));
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
    expect(liveOffsets(root, now, noPidfiles())).toEqual(new Set());
  });

  /*
   * The case #1687 is about: a `dev:full` left running in a worktree
   * nobody has touched for hours, on a Mac that has just woken from
   * sleep, so its heartbeat timer has not yet caught up. The worktree is
   * quiet, the heartbeat is stale, and the stack is old -- and its BFF
   * is still the child of the live `dev:full` that started it.
   */
  test('live: worktree quiet and heartbeat stale, but its BFF is still attached to dev:full (#1687)', () => {
    const root = temporaryWorktreesRoot();
    const worktree = worktreeWithOffset(
      root,
      '1941-pen-dates',
      '2',
      QUIET_MS + 60_000,
      now
    );
    const heartbeat = e2eHeartbeatPath(worktree);
    fs.writeFileSync(heartbeat, String(now));
    const stale = new Date(now - 8 * 60 * 60 * 1000);
    fs.utimesSync(heartbeat, stale, stale);
    restampWorktreeQuiet(worktree, now);

    const pidfileDir = temporaryPidfileDir();
    writePidfile(pidfileDir, 'api', 2, 34217, 13 * 60 * 60 * 1000, now);
    const inspect = fakeInspector({
      34217: { ppid: 32867, command: e2eApiBinaryPath(2) },
    });

    expect(liveOffsets(root, now, { pidfileDir, inspect })).toEqual(
      new Set([2])
    );
    // And so neither the stack nor its host processes are picked.
    expect(
      pickReapProjects(
        [stack('doula-cloud-e2e-2', 2, now - 13 * 60 * 60 * 1000)],
        liveOffsets(root, now, { pidfileDir, inspect }),
        now
      )
    ).toEqual([]);
  });
});

describe('attachedOffsets', () => {
  const now = Date.now();

  test('claims the offset of a host process whose parent is alive', () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 2, 500, 0, now);
    writePidfile(dir, 'mailbox', 3, 501, 0, now);
    writePidfile(dir, 'firebase-emulator', 5, 502, 0, now);
    const inspect = fakeInspector({
      500: { ppid: 400, command: e2eApiBinaryPath(2) },
      501: { ppid: 401, command: 'bun /wt/app/e2e/mailbox.ts' },
      502: { ppid: 402, command: 'node firebase emulators:start --only auth' },
    });
    expect(attachedOffsets(dir, inspect)).toEqual(new Set([2, 3, 5]));
  });

  test('claims nothing for a host process reparented to launchd: its run is gone', () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 2, 500, 0, now);
    const inspect = fakeInspector({
      500: { ppid: 1, command: e2eApiBinaryPath(2) },
    });
    expect(attachedOffsets(dir, inspect)).toEqual(new Set());
  });

  test('claims nothing for a pid that is gone', () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 2, 500, 0, now);
    expect(attachedOffsets(dir, fakeInspector({}))).toEqual(new Set());
  });

  test('claims nothing for a pid recycled onto an unrelated process', () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 2, 500, 0, now);
    const inspect = fakeInspector({
      500: { ppid: 400, command: '/usr/bin/some-other-thing' },
    });
    expect(attachedOffsets(dir, inspect)).toEqual(new Set());
  });

  test("claims nothing for another offset's BFF binary", () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 2, 500, 0, now);
    const inspect = fakeInspector({
      500: { ppid: 400, command: e2eApiBinaryPath(3) },
    });
    expect(attachedOffsets(dir, inspect)).toEqual(new Set());
  });

  test('claims nothing for a pidfile that does not hold a pid', () => {
    const dir = temporaryPidfileDir();
    fs.writeFileSync(path.join(dir, 'doula-cloud-e2e-api-2.pid'), 'nope');
    const inspect = () => ({ ppid: 400, command: e2eApiBinaryPath(2) });
    expect(attachedOffsets(dir, inspect)).toEqual(new Set());
  });
});

describe('parseProcessInfo', () => {
  test("reads ps's ppid and command columns", () => {
    expect(
      parseProcessInfo('32867 /var/folders/T/doula-cloud-e2e-api-2\n')
    ).toEqual({ ppid: 32867, command: '/var/folders/T/doula-cloud-e2e-api-2' });
  });

  test('keeps the spaces inside a command line', () => {
    expect(parseProcessInfo('    1 bun /wt/app/e2e/mailbox.ts')).toEqual({
      ppid: 1,
      command: 'bun /wt/app/e2e/mailbox.ts',
    });
  });

  test('undefined for empty or unrecognized output', () => {
    expect(parseProcessInfo('')).toBeUndefined();
    expect(parseProcessInfo('PPID COMMAND')).toBeUndefined();
  });
});

describe('inspectProcess', () => {
  test('reads this process: its real parent, and bun in its command line', () => {
    const info = inspectProcess(process.pid);
    expect(info?.ppid).toBe(process.ppid);
    expect(info?.command).toContain('bun');
  });

  test('undefined for a pid nothing on this machine plausibly holds', () => {
    expect(inspectProcess(999_999)).toBeUndefined();
  });
});

describe('planPidfiles', () => {
  const now = Date.now();
  const alive = (pid: number) => pid === 500 || pid === 501;
  const inspect = fakeInspector({
    500: { ppid: 1, command: e2eApiBinaryPath(4) },
    501: { ppid: 1, command: '/usr/bin/unrelated' },
  });

  test('kills an orphan that is still the process its pidfile names', () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 4, 500, 0, now);
    const entry = discoverPidfiles(dir)[0]!;
    expect(planPidfiles([entry], alive, inspect)).toEqual([
      { entry, action: 'kill', pid: 500 },
    ]);
  });

  test('only forgets a pidfile whose pid is gone', () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 4, 777, 0, now);
    const entry = discoverPidfiles(dir)[0]!;
    expect(planPidfiles([entry], alive, inspect)).toEqual([
      { entry, action: 'forget' },
    ]);
  });

  test('only forgets a pidfile that holds no pid', () => {
    const dir = temporaryPidfileDir();
    fs.writeFileSync(path.join(dir, 'doula-cloud-e2e-api-4.pid'), '');
    const entry = discoverPidfiles(dir)[0]!;
    expect(planPidfiles([entry], alive, inspect)).toEqual([
      { entry, action: 'forget' },
    ]);
  });

  test('does nothing to a live pid that is not the process its pidfile names', () => {
    const dir = temporaryPidfileDir();
    writePidfile(dir, 'api', 4, 501, 0, now);
    expect(planPidfiles(discoverPidfiles(dir), alive, inspect)).toEqual([]);
  });
});

describe('planStacks', () => {
  const now = Date.now();
  const old = Math.floor((now - REAP_THRESHOLD_MS * 4) / 1000);
  const ps = JSON.stringify([
    {
      Id: 'orphan',
      Names: ['doula-cloud-e2e-1_db_1'],
      Labels: { 'com.docker.compose.project': 'doula-cloud-e2e-1' },
      Created: old,
    },
    {
      Id: 'live',
      Names: ['doula-cloud-e2e-2_db_1'],
      Labels: { 'com.docker.compose.project': 'doula-cloud-e2e-2' },
      Created: old,
    },
  ]);

  test('asks the engine for compose containers and picks only the dead offset', () => {
    const calls: string[][] = [];
    const exec = (args: string[]) => {
      calls.push(args);
      return ps;
    };
    expect(
      planStacks(new Set([2]), now, exec).map((project) => project.project)
    ).toEqual(['doula-cloud-e2e-1']);
    expect(calls).toEqual([
      [
        'ps',
        '-a',
        '--filter',
        'label=com.docker.compose.project',
        '--format',
        'json',
      ],
    ]);
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
  /*
   * TMPDIR pointed at an empty fixture dir, not neutered like
   * CONTAINER_ENGINE below: reapPidfiles needs no container engine at
   * all, so nothing else here stops it from scanning os.tmpdir() for
   * real. Without this override these tests would run the real pidfile
   * sweep against whatever this machine's actual os.tmpdir() holds --
   * possibly another live agent's own pidfiles -- on every `bun test
   * scripts/`, making `expect(stdout).toBe('')` nondeterministic and,
   * far worse, capable of actually killing something. Node/Bun's
   * os.tmpdir() reads TMPDIR on POSIX before falling back to `/tmp`.
   */
  test('fails open when the container engine is unreachable', async () => {
    const { exitCode, stdout } = await invoke({
      ...process.env,
      DOCKER_HOST: 'unix:///nonexistent/e2e-stack-reap-test.sock',
      CONTAINER_ENGINE: '/nonexistent-binary-e2e-stack-reap-test',
      TMPDIR: temporaryPidfileDir(),
    });
    expect(exitCode).toBe(0);
    expect(stdout).toBe('');
  });

  /*
   * DOCKER_HOST unset is an ordinary case for a hook, not an exotic one:
   * docs/testing/api.md has it set by hand on the command that runs the
   * tests, and a SessionStart hook inherits the login environment, not
   * that shell's. (A login profile can export one of its own, for
   * another engine; the fake-engine tests below drive that case.) The
   * hook must still reach the engine (container-engine.ts drops `--url`
   * rather than giving up) and must still fail open when there is no
   * engine to reach.
   */
  test('fails open with no DOCKER_HOST rather than giving up on the engine', async () => {
    const env: Record<string, string | undefined> = {
      ...process.env,
      CONTAINER_ENGINE: '/nonexistent-binary-e2e-stack-reap-test',
      TMPDIR: temporaryPidfileDir(),
    };
    delete env.DOCKER_HOST;
    const { exitCode, stdout } = await invoke(env);
    expect(exitCode).toBe(0);
    expect(stdout).toBe('');
  });
});

/*
 * main() against a fake engine binary: a shell script that refuses every
 * `--url` call -- the way `podman --url <a Colima Docker socket>` fails
 * with `ping response was 404` -- answers a bare `ps` with canned JSON,
 * and logs every call. So no real container is touched here.
 *
 * main() reads this machine's real `.claude/worktrees` (it finds the
 * main checkout from the hook's own path), so the canned stack sits at
 * offset 77: no worktree can hold it, since only 9 offsets exist, and no
 * live worktree can make it look alive. TMPDIR is a fixture dir, for the
 * reason the block above gives.
 */
describe('e2e-stack-reap hook (subprocess, fake engine, #1687)', () => {
  const ORPHAN_PROJECT = 'doula-cloud-e2e-77';

  function fakeEngine(): { binary: string; log: string } {
    const dir = temporaryPidfileDir();
    const psFile = path.join(dir, 'ps.json');
    const log = path.join(dir, 'calls.log');
    const created = Math.floor((Date.now() - REAP_THRESHOLD_MS * 4) / 1000);
    fs.writeFileSync(
      psFile,
      JSON.stringify([
        {
          Id: 'orphan0000000077',
          Names: [`${ORPHAN_PROJECT}_db_1`],
          Labels: { 'com.docker.compose.project': ORPHAN_PROJECT },
          Created: created,
        },
      ])
    );
    const binary = path.join(dir, 'engine');
    fs.writeFileSync(
      binary,
      `#!/bin/sh\necho "$@" >> '${log}'\ncase "$1" in --url) exit 1;; ps) cat '${psFile}';; esac\n`,
      { mode: 0o755 }
    );
    return { binary, log };
  }

  function envFor(binary: string): Record<string, string | undefined> {
    return {
      ...process.env,
      DOCKER_HOST: 'unix:///Users/me/.colima/default/docker.sock',
      CONTAINER_ENGINE: binary,
      TMPDIR: temporaryPidfileDir(),
    };
  }

  test('--dry-run reaches the engine past a non-Podman DOCKER_HOST, lists the orphan and changes nothing', async () => {
    const engine = fakeEngine();
    const { exitCode, stdout } = await invoke(envFor(engine.binary), [
      '--dry-run',
    ]);
    expect(exitCode).toBe(0);
    expect(stdout).toContain('would tear down 1 orphaned e2e stack(s)');
    expect(stdout).toContain(`  ${ORPHAN_PROJECT} (1 container(s)`);
    expect(stdout).toContain('would kill 0 orphaned e2e host process(es)');
    const calls = fs.readFileSync(engine.log, 'utf8').trim().split('\n');
    expect(calls).toEqual([
      '--url unix:///Users/me/.colima/default/docker.sock ps -a --filter label=com.docker.compose.project --format json',
      'ps -a --filter label=com.docker.compose.project --format json',
    ]);
  });

  test('a real run tears the orphan down through the default connection', async () => {
    const engine = fakeEngine();
    const { exitCode, stdout } = await invoke(envFor(engine.binary));
    expect(exitCode).toBe(0);
    expect(stdout).toContain(
      `tore down 1 orphaned e2e stack(s) -- ${ORPHAN_PROJECT}`
    );
    const calls = fs.readFileSync(engine.log, 'utf8').trim().split('\n');
    expect(calls).toHaveLength(4);
    expect(calls[2]).toStartWith(
      `--url unix:///Users/me/.colima/default/docker.sock compose -p ${ORPHAN_PROJECT} `
    );
    expect(calls[3]).toStartWith(`compose -p ${ORPHAN_PROJECT} -f `);
    expect(calls[3]).toEndWith(' down -v -t 2');
  });

  test('--dry-run says so when the engine cannot be read at all', async () => {
    const { exitCode, stdout } = await invoke(
      {
        ...process.env,
        CONTAINER_ENGINE: '/nonexistent-binary-e2e-stack-reap-test',
        TMPDIR: temporaryPidfileDir(),
      },
      ['--dry-run']
    );
    expect(exitCode).toBe(0);
    expect(stdout).toContain('the container engine could not be read');
  });
});
