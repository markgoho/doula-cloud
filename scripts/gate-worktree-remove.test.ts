/**
 * `.claude/hooks/gate-worktree-remove.ts` -- the PreToolUse gate on a
 * Bash `git worktree remove` (#1212). On 2026-09-10 an orchestrating
 * session short of port offsets ran `git worktree remove --force` on a
 * worktree that looked abandoned by every branch-and-tree signal, and
 * destroyed a live agent's uncommitted work. The pruner cannot guard that
 * path; only a gate on the command can.
 *
 * Each test builds a throwaway repo with one worktree, writes the owner
 * marker worktree-owner.ts would have written, and feeds the hook the
 * same stdin payload Claude Code sends. `CLAUDE_PID` in the hook's env is
 * "this session"; a `sleep` child stands in for another session's live
 * Claude Code process.
 */
import { afterAll, describe, expect, test } from 'bun:test';
import { execFileSync, spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

const REPO_ROOT = path.resolve(import.meta.dir, '..');
const HOOK = path.join(
  REPO_ROOT,
  '.claude',
  'hooks',
  'gate-worktree-remove.ts'
);

const otherSession = spawn('sleep', ['300'], { stdio: 'ignore' });
afterAll(() => {
  otherSession.kill();
});

function git(cwd: string, args: string[]): string {
  return execFileSync('git', ['-C', cwd, ...args], { encoding: 'utf8' }).trim();
}

function startedAt(pid: number): string {
  return execFileSync('ps', ['-p', String(pid), '-o', 'lstart='], {
    encoding: 'utf8',
  }).trim();
}

function makeFixture(): {
  root: string;
  worktree: string;
  cleanup: () => void;
} {
  const base = realpathSync(
    mkdtempSync(path.join(tmpdir(), 'gate-worktree-remove-test-'))
  );
  const root = path.join(base, 'repo');
  git(base, ['init', '-q', root]);
  git(root, ['config', 'user.email', 'test@example.com']);
  git(root, ['config', 'user.name', 'Test User']);
  writeFileSync(path.join(root, '.gitignore'), '.worktree-owner.json\n');
  git(root, ['add', '.gitignore']);
  git(root, ['commit', '-q', '-m', 'init']);
  const worktree = path.join(root, '.claude', 'worktrees', 'agent-x');
  git(root, ['worktree', 'add', '-q', worktree, '-b', 'agent-x']);
  return {
    root,
    worktree,
    cleanup: () => rmSync(base, { recursive: true, force: true }),
  };
}

function writeOwner(
  worktree: string,
  pid: number,
  processStartedAt: string
): void {
  writeFileSync(
    path.join(worktree, '.worktree-owner.json'),
    JSON.stringify({ pid, processStartedAt, sessionId: 'other-session' })
  );
}

function invoke(
  command: string,
  cwd: string
): Promise<{ exitCode: number; stdout: string }> {
  return new Promise((resolve, reject) => {
    const child = spawn('bun', [HOOK], {
      stdio: ['pipe', 'pipe', 'inherit'],
      env: { ...process.env, CLAUDE_PID: String(process.pid) },
    });
    let stdout = '';
    child.stdout.on('data', (chunk) => {
      stdout += chunk;
    });
    child.on('error', reject);
    child.on('close', (code) => resolve({ exitCode: code ?? 1, stdout }));
    child.stdin.write(
      JSON.stringify({ tool_name: 'Bash', tool_input: { command }, cwd })
    );
    child.stdin.end();
  });
}

const otherPid = () => otherSession.pid as number;

describe('gate-worktree-remove', () => {
  test('blocks removing a clean worktree another live session owns, --force or not', async () => {
    const { root, worktree, cleanup } = makeFixture();
    try {
      writeOwner(worktree, otherPid(), startedAt(otherPid()));
      for (const command of [
        `git worktree remove ${worktree}`,
        `git worktree remove --force ${worktree}`,
      ]) {
        const { exitCode, stdout } = await invoke(command, root);
        expect(exitCode).toBe(2);
        const reason = JSON.parse(stdout).reason as string;
        expect(reason).toContain(`pid ${otherPid()}`);
        expect(reason).toContain('ALLOW_LIVE_WORKTREE_REMOVE=1');
      }
    } finally {
      cleanup();
    }
  });

  test('resolves a relative path against the command cwd', async () => {
    const { root, worktree, cleanup } = makeFixture();
    try {
      writeOwner(worktree, otherPid(), startedAt(otherPid()));
      const { exitCode } = await invoke(
        'git worktree remove -f .claude/worktrees/agent-x',
        root
      );
      expect(exitCode).toBe(2);
    } finally {
      cleanup();
    }
  });

  test('allows this session to remove a worktree it owns', async () => {
    const { root, worktree, cleanup } = makeFixture();
    try {
      writeOwner(worktree, process.pid, startedAt(process.pid));
      const { exitCode } = await invoke(
        `git worktree remove ${worktree}`,
        root
      );
      expect(exitCode).toBe(0);
    } finally {
      cleanup();
    }
  });

  test('allows removing a clean worktree whose owner is gone', async () => {
    const { root, worktree, cleanup } = makeFixture();
    try {
      writeOwner(worktree, spawnSync('true').pid, 'Thu Jan  1 00:00:00 1970');
      const { exitCode } = await invoke(
        `git worktree remove --force ${worktree}`,
        root
      );
      expect(exitCode).toBe(0);
    } finally {
      cleanup();
    }
  });

  test('blocks --force on a worktree holding uncommitted changes, whoever owns it', async () => {
    const { root, worktree, cleanup } = makeFixture();
    try {
      writeOwner(worktree, spawnSync('true').pid, 'Thu Jan  1 00:00:00 1970');
      writeFileSync(path.join(worktree, 'work.txt'), 'uncommitted\n');
      const { exitCode, stdout } = await invoke(
        `git worktree remove --force ${worktree}`,
        root
      );
      expect(exitCode).toBe(2);
      expect(JSON.parse(stdout).reason).toContain('uncommitted changes');
    } finally {
      cleanup();
    }
  });

  test('leaves a dirty worktree without --force to git, which refuses it itself', async () => {
    const { root, worktree, cleanup } = makeFixture();
    try {
      writeFileSync(path.join(worktree, 'work.txt'), 'uncommitted\n');
      const { exitCode } = await invoke(
        `git worktree remove ${worktree}`,
        root
      );
      expect(exitCode).toBe(0);
    } finally {
      cleanup();
    }
  });

  test('the override lets a deliberate removal through', async () => {
    const { root, worktree, cleanup } = makeFixture();
    try {
      writeOwner(worktree, otherPid(), startedAt(otherPid()));
      writeFileSync(path.join(worktree, 'work.txt'), 'uncommitted\n');
      const { exitCode } = await invoke(
        `ALLOW_LIVE_WORKTREE_REMOVE=1 git worktree remove --force ${worktree}`,
        root
      );
      expect(exitCode).toBe(0);
    } finally {
      cleanup();
    }
  });

  test('ignores commands that remove no worktree', async () => {
    const { root, cleanup } = makeFixture();
    try {
      for (const command of [
        'git worktree list',
        'git status',
        'echo worktree remove',
      ]) {
        const { exitCode } = await invoke(command, root);
        expect(exitCode).toBe(0);
      }
    } finally {
      cleanup();
    }
  });
});
