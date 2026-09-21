/**
 * `.claude/hooks/worktree-owner.ts` -- the owner record provisioning writes
 * into every worktree (#1212). The pruner and gate-worktree-remove specs
 * cover how the record is read; this covers how it is written.
 */
import { describe, expect, test } from 'bun:test';
import { execFileSync, spawn } from 'node:child_process';
import { mkdtempSync, readFileSync, realpathSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import {
  claimWorktree,
  OWNER_FILE,
  ownerStatus,
} from '../.claude/hooks/worktree-owner.ts';

function git(cwd: string, args: string[]): string {
  return execFileSync('git', ['-C', cwd, ...args], { encoding: 'utf8' }).trim();
}

/** A repo whose branch does NOT ignore the marker -- a branch cut before #1212. */
function makeWorktree(): { worktree: string; cleanup: () => void } {
  const base = realpathSync(
    mkdtempSync(path.join(tmpdir(), 'worktree-owner-test-'))
  );
  const root = path.join(base, 'repo');
  git(base, ['init', '-q', root]);
  git(root, ['config', 'user.email', 'test@example.com']);
  git(root, ['config', 'user.name', 'Test User']);
  git(root, ['commit', '-q', '--allow-empty', '-m', 'init']);
  const worktree = path.join(root, '.claude', 'worktrees', 'agent-x');
  git(root, ['worktree', 'add', '-q', worktree, '-b', 'agent-x']);
  return {
    worktree,
    cleanup: () => rmSync(base, { recursive: true, force: true }),
  };
}

describe('claimWorktree', () => {
  test('records the Claude Code process, which then reads as alive', () => {
    const { worktree, cleanup } = makeWorktree();
    try {
      const message = claimWorktree(worktree, {
        CLAUDE_PID: String(process.pid),
        CLAUDE_CODE_SESSION_ID: 'session-a',
      });

      expect(message).toBe(`recorded owner pid ${process.pid}`);
      const record = JSON.parse(
        readFileSync(path.join(worktree, OWNER_FILE), 'utf8')
      );
      expect(record.pid).toBe(process.pid);
      expect(record.sessionId).toBe('session-a');
      expect(ownerStatus(worktree).state).toBe('alive');
    } finally {
      cleanup();
    }
  });

  test('the marker never reads as an uncommitted change, even on a branch whose .gitignore predates it', () => {
    const { worktree, cleanup } = makeWorktree();
    try {
      claimWorktree(worktree, { CLAUDE_PID: String(process.pid) });
      claimWorktree(worktree, { CLAUDE_PID: String(process.pid) });

      expect(git(worktree, ['status', '--porcelain'])).toBe('');
      const exclude = readFileSync(
        path.join(
          git(worktree, [
            'rev-parse',
            '--path-format=absolute',
            '--git-common-dir',
          ]),
          'info',
          'exclude'
        ),
        'utf8'
      );
      expect(
        exclude.split('\n').filter((line) => line === OWNER_FILE)
      ).toHaveLength(1);
    } finally {
      cleanup();
    }
  });

  test('leaves another live session its claim', async () => {
    const { worktree, cleanup } = makeWorktree();
    const other = spawn('sleep', ['30'], { stdio: 'ignore' });
    try {
      // Let `sleep` exec, so `ps` reads the start time the recheck will compare.
      await new Promise((resolve) => setTimeout(resolve, 100));
      claimWorktree(worktree, { CLAUDE_PID: String(other.pid) });

      const message = claimWorktree(worktree, {
        CLAUDE_PID: String(process.pid),
      });

      expect(message).toContain(`left existing owner (alive(pid ${other.pid}`);
      const record = JSON.parse(
        readFileSync(path.join(worktree, OWNER_FILE), 'utf8')
      );
      expect(record.pid).toBe(other.pid);
    } finally {
      other.kill();
      cleanup();
    }
  });

  test('takes over a claim whose process is gone', async () => {
    const { worktree, cleanup } = makeWorktree();
    const other = spawn('sleep', ['30'], { stdio: 'ignore' });
    try {
      await new Promise((resolve) => setTimeout(resolve, 100));
      claimWorktree(worktree, { CLAUDE_PID: String(other.pid) });
      other.kill();
      await new Promise((resolve) => other.on('exit', resolve));

      const message = claimWorktree(worktree, {
        CLAUDE_PID: String(process.pid),
      });

      expect(message).toBe(`recorded owner pid ${process.pid}`);
    } finally {
      cleanup();
    }
  });

  test('an unreadable record is unknown -- neither proof of life nor of death', () => {
    const { worktree, cleanup } = makeWorktree();
    try {
      execFileSync('sh', [
        '-c',
        `echo not-json > ${JSON.stringify(path.join(worktree, OWNER_FILE))}`,
      ]);
      expect(ownerStatus(worktree)).toEqual({
        state: 'unknown',
        reason: 'unreadable owner record',
      });
      rmSync(path.join(worktree, OWNER_FILE));
      expect(ownerStatus(worktree)).toEqual({
        state: 'unknown',
        reason: 'no owner record',
      });
    } finally {
      cleanup();
    }
  });
});
