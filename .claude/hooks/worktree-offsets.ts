// Which worktree owns which port offset.
//
// A worktree's `.port-offset` file, written by worktree-provision.ts, is
// the whole of its identity on this machine: every port the local stack
// binds is shifted by it (app/e2e/ports.ts), the e2e stack's compose
// project is named for it, and the host-process pidfiles are suffixed
// with it. Two hooks have to read that mapping and would answer
// differently if each kept its own scan -- worktree-provision.ts, to
// refuse an offset another worktree already claims, and e2e-stack-reap.ts,
// to decide whether a running compose stack still has a session behind it.
//
// Deliberately side-effect-free, and it shells out to nothing: both
// callers are hooks that must fail open, and a module that ran a git
// command at import time would throw before either one's own try/catch
// could catch it.
import fs from 'node:fs';
import path from 'node:path';

export interface WorktreeOffset {
  /** Absolute path to the worktree directory. */
  path: string;
  offset: number;
}

// Every worktree under `worktreesRoot` that has an offset assigned. A
// directory with no readable, integral `.port-offset` claims nothing --
// it has not been provisioned yet -- and is left out rather than reported
// with a made-up offset.
export function readWorktreeOffsets(worktreesRoot: string): WorktreeOffset[] {
  let entries: string[];
  try {
    entries = fs.readdirSync(worktreesRoot);
  } catch {
    return []; // no worktrees directory at all -- nothing claims anything
  }

  const found: WorktreeOffset[] = [];
  for (const entry of entries) {
    const worktreePath = path.join(worktreesRoot, entry);
    let offset: number;
    try {
      offset = Number.parseInt(
        fs.readFileSync(path.join(worktreePath, '.port-offset'), 'utf8').trim(),
        10
      );
    } catch {
      continue;
    }
    if (Number.isInteger(offset)) found.push({ path: worktreePath, offset });
  }
  return found;
}
