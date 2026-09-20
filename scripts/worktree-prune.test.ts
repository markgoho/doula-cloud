/**
 * `.claude/hooks/worktree-prune.ts` -- the cleanup pruner from
 * docs/agents/worktree-flow.md. Covers #1058: a registered worktree whose
 * directory is gone (`rm -rf` instead of `git worktree remove`) used to
 * crash the whole run with an unhandled `execFileSync` failure the moment
 * `runGit` tried `git -C <that path> rev-parse HEAD`, taking the report
 * for every other worktree down with it.
 *
 * The pruner resolves its own root via `findMainCheckoutRoot(import.meta.dir)`
 * (worktree-root.ts), which asks `git worktree list` from wherever the
 * running copy of the script physically sits -- so testing this for real
 * means giving it its own copy of the hook files inside a disposable
 * git repo, not importing functions against this repo's own worktrees
 * (several other Claude Code sessions may be using those at the same
 * time). Each test builds a fresh throwaway repo with its own bare
 * "origin" remote (so `origin/trunk` resolves and `sweepOrphanBranches`'s
 * unguarded `branch --merged origin/trunk` call has something to look at)
 * and a copy of every file in HOOK_FILES.
 */
import { describe, expect, test } from 'bun:test';
import { execFileSync, spawn, spawnSync } from 'node:child_process';
import {
  mkdtempSync,
  realpathSync,
  rmSync,
  utimesSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

const REPO_ROOT = path.resolve(import.meta.dir, '..');
const HOOK_FILES = [
  'worktree-prune.ts',
  'sync-trunk.ts',
  'worktree-root.ts',
  'worktree-owner.ts',
];

function git(cwd: string, args: string[]): string {
  return execFileSync('git', ['-C', cwd, ...args], { encoding: 'utf8' }).trim();
}

/** A disposable repo with its own origin remote and its own copy of the hook files. */
function makeFixture(): { root: string; cleanup: () => void } {
  const base = realpathSync(
    mkdtempSync(path.join(tmpdir(), 'worktree-prune-test-'))
  );
  const origin = path.join(base, 'origin.git');
  const root = path.join(base, 'fixture');
  git(base, ['init', '--bare', '-q', origin]);
  git(base, ['init', '-q', root]);
  git(root, ['config', 'user.email', 'test@example.com']);
  git(root, ['config', 'user.name', 'Test User']);
  git(root, ['symbolic-ref', 'HEAD', 'refs/heads/trunk']);
  execFileSync('sh', [
    '-c',
    `echo hello > ${JSON.stringify(path.join(root, 'README.md'))}`,
  ]);
  // The owner marker is ignored in the real repo; the fixture needs the
  // same, or the marker alone makes every owned worktree read as dirty.
  writeFileSync(path.join(root, '.gitignore'), '.worktree-owner.json\n');
  git(root, ['add', 'README.md', '.gitignore']);
  git(root, ['commit', '-q', '-m', 'init']);
  git(root, ['remote', 'add', 'origin', origin]);
  git(root, ['push', '-q', '-u', 'origin', 'trunk']);

  execFileSync('mkdir', ['-p', path.join(root, '.claude', 'hooks')]);
  for (const file of HOOK_FILES) {
    execFileSync('cp', [
      path.join(REPO_ROOT, '.claude', 'hooks', file),
      path.join(root, '.claude', 'hooks', file),
    ]);
  }

  return {
    root,
    cleanup: () => rmSync(base, { recursive: true, force: true }),
  };
}

function addWorktree(root: string, name: string, branch: string): string {
  const worktreePath = path.join(root, '.claude', 'worktrees', name);
  git(root, ['worktree', 'add', '-q', worktreePath, '-b', branch]);
  return worktreePath;
}

function makeStale(worktreePath: string): void {
  rmSync(worktreePath, { recursive: true, force: true });
}

/*
 * Inherits stderr (like scripts/testdb-reap.test.ts and
 * scripts/gate-bash-write.test.ts) so a genuine crash's real error text
 * surfaces in the test runner's own output instead of being swallowed --
 * the whole point of this fix is that a crash should never go
 * unexplained.
 */
function invoke(
  root: string,
  flag: '--dry-run' | '--merged'
): Promise<{ exitCode: number; stdout: string }> {
  return new Promise((resolve, reject) => {
    const child = spawn(
      'bun',
      [path.join(root, '.claude', 'hooks', 'worktree-prune.ts'), flag],
      {
        cwd: root,
        stdio: ['ignore', 'pipe', 'inherit'],
      }
    );
    let stdout = '';
    child.stdout.on('data', (chunk) => {
      stdout += chunk;
    });
    child.on('error', reject);
    child.on('close', (code) => resolve({ exitCode: code ?? 1, stdout }));
  });
}

describe('worktree-prune (stale registration, #1058)', () => {
  test('--dry-run reports a stale worktree instead of crashing, and still reports a healthy one', async () => {
    const { root, cleanup } = makeFixture();
    try {
      const healthy = addWorktree(root, 'healthy-wt', 'healthy-branch');
      const stale = addWorktree(root, 'stale-wt', 'stale-branch');
      makeStale(stale);

      const { exitCode, stdout } = await invoke(root, '--dry-run');

      expect(exitCode).toBe(0);
      expect(stdout).toContain(`${stale}  branch=stale-branch  STALE`);
      expect(stdout).toContain(healthy);
      expect(stdout).not.toContain('execFileSync');
    } finally {
      cleanup();
    }
  });

  test('--merged clears the stale registration without the caller running `git worktree prune` by hand', async () => {
    const { root, cleanup } = makeFixture();
    try {
      const stale = addWorktree(root, 'stale-wt', 'stale-branch');
      makeStale(stale);

      const { exitCode, stdout } = await invoke(root, '--merged');

      expect(exitCode).toBe(0);
      expect(stdout).toContain(`${stale}  branch=stale-branch  STALE`);

      const remaining = git(root, ['worktree', 'list', '--porcelain']);
      expect(remaining).not.toContain(stale);
    } finally {
      cleanup();
    }
  });

  test('a worktree whose directory exists is never treated as stale', async () => {
    const { root, cleanup } = makeFixture();
    try {
      const healthy = addWorktree(root, 'healthy-wt', 'healthy-branch');

      const { exitCode, stdout } = await invoke(root, '--dry-run');

      expect(exitCode).toBe(0);
      expect(stdout).toContain(healthy);
      expect(stdout).not.toContain('STALE');
    } finally {
      cleanup();
    }
  });
});

/*
 * #1212: a worktree whose agent is mid-run and has not reached its first
 * commit is clean, has no commits of its own, and -- if the agent spent
 * the last half hour editing files below the worktree root, reading, or
 * watching CI -- has none of `recentlyTouched`'s three mtimes moving. By
 * every branch-and-tree signal it is an abandoned spawn, and `--merged`
 * used to remove it: a branch with no commits sits at trunk's tip, so
 * `git branch --merged origin/trunk` listed it as landed.
 *
 * The only signal that tells the two apart is whether the Claude Code
 * process that provisioned the worktree is still running, which
 * worktree-owner.ts records in `.worktree-owner.json`.
 */

/** Backdates every mtime `recentlyTouched` reads, so the 30-minute quiet window has passed. */
function makeQuiet(worktreePath: string): void {
  const old = new Date(Date.now() - 2 * 60 * 60 * 1000);
  const gitDir = git(worktreePath, ['rev-parse', '--absolute-git-dir']);
  for (const target of [worktreePath, gitDir, path.join(gitDir, 'index')]) {
    utimesSync(target, old, old);
  }
}

function startedAt(pid: number): string {
  return execFileSync('ps', ['-p', String(pid), '-o', 'lstart='], {
    encoding: 'utf8',
  }).trim();
}

/** A pid that existed a moment ago and has exited since. */
function deadPid(): number {
  return spawnSync('true').pid;
}

const LONG_AGO = 'Thu Jan  1 00:00:00 1970';

function writeOwner(
  worktreePath: string,
  pid: number,
  processStartedAt: string
): void {
  writeFileSync(
    path.join(worktreePath, '.worktree-owner.json'),
    JSON.stringify({ pid, processStartedAt, sessionId: 'test-session' })
  );
}

function isRegistered(root: string, worktreePath: string): boolean {
  return git(root, ['worktree', 'list', '--porcelain']).includes(worktreePath);
}

describe('worktree-prune (a live agent with no commits yet, #1212)', () => {
  test('a clean, quiet worktree with no commits and no owner record is no longer removed on "no commits of its own" alone', async () => {
    const { root, cleanup } = makeFixture();
    try {
      const agent = addWorktree(root, 'agent-live', 'agent-live');
      makeQuiet(agent);

      const { exitCode, stdout } = await invoke(root, '--merged');

      expect(exitCode).toBe(0);
      expect(isRegistered(root, agent)).toBe(true);
      expect(stdout).toContain(`skip (not merged into trunk): ${agent}`);
    } finally {
      cleanup();
    }
  });

  test('a worktree whose owning process is still running is never removed, and the report names the owner', async () => {
    const { root, cleanup } = makeFixture();
    try {
      const agent = addWorktree(root, 'agent-live', 'agent-live');
      writeOwner(agent, process.pid, startedAt(process.pid));
      makeQuiet(agent);

      const { exitCode, stdout } = await invoke(root, '--merged');

      expect(exitCode).toBe(0);
      expect(isRegistered(root, agent)).toBe(true);
      expect(stdout).toContain(
        `skip (owner alive: pid ${process.pid}, session test-session): ${agent}`
      );
    } finally {
      cleanup();
    }
  });

  test('a recycled pid (running, but started at a different time) reads as the owner being gone', async () => {
    const { root, cleanup } = makeFixture();
    try {
      const agent = addWorktree(root, 'agent-gone', 'agent-gone');
      writeOwner(agent, process.pid, LONG_AGO);
      makeQuiet(agent);

      const { exitCode, stdout } = await invoke(root, '--merged');

      expect(exitCode).toBe(0);
      expect(isRegistered(root, agent)).toBe(false);
      expect(stdout).toContain(`removed: ${agent}`);
    } finally {
      cleanup();
    }
  });

  test('a clean worktree with no commits whose owner is provably gone is reclaimed -- nothing on it can be lost', async () => {
    const { root, cleanup } = makeFixture();
    try {
      const agent = addWorktree(root, 'agent-gone', 'agent-gone');
      writeOwner(agent, deadPid(), LONG_AGO);
      makeQuiet(agent);

      const { exitCode, stdout } = await invoke(root, '--merged');

      expect(exitCode).toBe(0);
      expect(isRegistered(root, agent)).toBe(false);
      expect(stdout).toContain(`removed: ${agent}`);
    } finally {
      cleanup();
    }
  });

  test('a dirty worktree is never removed, even when its owner is gone', async () => {
    const { root, cleanup } = makeFixture();
    try {
      const agent = addWorktree(root, 'agent-dirty', 'agent-dirty');
      writeOwner(agent, deadPid(), LONG_AGO);
      writeFileSync(path.join(agent, 'work.txt'), 'uncommitted\n');
      makeQuiet(agent);

      const { exitCode, stdout } = await invoke(root, '--merged');

      expect(exitCode).toBe(0);
      expect(isRegistered(root, agent)).toBe(true);
      expect(stdout).toContain(`skip (dirty): ${agent}`);
    } finally {
      cleanup();
    }
  });

  test('--dry-run shows each worktree owner, so a session hunting for slots can tell live from abandoned', async () => {
    const { root, cleanup } = makeFixture();
    try {
      const live = addWorktree(root, 'agent-live', 'agent-live');
      writeOwner(live, process.pid, startedAt(process.pid));
      const gone = addWorktree(root, 'agent-gone', 'agent-gone');
      writeOwner(gone, deadPid(), LONG_AGO);
      const unknown = addWorktree(root, 'agent-unknown', 'agent-unknown');

      const { exitCode, stdout } = await invoke(root, '--dry-run');

      expect(exitCode).toBe(0);
      const lineFor = (wt: string) =>
        stdout.split('\n').find((line) => line.startsWith(`${wt}  `)) ?? '';
      expect(lineFor(live)).toContain(
        `owner=alive(pid ${process.pid}, session test-session)`
      );
      expect(lineFor(gone)).toContain('owner=gone(pid ');
      expect(lineFor(unknown)).toContain('owner=unknown');
    } finally {
      cleanup();
    }
  });
});
