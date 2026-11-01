#!/usr/bin/env bun
// SessionStart hook: reaps the podman-compose e2e stack, and the
// host-process pidfiles beside it, that a killed worktree session leaves
// running. See docs/testing/e2e.md's "Reaping orphaned e2e stacks" section
// for the full story; the short version is below.
//
// app/e2e/stack.ts brings up app/compose.e2e.yaml under a per-worktree
// compose project name -- `doula-cloud-e2e-<PORT_OFFSET>` -- and tears it
// down again in stopStack. A killed session (an interrupted agent, a
// timeout, a cut-short Playwright run) never reaches that teardown, so
// its `db` and `gcs` containers, the project's network and its named
// volume sit on the shared 4GB Podman machine indefinitely. That is the
// same starvation #889 diagnosed and #782 most likely hit, and
// testdb-reap.ts does nothing about it: it only looks at containers
// carrying a Go test's owner label (`cloud.doula.test-container=true`),
// which a compose stack is not.
//
// The same file spawns three host processes -- the Firebase Auth
// emulator, the Go BFF and the sandbox mailbox -- `detached` and
// `unref()`'d, tracked by pidfile in os.tmpdir() rather than by compose
// (#1194). A killed session leaves those running too, and unlike the
// containers above they hold the offset's actual TCP ports: a killed
// session's orphaned BFF answering on its port is what makes
// worktree-provision.ts's `isPortFree` refuse to hand that offset to a
// new worktree at all, even after the worktree that used to claim it is
// gone -- with only 9 offsets on the machine, that is how "no usable
// port offset" happens. groupStackProjects/pickReapProjects below decide
// which compose stacks are orphaned; discoverPidfiles/pickReapPidfiles
// answer the same question for pidfiles, under the same rule
// (liveOffsets, REAP_THRESHOLD_MS) so a live worktree, a fresh heartbeat
// or a host process still attached to the run that started it protects
// both the same way. Killing a pidfile's pid is the
// one step that is not just cleanup: looksLikeOurProcess below is what
// keeps a dead-and-recycled pid from ever being handed to `kill` just
// because a stale pidfile still names it.
//
// The reap decisions (groupStackProjects/pickReapProjects,
// discoverPidfiles/pickReapPidfiles) and the filesystem reader behind the
// latter are exported for direct unit testing -- pickReapProjects/
// pickReapPidfiles are pure, discoverPidfiles reads the filesystem the
// same way recentlyTouched/liveWorktreeOffsets already do below. main()
// -- the engine invocation, the compose teardown, the pidfile kills,
// `--dry-run`, the fail-open catch -- is exercised as a subprocess
// instead; see scripts/e2e-stack-reap.test.ts.
//
// `--dry-run` prints what a real run would tear down and kill, from the
// same plan the real run executes, and changes nothing.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import {
  E2E_COMPOSE_FILE,
  E2E_COMPOSE_PROJECT_PREFIX,
  E2E_HEARTBEAT_INTERVAL_MS,
  E2E_PIDFILE_KINDS,
  e2eApiBinaryPath,
  e2eOpsBinaryPath,
  e2eHeartbeatPath,
  type E2EPidfileKind,
} from '../../app/e2e/ports.ts';
import {
  execEngine,
  parseContainers,
  type ReapCandidate,
} from './container-engine.ts';
import { readWorktreeOffsets } from './worktree-offsets.ts';
import { findMainCheckoutRoot } from './worktree-root.ts';

// podman-compose 1.6.0 and Docker Compose v2 both stamp this key on every
// container they create (confirmed by starting app/compose.e2e.yaml on
// this machine and reading the labels back). podman-compose writes its
// own `io.podman.compose.project` alongside it; the docker-namespaced key
// is the one both engines agree on, so it is the one matched here.
const COMPOSE_PROJECT_LABEL = 'com.docker.compose.project';

/*
 * Only a worktree's stack, never the main checkout's.
 *
 * Built from `E2E_COMPOSE_PROJECT_PREFIX` rather than spelled out, so a
 * rename in app/e2e/ports.ts -- the file that names these projects in the
 * first place -- cannot leave this hook silently matching nothing.
 *
 * The `-<digits>` is required, and that is the exclusion: e2eComposeProject
 * suffixes the name only for a non-zero offset, so the bare prefix is the
 * main checkout's and CI's. The main checkout has no `.port-offset` file
 * to go quiet, it is where a long interactive `bun run dev:full` runs, and
 * it is the one place a person is standing when they would notice their
 * own database vanish. Its stack is left alone.
 */
const WORKTREE_PROJECT_PATTERN = new RegExp(
  `^${E2E_COMPOSE_PROJECT_PREFIX}-(\\d+)$`
);

/*
 * Two clocks decide, and neither on its own.
 *
 * REAP_THRESHOLD_MS is the stack's own age, and it is the weaker of the
 * two: unlike a testcontainers Postgres, which backs one `go test`
 * process and lives minutes at most, an e2e stack legitimately outlives
 * any fixed budget -- `bun run dev:full` shares it and runs for as long
 * as somebody is working. So age alone must never authorize a reap. It is
 * here to cover the one window the quiet check cannot: a stack brought up
 * seconds ago inside a worktree whose files happen not to have been
 * touched since. 15 minutes is far longer than
 * `compose up -d` plus the migrate (<=90s) and `go build` (<=120s) steps
 * that follow it.
 *
 * QUIET_MS is one of two liveness signals: has anything touched the
 * worktree that claims this stack's port offset lately. It is the same
 * 30 minutes, for the same reason, that worktree-prune.ts waits before
 * it will touch a worktree at all -- half an hour of silence is that
 * file's own measure of "nobody is standing here", written because a
 * session that has just landed its PR sits in a finished worktree for as
 * long as it takes to write a summary. Be clear about what is NOT being
 * borrowed: prune removes a directory only when it is quiet AND clean
 * AND merged, because deleting a worktree can destroy unlanded work.
 * Nothing here can: a reaped stack is a database that is rebuilt by the
 * next `up -d`, and its contents are fixtures. Quiet plus the age check
 * below is the whole bar precisely because the stakes are lower.
 *
 * But QUIET_MS alone misses a real case (#1193): a `bun run test:e2e` or
 * `bun run dev:full` run in progress touches none of the three things
 * QUIET_MS reads -- the worktree directory, its git dir, that dir's
 * index -- for as long as it runs. A session's own hooks rewrite the
 * git index on nearly every Bash command, which is what keeps this
 * narrow, but a long Playwright run or a quiet stretch of `dev:full`
 * can still outrun 30 minutes with the stack very much in use.
 * HEARTBEAT_STALE_MS/isHeartbeatFresh below is the second signal that
 * closes that gap: `e2e/stack.ts`'s startStack/stopStack keep a file
 * beside the worktree's `.port-offset` fresh for exactly as long as the
 * stack is actually being used, the same heartbeat shape
 * `scripts/gate-lock.ts` already uses for its own lock. It only ever
 * ADDS liveness on top of the quiet check, never replaces it: an offset
 * is live if either signal says so, and a killed session simply stops
 * writing the heartbeat, so an orphaned stack still ages into
 * reapability once both clocks below run out.
 *
 * The heartbeat has a gap of its own (#1687): it is a wall-clock mtime
 * written by a timer, and a Mac that sleeps with `dev:full` running
 * wakes with the file hours old until the next tick. A session started
 * in that window would read a quiet worktree and a stale heartbeat, and
 * find the stack older than 15 minutes. attachedOffsets below is the
 * third signal, and it closes that gap without any writer's help: the
 * stack's host processes (the BFF, the mailbox, the emulator) are
 * children of the `dev:full` or Playwright process that started them,
 * and the kernel reparents a child to launchd (pid 1) the moment its
 * parent dies. So a host process that is alive, is the process its
 * pidfile names, and still has a parent other than pid 1 is proof that
 * the run which owns the stack is still alive -- kernel state, read
 * fresh, with no timer to fall behind.
 *
 * Whether the stack's host BFF is up, on its own, is still NOT a liveness
 * signal. app/e2e/stack.ts spawns it `detached` and `unref`s it, so it
 * outlives the session that started it -- an orphaned stack's BFF is
 * still listening on its port, which would make every orphan look alive
 * and this hook a no-op. The parent is what tells the two apart.
 */
export const REAP_THRESHOLD_MS = 15 * 60 * 1000;
export const QUIET_MS = 30 * 60 * 1000;
// Five heartbeats' worth of silence -- comfortably past a slow tick from
// event-loop pressure, the same "generously above anything a live run
// could produce" reasoning REAP_THRESHOLD_MS above and gate-lock.ts's own
// STALE_LOCK_MS both use. Derived from the writer's own interval so this
// can never be set shorter than the beat that is supposed to satisfy it.
export const HEARTBEAT_STALE_MS = E2E_HEARTBEAT_INTERVAL_MS * 5;

export interface StackProject {
  project: string;
  offset: number;
  // The newest container in the project, not the oldest: the whole
  // project is only as stale as its most recent sign of life.
  newestCreatedAtMs: number;
  containerIds: string[];
}

// Collapses a flat container list into the compose projects behind it,
// keeping only names app/e2e/stack.ts could have produced for a worktree.
export function groupStackProjects(
  containers: ReapCandidate[]
): StackProject[] {
  const projects = new Map<string, StackProject>();
  for (const container of containers) {
    const name = container.labels[COMPOSE_PROJECT_LABEL];
    if (name === undefined) continue;
    const match = WORKTREE_PROJECT_PATTERN.exec(name);
    if (match === null) continue;

    const existing = projects.get(name);
    if (existing === undefined) {
      projects.set(name, {
        project: name,
        offset: Number(match[1]),
        newestCreatedAtMs: container.createdAtMs,
        containerIds: [container.id],
      });
      continue;
    }
    existing.containerIds.push(container.id);
    existing.newestCreatedAtMs = Math.max(
      existing.newestCreatedAtMs,
      container.createdAtMs
    );
  }
  return [...projects.values()];
}

// The one place that decides what gets torn down. Matches the project
// name itself rather than trusting the engine's own `--filter` flag to
// have done it -- a destructive command should not depend solely on a
// filter string built elsewhere staying correct.
export function pickReapProjects(
  projects: StackProject[],
  liveOffsets: ReadonlySet<number>,
  nowMs: number,
  thresholdMs: number = REAP_THRESHOLD_MS
): StackProject[] {
  return projects.filter(
    (project) =>
      !liveOffsets.has(project.offset) &&
      nowMs - project.newestCreatedAtMs > thresholdMs
  );
}

/*
 * Has anything touched this worktree lately?
 *
 * Three timestamps: the worktree's own directory, the git dir its `.git`
 * file points at, and that dir's index. worktree-prune.ts's own
 * recentlyTouched reads the same three, and asks git for the middle one;
 * this reads the pointer file instead, so a hook that must fail open never
 * depends on a subprocess it would have to catch around.
 *
 * Fails closed, exactly as prune's does: a worktree that is there but
 * whose timestamps cannot be read at all counts as touched and is left
 * alone. Only a directory that is not there answers "no" -- and that one
 * is not a guess, it is the session's directory being gone.
 */
export function recentlyTouched(
  worktreePath: string,
  nowMs: number,
  quietMs: number = QUIET_MS
): boolean {
  if (!fs.existsSync(worktreePath)) return false;

  const candidates = [worktreePath];
  try {
    const pointer = fs
      .readFileSync(path.join(worktreePath, '.git'), 'utf8')
      .trim();
    const gitDir = pointer.startsWith('gitdir:')
      ? pointer.slice('gitdir:'.length).trim()
      : path.join(worktreePath, '.git');
    candidates.push(gitDir, path.join(gitDir, 'index'));
  } catch {
    // No readable `.git` -- the directory itself still answers below.
  }

  let readAny = false;
  for (const candidate of candidates) {
    try {
      const mtimeMs = fs.statSync(candidate).mtimeMs;
      readAny = true;
      if (nowMs - mtimeMs < quietMs) return true;
    } catch {
      // A missing path tells us nothing -- keep checking the others.
    }
  }
  return !readAny;
}

/*
 * The port offsets whose worktree is both present and recently touched.
 *
 * An offset missing from this set is one no live session can be using:
 * either no worktree carries that `.port-offset` at all (the session's
 * directory is gone), or the one that does has been quiet for QUIET_MS.
 * The scan itself is shared with worktree-provision.ts
 * (worktree-offsets.ts); only the quiet test is this hook's, because
 * provisioning must refuse an offset any directory holds however quiet,
 * while this is asking the narrower question of whether a running
 * container still has a session behind it.
 */
export function liveWorktreeOffsets(
  worktreesRoot: string,
  nowMs: number,
  quietMs: number = QUIET_MS
): Set<number> {
  return new Set(
    readWorktreeOffsets(worktreesRoot)
      .filter((worktree) => recentlyTouched(worktree.path, nowMs, quietMs))
      .map((worktree) => worktree.offset)
  );
}

/*
 * Is a heartbeat file (e2e/ports.ts's e2eHeartbeatPath) still being
 * written to? Takes the path directly rather than an offset, so a spec
 * can point this at a fixture file without touching a real worktree.
 *
 * Deliberately asymmetric with recentlyTouched's fail-CLOSED (unreadable
 * counts as touched, to protect against deleting live work): a missing
 * or unreadable heartbeat here just means this offset isn't using the
 * signal, and falls back to the worktree quiet check like it always
 * did. This function can only ever ADD liveness on top of that check
 * (see the block above QUIET_MS/HEARTBEAT_STALE_MS), so failing closed
 * here would buy nothing -- the worktree check already covers the
 * "can't tell" case on its own.
 */
export function isHeartbeatFresh(
  heartbeatPath: string,
  nowMs: number,
  staleMs: number = HEARTBEAT_STALE_MS
): boolean {
  try {
    return nowMs - fs.statSync(heartbeatPath).mtimeMs < staleMs;
  } catch {
    return false; // no heartbeat file -- defer entirely to the other check
  }
}

// Offsets whose worktree holds a heartbeat file that is still fresh --
// the second half of liveOffsets below.
export function heartbeatLiveOffsets(
  worktreesRoot: string,
  nowMs: number,
  staleMs: number = HEARTBEAT_STALE_MS
): Set<number> {
  return new Set(
    readWorktreeOffsets(worktreesRoot)
      .filter((worktree) =>
        isHeartbeatFresh(e2eHeartbeatPath(worktree.path), nowMs, staleMs)
      )
      .map((worktree) => worktree.offset)
  );
}

// What `ps` says a pid is: its parent, and its full command line.
export interface ProcessInfo {
  ppid: number;
  command: string;
}

export type ProcessInspector = (pid: number) => ProcessInfo | undefined;

// launchd, which every orphan on macOS is reparented to. (Linux can
// reparent to a subreaper instead; this hook only ever runs on the Mac
// the worktrees live on, and CI never meets it.)
const ORPHAN_PARENT_PID = 1;

/*
 * Offsets that a host process still attached to its run keeps live
 * (#1687; see the block above REAP_THRESHOLD_MS). For each worktree
 * pidfile: the pid must be alive, must be the process its pidfile names
 * (looksLikeOurProcess, the same check the kill path uses), and must
 * still have a parent other than pid 1. A pid `ps` cannot read is
 * treated as nothing to add, never as an error: this can only ever ADD
 * liveness, so "cannot tell" falls back to the other two signals.
 *
 * A pid the OS has recycled onto another worktree's own mailbox or
 * emulator could pass the weaker identity markers those two kinds have;
 * that keeps a stack, never removes one.
 */
export function attachedOffsets(
  pidfileDir: string,
  inspect: ProcessInspector = inspectProcess
): Set<number> {
  const offsets = new Set<number>();
  for (const entry of discoverPidfiles(pidfileDir)) {
    const pid = readPid(entry.path);
    if (pid === undefined) continue;
    const info = inspect(pid);
    if (info === undefined || info.ppid === ORPHAN_PARENT_PID) continue;
    if (!looksLikeOurProcess(entry.kind, entry.offset, info.command)) continue;
    offsets.add(entry.offset);
  }
  return offsets;
}

export interface LivenessSources {
  quietMs?: number;
  staleMs?: number;
  // Where the host-process pidfiles live; os.tmpdir() unless a spec
  // points it at a fixture directory.
  pidfileDir?: string;
  inspect?: ProcessInspector;
}

// The one place the three signals are combined: an offset is live if
// its worktree has been touched recently, OR its heartbeat is still
// fresh (#1193), OR one of its host processes is still attached to the
// run that started it (#1687). Each only ever adds liveness. This is
// what main() passes to pickReapProjects as `liveOffsets`.
export function liveOffsets(
  worktreesRoot: string,
  nowMs: number,
  {
    quietMs = QUIET_MS,
    staleMs = HEARTBEAT_STALE_MS,
    pidfileDir = tmpdir(),
    inspect = inspectProcess,
  }: LivenessSources = {}
): Set<number> {
  return new Set([
    ...liveWorktreeOffsets(worktreesRoot, nowMs, quietMs),
    ...heartbeatLiveOffsets(worktreesRoot, nowMs, staleMs),
    ...attachedOffsets(pidfileDir, inspect),
  ]);
}

/*
 * Matches a pidfile app/e2e/ports.ts's e2ePidfilePath could have
 * produced for a worktree -- never the main checkout's or CI's. The
 * offset group is mandatory, not optional: e2ePidfilePath omits the
 * suffix only at offset 0, so requiring `-(\d+)` here is what keeps an
 * unsuffixed `doula-cloud-e2e-api.pid` (the main checkout's own, or a
 * plain `bun run dev:full` outside any worktree) from ever matching --
 * the same exclusion WORKTREE_PROJECT_PATTERN above makes for compose
 * projects, and for the same reason: that pidfile's offset (0) is never
 * in liveOffsets' output (main checkout has no `.port-offset` to find),
 * so without this exclusion a long-lived interactive session would read
 * as orphaned the moment it went quiet for REAP_THRESHOLD_MS.
 */
const PIDFILE_PATTERN = new RegExp(
  `^${E2E_COMPOSE_PROJECT_PREFIX}-(${E2E_PIDFILE_KINDS.join('|')})-(\\d+)\\.pid$`
);

export interface PidfileEntry {
  kind: E2EPidfileKind;
  offset: number;
  path: string;
  mtimeMs: number;
}

// Every worktree-suffixed doula-cloud-e2e-* pidfile os.tmpdir() is
// currently holding. A file that vanishes between the directory listing
// and the stat (a session's own teardown racing this scan) is simply
// left out -- there is nothing left to reap.
export function discoverPidfiles(tmpDir: string): PidfileEntry[] {
  let names: string[];
  try {
    names = fs.readdirSync(tmpDir);
  } catch {
    return [];
  }

  const found: PidfileEntry[] = [];
  for (const name of names) {
    const match = PIDFILE_PATTERN.exec(name);
    if (match === null) continue;
    const filePath = path.join(tmpDir, name);
    try {
      found.push({
        kind: match[1] as E2EPidfileKind,
        offset: Number(match[2]),
        path: filePath,
        mtimeMs: fs.statSync(filePath).mtimeMs,
      });
    } catch {
      continue;
    }
  }
  return found;
}

// The same conjunction pickReapProjects applies to a compose stack,
// applied to a pidfile instead: not live by liveOffsets, AND older than
// thresholdMs. Age is read from the pidfile's own mtime -- killPidfile
// (e2e/stack.ts) removes and rewrites it fresh on every startAPI/
// startEmulator/startMailbox, so its mtime is that process's own start
// time, the direct pidfile equivalent of a container's `Created`.
export function pickReapPidfiles(
  entries: PidfileEntry[],
  liveOffsets: ReadonlySet<number>,
  nowMs: number,
  thresholdMs: number = REAP_THRESHOLD_MS
): PidfileEntry[] {
  return entries.filter(
    (entry) =>
      !liveOffsets.has(entry.offset) && nowMs - entry.mtimeMs > thresholdMs
  );
}

// True unless the pid is definitely gone (ESRCH). EPERM -- a pid that
// exists but this user cannot signal -- reads as alive: on this machine
// every one of these processes is spawned by, and owned by, the same
// user running this hook, so a pid this hook cannot signal is already a
// sign that whatever now holds it is not one of ours.
export function isProcessAlive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch (error) {
    return (error as NodeJS.ErrnoException).code !== 'ESRCH';
  }
}

/*
 * The one fact that lets this hook kill a pid at all: not just that a
 * pidfile names it, but that the pid's own command line still looks like
 * the process that pidfile was written for. This is what stands between
 * a genuinely orphaned BFF and a `kill` aimed at whatever the OS has
 * since recycled that pid onto -- another worktree's live process, or
 * anything else on the machine (see #1212, on what a wrong reclaim here
 * costs).
 *
 * `api`'s marker is the exact binary path startAPI built and exec'd
 * (e2eApiBinaryPath) -- offset-suffixed, so it is unique to this
 * pidfile's own offset and never matches a different worktree's BFF.
 * `ops`'s is startOps's binary (e2eOpsBinaryPath), the same way.
 * `mailbox` and `firebase-emulator` can't be pinned to an offset the
 * same way -- mailbox.ts's own path is the *worktree's* (its source
 * tree may no longer exist to check), and firebase-tools' invocation
 * carries no offset at all in its argv -- so their markers only confirm
 * "this is one of ours", the same bar killPidfile itself has always
 * applied to its own pidfile. That is weaker for those two kinds, and
 * deliberately so: it can still produce a false negative (skip a
 * genuine orphan) but never a false positive strong enough to kill an
 * unrelated process apart from another worktree's own mailbox/emulator,
 * which the offset-scoped liveOffsets check above already excludes from
 * ever reaching this function while that worktree is live.
 */
const IDENTITY_MARKER: Record<E2EPidfileKind, (offset: number) => string> = {
  api: (offset) => e2eApiBinaryPath(offset),
  ops: (offset) => e2eOpsBinaryPath(offset),
  mailbox: () => `${path.sep}app${path.sep}e2e${path.sep}mailbox.ts`,
  'firebase-emulator': () => 'emulators:start',
};

export function looksLikeOurProcess(
  kind: E2EPidfileKind,
  offset: number,
  commandLine: string | undefined
): boolean {
  if (!commandLine) return false;
  return commandLine.includes(IDENTITY_MARKER[kind](offset));
}

// Reads `ps -o ppid=,command=` output: the parent pid, then the command
// line after it. Undefined for anything else (an empty line from a pid
// that is gone, or output this does not recognize).
export function parseProcessInfo(output: string): ProcessInfo | undefined {
  const match = /^\s*(\d+)\s+(\S.*)$/s.exec(output.trim());
  const [, ppid, command] = match ?? [];
  if (ppid === undefined || command === undefined) return undefined;
  return { ppid: Number(ppid), command };
}

// `ps`'s own view of what a pid is actually running and who its parent
// is, not what the pidfile claims. `-ww` asks for the untruncated line --
// os.tmpdir() paths on macOS routinely run past a terminal-width default.
// Returns undefined on any failure (pid gone, `ps` missing, anything
// else): looksLikeOurProcess treats that the same as "does not match",
// attachedOffsets as "nothing to add", and this hook fails open around it.
export function inspectProcess(pid: number): ProcessInfo | undefined {
  try {
    return parseProcessInfo(
      execFileSync('ps', ['-p', String(pid), '-o', 'ppid=,command=', '-ww'], {
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'ignore'],
      })
    );
  } catch {
    return undefined;
  }
}

// The pid a pidfile names, or undefined when the file is gone or does
// not hold a plain positive integer.
function readPid(pidfilePath: string): number | undefined {
  try {
    const pid = Number(fs.readFileSync(pidfilePath, 'utf8').trim());
    return Number.isInteger(pid) && pid > 0 ? pid : undefined;
  } catch {
    return undefined;
  }
}

export interface PidfileAction {
  entry: PidfileEntry;
  // `kill`: the pid is alive and is the process its pidfile names, so
  // it is killed and its pidfile removed. `forget`: the pidfile names
  // nothing that is running (or nothing at all), so only the file goes.
  action: 'kill' | 'forget';
  pid?: number;
}

/*
 * What a real run does to each pidfile pickReapPidfiles chose -- and what
 * a dry run prints, from the same function, so the two cannot disagree.
 * A pid that is alive but is not the process its pidfile names gets no
 * action at all: almost certainly a pid the OS has recycled onto
 * something unrelated since. Killing the pid is the one irreversible
 * step here, and "cannot confirm" must resolve to "do nothing" (see
 * #1212 on what a wrong reclaim costs).
 */
export function planPidfiles(
  doomed: PidfileEntry[],
  isAlive: (pid: number) => boolean = isProcessAlive,
  inspect: ProcessInspector = inspectProcess
): PidfileAction[] {
  const actions: PidfileAction[] = [];
  for (const entry of doomed) {
    const pid = readPid(entry.path);
    if (pid === undefined || !isAlive(pid)) {
      actions.push({ entry, action: 'forget' });
      continue;
    }
    if (!looksLikeOurProcess(entry.kind, entry.offset, inspect(pid)?.command))
      continue;
    actions.push({ entry, action: 'kill', pid });
  }
  return actions;
}

// Both reapComposeStacks and reapPidfiles end the same way: name what
// they removed, and how long nothing live could have claimed it for.
// Shared so that sentence can't drift between the two. `detail` is the
// one thing that differs -- what a compose teardown also collects
// (containers, network, volume) that a pidfile kill has nothing
// equivalent to say.
function logReaped(
  action: string,
  noun: string,
  names: string[],
  detail?: string
): void {
  const minutes = Math.round(QUIET_MS / 60000);
  console.log(
    `e2e-stack-reap: ${action} ${names.length} orphaned e2e ${noun}(s) -- ${names.join(', ')}${detail ? ` -- ${detail}` : ''} (no worktree touched in the last ${minutes}m, no fresh heartbeat and no attached host process claims their port offset)`
  );
}

const DRY_RUN_FLAG = '--dry-run';
const DRY_RUN_PREFIX = 'e2e-stack-reap (dry run):';

// The compose stacks a real run would tear down, read through
// execEngine: the `--url $DOCKER_HOST` form first, then the engine's own
// default connection, so a login profile's DOCKER_HOST for another
// engine (#1687) no longer leaves this hook unable to see a single stack.
export function planStacks(
  live: ReadonlySet<number>,
  now: number,
  exec: (args: string[]) => string = execEngine
): StackProject[] {
  const psOutput = exec([
    'ps',
    '-a',
    '--filter',
    `label=${COMPOSE_PROJECT_LABEL}`,
    '--format',
    'json',
  ]);
  return pickReapProjects(
    groupStackProjects(parseContainers(psOutput)),
    live,
    now
  );
}

// Tears down every orphaned compose stack in `doomed`. Its own try/catch,
// separate from reapPidfiles' below, so a container engine that is down
// or unreachable -- which has nothing to do with whether a host-process
// pidfile is orphaned -- can never stop the pidfile sweep from running.
function reapComposeStacks(
  mainCheckout: string,
  live: ReadonlySet<number>,
  now: number,
  dryRun: boolean
): void {
  try {
    const doomed = planStacks(live, now);
    if (dryRun) {
      console.log(
        `${DRY_RUN_PREFIX} would tear down ${doomed.length} orphaned e2e stack(s)`
      );
      for (const project of doomed)
        console.log(
          `  ${project.project} (${project.containerIds.length} container(s), newest created ${new Date(project.newestCreatedAtMs).toISOString()})`
        );
      return;
    }
    if (doomed.length === 0) return;

    /* `compose down -v` rather than `rm -f` on the container ids: the
       containers are the smaller half of what a stack leaves behind.
       Scoped to the project name, `down -v` also removes the project's
       network (`<project>_default`) and its named `db-data` volume,
       which nothing else on the machine would ever collect -- confirmed
       against podman-compose 1.6.0 by starting the stack and reading
       `network ls`/`volume ls` back on either side of it. The compose
       file only names the services; the project name selects what goes. */
    const composeFile = path.join(mainCheckout, 'app', E2E_COMPOSE_FILE);
    const removed: string[] = [];
    for (const project of doomed) {
      try {
        execEngine([
          'compose',
          '-p',
          project.project,
          '-f',
          composeFile,
          'down',
          '-v',
          '-t',
          '2',
        ]);
        removed.push(project.project);
      } catch {
        // One project's teardown failing must not stop the others.
      }
    }
    if (removed.length === 0) return;

    logReaped('tore down', 'stack', removed, 'containers, network and volume');
  } catch {
    // Engine unreachable, binary missing, malformed output --
    // none of it may ever surface as a blocked or slowed SessionStart.
    if (dryRun)
      console.log(
        `${DRY_RUN_PREFIX} the container engine could not be read; no stack judged`
      );
    return;
  }
}

// Kills every orphaned host-process pidfile in os.tmpdir() (#1194). Needs
// no container engine at all, so it runs and fails open independently of
// reapComposeStacks above.
function reapPidfiles(
  live: ReadonlySet<number>,
  now: number,
  dryRun: boolean
): void {
  try {
    const actions = planPidfiles(
      pickReapPidfiles(discoverPidfiles(tmpdir()), live, now)
    );
    if (dryRun) {
      const kills = actions.filter((a) => a.action === 'kill');
      console.log(
        `${DRY_RUN_PREFIX} would kill ${kills.length} orphaned e2e host process(es) and forget ${actions.length - kills.length} stale pidfile(s)`
      );
      for (const { entry, action, pid } of actions)
        console.log(
          `  ${action} ${entry.kind}-${entry.offset}${pid === undefined ? '' : ` (pid ${pid})`}`
        );
      return;
    }
    if (actions.length === 0) return;

    const killed: string[] = [];
    for (const { entry, action, pid } of actions) {
      try {
        if (action === 'kill' && pid !== undefined) {
          process.kill(pid);
          killed.push(`${entry.kind}-${entry.offset}`);
        }
        fs.rmSync(entry.path, { force: true });
      } catch {
        // One pidfile's teardown failing must not stop the others.
      }
    }
    if (killed.length === 0) return;

    logReaped('killed', 'host process', killed);
  } catch {
    // Anything unexpected here (a tmpdir that can't be read, and so on)
    // -- never surfaced as a blocked or slowed SessionStart.
    return;
  }
}

/*
 * Fails open, always -- the same rule, for the same reason, as
 * testdb-reap.ts's own main(): this runs on SessionStart, decides nothing
 * about the tool call that follows, and exists purely to tidy up. A
 * reaper that errors must never block or slow a session from starting.
 * Do not "fix" this into failing closed.
 */
function main(argv: string[]): void {
  const dryRun = argv.includes(DRY_RUN_FLAG);
  let mainCheckout: string;
  let now: number;
  let live: Set<number>;
  try {
    mainCheckout = findMainCheckoutRoot(import.meta.dir);
    now = Date.now();
    live = liveOffsets(path.join(mainCheckout, '.claude', 'worktrees'), now);
  } catch {
    // git absent, or anything else -- nothing safe to decide without this.
    return;
  }

  if (dryRun)
    console.log(
      `${DRY_RUN_PREFIX} live port offsets: ${[...live].sort((a, b) => a - b).join(', ') || 'none'}`
    );
  reapComposeStacks(mainCheckout, live, now, dryRun);
  reapPidfiles(live, now, dryRun);
}

if (import.meta.main) main(process.argv.slice(2));
