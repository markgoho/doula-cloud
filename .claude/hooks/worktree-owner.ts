// Who owns a worktree, and are they still running? (#1212)
//
// A worktree whose agent is mid-run and has not reached its first commit
// is clean, has no commits of its own, and may have touched no mtime the
// pruner watches for half an hour (reading, reviewing, a CI watch). By
// every branch-and-tree signal it is an abandoned spawn -- and that is
// exactly how two agents' uncommitted work was destroyed on 2026-09-10,
// once by a session removing it by hand and once by something else.
//
// The signal that tells the two apart is whether the Claude Code process
// that provisioned the worktree is still running. Subagents run inside
// their session's own process, so "the process is alive" covers an
// isolated subagent too. provisionWorktree() records that process in
// `<worktree>/.worktree-owner.json` (gitignored, like `.port-offset`):
//
//   {"pid":28479,"processStartedAt":"Sun Sep 20 16:32:22 2026",
//    "sessionId":"eabdbfe2-...","claimedAt":"2026-09-20T20:35:01.000Z"}
//
// and anything about to remove a worktree reads it back:
//
//   alive    -- the pid exists AND `ps -o lstart=` still reports the start
//               time recorded at claim. Never removed.
//   gone     -- the pid is gone, or now belongs to a process that started
//               at a different time (the OS recycled it). Nothing is
//               running in the worktree on its owner's behalf.
//   unknown  -- no marker (a worktree made before #1212, or by hand
//               outside Claude Code) or one that cannot be read. Neither
//               proof of life nor proof of death, so a caller must not
//               treat it as either.
//
// No heartbeat, deliberately: mtime is exactly the signal that failed, and
// the OS answers "is this pid running" directly. From a shell:
//
//   cat <worktree>/.worktree-owner.json
//   ps -o lstart= -p <pid>   # alive only if it prints the recorded time
//
// or, for every worktree at once, `bun .claude/hooks/worktree-prune.ts
// --dry-run` (its `owner=` column). docs/agents/worktree-flow.md has the
// rule this serves.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

export const OWNER_FILE = '.worktree-owner.json';

export interface WorktreeOwner {
  pid: number;
  processStartedAt: string | null;
  sessionId: string | null;
}

export type OwnerStatus =
  | { state: 'alive'; owner: WorktreeOwner }
  | { state: 'gone'; owner: WorktreeOwner }
  | { state: 'unknown'; reason: string };

// ESRCH is the only proof a pid is gone. EPERM means it exists and
// belongs to another user -- alive, and the side that removes nothing.
export function isProcessAlive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch (error) {
    return (error as NodeJS.ErrnoException).code !== 'ESRCH';
  }
}

function ps(pid: number, field: string): string | undefined {
  try {
    const out = execFileSync('ps', ['-p', String(pid), '-o', `${field}=`], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim();
    return out || undefined;
  } catch {
    return undefined;
  }
}

export function processStartedAt(pid: number): string | undefined {
  return ps(pid, 'lstart');
}

/*
 * The Claude Code process this hook is running under. Claude Code exports
 * CLAUDE_PID to what it spawns; when that is missing, walk up the process
 * tree to the nearest `claude`. Returns null outside Claude Code (a person
 * running `git worktree add` in a terminal), which leaves the worktree
 * `unknown` rather than claiming it for a shell that exits in a second.
 */
export function findClaudePid(
  env: NodeJS.ProcessEnv = process.env
): number | null {
  const fromEnv = Number(env.CLAUDE_PID);
  if (Number.isInteger(fromEnv) && fromEnv > 0 && isProcessAlive(fromEnv)) {
    return fromEnv;
  }
  let pid = process.ppid;
  for (let hop = 0; hop < 10 && pid > 1; hop++) {
    const comm = ps(pid, 'comm');
    if (comm && path.basename(comm) === 'claude') return pid;
    const parent = Number(ps(pid, 'ppid'));
    if (!Number.isInteger(parent)) break;
    pid = parent;
  }
  return null;
}

export function readOwner(worktreePath: string): WorktreeOwner | null {
  const raw = fs.readFileSync(path.join(worktreePath, OWNER_FILE), 'utf8');
  const parsed = JSON.parse(raw) as Partial<WorktreeOwner>;
  if (!Number.isInteger(parsed.pid) || (parsed.pid as number) <= 0) return null;
  return {
    pid: parsed.pid as number,
    processStartedAt:
      typeof parsed.processStartedAt === 'string'
        ? parsed.processStartedAt
        : null,
    sessionId: typeof parsed.sessionId === 'string' ? parsed.sessionId : null,
  };
}

export function ownerStatus(worktreePath: string): OwnerStatus {
  let owner: WorktreeOwner | null;
  try {
    owner = readOwner(worktreePath);
  } catch (error) {
    const missing = (error as NodeJS.ErrnoException).code === 'ENOENT';
    return {
      state: 'unknown',
      reason: missing ? 'no owner record' : 'unreadable owner record',
    };
  }
  if (!owner) return { state: 'unknown', reason: 'unreadable owner record' };
  if (!isProcessAlive(owner.pid)) return { state: 'gone', owner };

  /* A live pid with a different start time is the OS's recycling, not
     the owner. One whose start time cannot be read is given the benefit
     of the doubt: "alive" is the answer that removes nothing. */
  const started = processStartedAt(owner.pid);
  if (owner.processStartedAt && started && started !== owner.processStartedAt) {
    return { state: 'gone', owner };
  }
  return { state: 'alive', owner };
}

export function describeOwner(status: OwnerStatus): string {
  if (status.state === 'unknown') return 'unknown';
  return `${status.state}(pid ${status.owner.pid}${status.owner.sessionId ? `, session ${status.owner.sessionId}` : ''})`;
}

/*
 * Record the running Claude Code process as this worktree's owner. A
 * marker naming another process that is still alive is left alone -- that
 * session is still working here, and taking its claim would let the
 * pruner remove the worktree the moment this one exits.
 */
export function claimWorktree(
  worktreePath: string,
  env: NodeJS.ProcessEnv = process.env
): string {
  const pid = findClaudePid(env);
  if (pid === null)
    return 'no Claude Code process found -- owner left unrecorded';

  const current = ownerStatus(worktreePath);
  if (current.state === 'alive' && current.owner.pid !== pid) {
    return `left existing owner (${describeOwner(current)})`;
  }

  const record = {
    pid,
    processStartedAt: processStartedAt(pid) ?? null,
    sessionId: env.CLAUDE_CODE_SESSION_ID || null,
    claimedAt: new Date().toISOString(),
  };
  excludeOwnerFile(worktreePath);
  fs.writeFileSync(
    path.join(worktreePath, OWNER_FILE),
    `${JSON.stringify(record)}\n`
  );
  return `recorded owner pid ${pid}`;
}

/*
 * `.gitignore` lists the marker, but only on branches cut after #1212. On
 * an older branch an unignored marker would read as an uncommitted change
 * -- the worktree could then never be pruned, and plain `git worktree
 * remove` would refuse it. The shared `info/exclude` covers every worktree
 * of this checkout whatever its branch holds.
 */
function excludeOwnerFile(worktreePath: string): void {
  const commonDir = path.resolve(
    worktreePath,
    execFileSync('git', ['-C', worktreePath, 'rev-parse', '--git-common-dir'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim()
  );
  const exclude = path.join(commonDir, 'info', 'exclude');
  const current = fs.existsSync(exclude)
    ? fs.readFileSync(exclude, 'utf8')
    : '';
  if (current.split('\n').includes(OWNER_FILE)) return;
  fs.mkdirSync(path.dirname(exclude), { recursive: true });
  const separator = current === '' || current.endsWith('\n') ? '' : '\n';
  fs.appendFileSync(exclude, `${separator}${OWNER_FILE}\n`);
}
