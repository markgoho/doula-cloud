/**
 * `.claude/hooks/worktree-claim.ts` -- re-records a worktree's owner when
 * a session enters one provisioning never ran for (#1212). Without it, a
 * worktree resumed with `EnterWorktree path:` or a plain `cd` keeps the
 * dead session's marker, reads as `gone`, and the pruner's ancestor test
 * would reap the new session's uncommitted work.
 */
import { describe, expect, test } from 'bun:test';
import { execFileSync, spawn, spawnSync } from 'node:child_process';
import {
  existsSync,
  mkdtempSync,
  readFileSync,
  realpathSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

const REPO_ROOT = path.resolve(import.meta.dir, '..');
const HOOK = path.join(REPO_ROOT, '.claude', 'hooks', 'worktree-claim.ts');
const OWNER_FILE = '.worktree-owner.json';

function git(cwd: string, args: string[]): string {
  return execFileSync('git', ['-C', cwd, ...args], { encoding: 'utf8' }).trim();
}

function makeFixture(): {
  root: string;
  worktree: string;
  cleanup: () => void;
} {
  const base = realpathSync(
    mkdtempSync(path.join(tmpdir(), 'worktree-claim-test-'))
  );
  const root = path.join(base, 'repo');
  git(base, ['init', '-q', root]);
  git(root, ['config', 'user.email', 'test@example.com']);
  git(root, ['config', 'user.name', 'Test User']);
  git(root, ['commit', '-q', '--allow-empty', '-m', 'init']);
  const worktree = path.join(root, '.claude', 'worktrees', 'resumed');
  git(root, ['worktree', 'add', '-q', worktree, '-b', 'resumed']);
  return {
    root,
    worktree,
    cleanup: () => rmSync(base, { recursive: true, force: true }),
  };
}

function invoke(payload: object, cwd: string): Promise<number> {
  return new Promise((resolve, reject) => {
    const child = spawn('bun', [HOOK], {
      cwd,
      stdio: ['pipe', 'ignore', 'inherit'],
      env: { ...process.env, CLAUDE_PID: String(process.pid) },
    });
    child.on('error', reject);
    child.on('close', (code) => resolve(code ?? 1));
    child.stdin.write(JSON.stringify(payload));
    child.stdin.end();
  });
}

function ownerPid(worktree: string): number {
  return JSON.parse(readFileSync(path.join(worktree, OWNER_FILE), 'utf8')).pid;
}

describe('worktree-claim', () => {
  test('a Stop in a worktree whose recorded owner is gone makes this session its owner', async () => {
    const { worktree, cleanup } = makeFixture();
    try {
      writeFileSync(
        path.join(worktree, OWNER_FILE),
        JSON.stringify({ pid: spawnSync('true').pid, processStartedAt: null })
      );
      const exitCode = await invoke(
        { hook_event_name: 'Stop', cwd: path.join(worktree) },
        worktree
      );
      expect(exitCode).toBe(0);
      expect(ownerPid(worktree)).toBe(process.pid);
    } finally {
      cleanup();
    }
  });

  test('EnterWorktree by path claims the worktree it names, from a subdirectory too', async () => {
    const { root, worktree, cleanup } = makeFixture();
    try {
      execFileSync('mkdir', ['-p', path.join(worktree, 'app')]);
      const exitCode = await invoke(
        {
          hook_event_name: 'PostToolUse',
          tool_name: 'EnterWorktree',
          tool_input: { path: path.join(worktree, 'app') },
          cwd: root,
        },
        root
      );
      expect(exitCode).toBe(0);
      expect(ownerPid(worktree)).toBe(process.pid);
    } finally {
      cleanup();
    }
  });

  test('never writes a marker into the main checkout', async () => {
    const { root, cleanup } = makeFixture();
    try {
      const exitCode = await invoke(
        { hook_event_name: 'Stop', cwd: root },
        root
      );
      expect(exitCode).toBe(0);
      expect(existsSync(path.join(root, OWNER_FILE))).toBe(false);
    } finally {
      cleanup();
    }
  });
});
