#!/usr/bin/env bun
// Re-record a worktree's owner when a session enters it without
// provisioning (#1212). Registered on PostToolUse `EnterWorktree` (a
// worktree entered by `path`) and on `Stop` (the backstop for a plain
// `cd`, once per turn).
//
// provisionWorktree() claims a worktree when it is created, but a session
// that resumes one later -- `EnterWorktree path:`, or `cd` -- runs no
// provisioning. The marker then still names the session that died, the
// owner reads `gone`, and `gone` is the one state that lets the pruner
// reap a clean worktree with no commits of its own. This hands the claim
// to the session actually working there. claimWorktree() leaves another
// live owner's claim alone, so this can only ever take over a dead one.
//
// Only a worktree under `.claude/worktrees/` is claimed -- never the main
// checkout. Fails open, always: it decides nothing about the tool call or
// the turn, and a crash here must never block either.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { claimWorktree } from './worktree-owner.ts';

function worktreeRootOf(dir: string): string | null {
  try {
    const top = execFileSync(
      'git',
      ['-C', dir, 'rev-parse', '--show-toplevel'],
      {
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'ignore'],
      }
    ).trim();
    const managed = `${path.sep}.claude${path.sep}worktrees${path.sep}`;
    return top.includes(managed) ? top : null;
  } catch {
    return null;
  }
}

function main(): void {
  let payload: Record<string, unknown> = {};
  try {
    payload = JSON.parse(fs.readFileSync(0, 'utf8'));
  } catch {
    // no payload -- fall back to where the hook runs
  }
  const toolInput = payload['tool_input'] as
    Record<string, unknown> | undefined;
  const target =
    (typeof toolInput?.['path'] === 'string' && toolInput['path']) ||
    (typeof payload['cwd'] === 'string' && payload['cwd']) ||
    process.cwd();

  const root = worktreeRootOf(target);
  if (root) claimWorktree(root);
}

try {
  main();
} catch {
  // fail open -- see the header
}
