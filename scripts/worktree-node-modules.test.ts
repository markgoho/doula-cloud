/**
 * Sibling SvelteKit package detection in
 * `.claude/hooks/worktree-provision.ts` (#950).
 *
 * Before this, `app/` was the only sibling package the hook knew about:
 * `dependencyManifestsChanged` hardcoded its `package.json`/`bun.lock`
 * paths, and `ensureNodeModulesReal` (formerly `ensureAppNodeModulesReal`)
 * targeted `app/node_modules` by name. `gcp-dashboard/` -- a second
 * package on the same `@sveltejs/kit: "next"` line -- got no `bun install`
 * at all in a fresh worktree.
 *
 * `sveltekitPackages` detects the package list by scanning for a
 * `package.json` that declares `@sveltejs/kit`, rather than a
 * hand-maintained constant, so a third sibling package needs no update
 * here. One test below runs it against this repo's own root to prove the
 * detection actually finds `app` and `gcp-dashboard` -- the same role
 * `worktree-port-offset.test.ts`'s "the real probe" section plays.
 */
import { describe, expect, test } from 'bun:test';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {
  dependencyManifestPaths,
  dependencyManifestsChanged,
  ensureNodeModulesReal,
  sveltekitPackages,
  unlinkIfSymlink,
} from '../.claude/hooks/worktree-provision.ts';

const REPO_ROOT = path.resolve(import.meta.dir, '..');

function makeFixtureRoot(
  dirs: Record<string, { packageJson?: unknown; raw?: string } | null>
): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'worktree-node-modules-'));
  for (const [name, contents] of Object.entries(dirs)) {
    const dir = path.join(root, name);
    fs.mkdirSync(dir, { recursive: true });
    if (contents === null) continue; // directory with no package.json
    const pkgPath = path.join(dir, 'package.json');
    if (contents.raw !== undefined) {
      fs.writeFileSync(pkgPath, contents.raw);
    } else {
      fs.writeFileSync(pkgPath, JSON.stringify(contents.packageJson));
    }
  }
  return root;
}

describe('sveltekitPackages', () => {
  test('finds a package declaring @sveltejs/kit in dependencies', () => {
    const root = makeFixtureRoot({
      app: { packageJson: { dependencies: { '@sveltejs/kit': 'next' } } },
    });
    expect(sveltekitPackages(root)).toEqual(['app']);
  });

  test('finds a package declaring @sveltejs/kit in devDependencies', () => {
    const root = makeFixtureRoot({
      'gcp-dashboard': {
        packageJson: { devDependencies: { '@sveltejs/kit': 'next' } },
      },
    });
    expect(sveltekitPackages(root)).toEqual(['gcp-dashboard']);
  });

  test('finds every matching sibling, sorted', () => {
    const root = makeFixtureRoot({
      'gcp-dashboard': {
        packageJson: { devDependencies: { '@sveltejs/kit': 'next' } },
      },
      app: { packageJson: { dependencies: { '@sveltejs/kit': 'next' } } },
    });
    expect(sveltekitPackages(root)).toEqual(['app', 'gcp-dashboard']);
  });

  test('skips a package with no @sveltejs/kit dependency', () => {
    const root = makeFixtureRoot({
      hugo: { packageJson: { dependencies: { sass: '^1' } } },
    });
    expect(sveltekitPackages(root)).toEqual([]);
  });

  test('skips a directory with no package.json', () => {
    const root = makeFixtureRoot({ docs: null });
    expect(sveltekitPackages(root)).toEqual([]);
  });

  test('skips a directory with unparseable package.json', () => {
    const root = makeFixtureRoot({ broken: { raw: '{ not json' } });
    expect(sveltekitPackages(root)).toEqual([]);
  });

  test('skips dotdirs and node_modules', () => {
    const root = makeFixtureRoot({
      '.claude': { packageJson: { dependencies: { '@sveltejs/kit': 'next' } } },
      node_modules: {
        packageJson: { dependencies: { '@sveltejs/kit': 'next' } },
      },
    });
    expect(sveltekitPackages(root)).toEqual([]);
  });

  test("finds app and gcp-dashboard on this repo's own root", () => {
    // The drift guard: if a future package renames @sveltejs/kit out of
    // its manifest, or a package.json goes missing, this fails here
    // rather than silently in a real worktree.
    expect(sveltekitPackages(REPO_ROOT)).toEqual(['app', 'gcp-dashboard']);
  });
});

describe('dependencyManifestPaths', () => {
  test('always includes the root manifest pair', () => {
    expect(dependencyManifestPaths([])).toEqual(['package.json', 'bun.lock']);
  });

  test("adds each package's own manifest pair", () => {
    expect(dependencyManifestPaths(['app', 'gcp-dashboard'])).toEqual([
      'package.json',
      'bun.lock',
      'app/package.json',
      'app/bun.lock',
      'gcp-dashboard/package.json',
      'gcp-dashboard/bun.lock',
    ]);
  });
});

describe('ensureNodeModulesReal', () => {
  // A stub in place of the real installReal (which shells out to `bun
  // install`), so these tests cover the leave-alone-vs-reinstall decision
  // without actually running an install.
  type Install = (
    worktreePath: string,
    subdir: string,
    reason: string,
    messages: string[]
  ) => void;

  function recordingInstall(): {
    install: Install;
    calls: { subdir: string; reason: string }[];
  } {
    const calls: { subdir: string; reason: string }[] = [];
    const install = (
      _worktreePath: string,
      subdir: string,
      reason: string,
      _messages: string[]
    ): void => {
      calls.push({ subdir, reason });
    };
    return { install, calls };
  }

  test('leaves an existing real, unchanged install alone', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'ensure-real-'));
    fs.mkdirSync(path.join(root, 'gcp-dashboard', 'node_modules'), {
      recursive: true,
    });
    const messages: string[] = [];
    const { install, calls } = recordingInstall();

    ensureNodeModulesReal(root, 'gcp-dashboard', false, messages, install);

    expect(calls).toEqual([]);
    expect(messages).toEqual([
      'left existing gcp-dashboard/node_modules (real install)',
    ]);
  });

  test('installs when node_modules does not exist yet', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'ensure-missing-'));
    fs.mkdirSync(path.join(root, 'gcp-dashboard'), { recursive: true });
    const messages: string[] = [];
    const { install, calls } = recordingInstall();

    ensureNodeModulesReal(root, 'gcp-dashboard', false, messages, install);

    expect(calls).toEqual([
      { subdir: 'gcp-dashboard', reason: 'always real, see comment' },
    ]);
  });

  test('reinstalls when the existing node_modules is a symlink, even unchanged', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'ensure-symlink-'));
    const pkgDir = path.join(root, 'gcp-dashboard');
    fs.mkdirSync(pkgDir, { recursive: true });
    const elsewhere = fs.mkdtempSync(
      path.join(os.tmpdir(), 'ensure-elsewhere-')
    );
    fs.symlinkSync(elsewhere, path.join(pkgDir, 'node_modules'), 'junction');
    const messages: string[] = [];
    const { install, calls } = recordingInstall();

    ensureNodeModulesReal(root, 'gcp-dashboard', false, messages, install);

    expect(calls).toEqual([
      { subdir: 'gcp-dashboard', reason: 'always real, see comment' },
    ]);
  });

  test('reinstalls when the manifest changed, even over a real existing install', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'ensure-changed-'));
    fs.mkdirSync(path.join(root, 'gcp-dashboard', 'node_modules'), {
      recursive: true,
    });
    const messages: string[] = [];
    const { install, calls } = recordingInstall();

    ensureNodeModulesReal(root, 'gcp-dashboard', true, messages, install);

    expect(calls).toEqual([
      { subdir: 'gcp-dashboard', reason: 'dependency manifest changed' },
    ]);
  });
});

describe('dependencyManifestsChanged', () => {
  function makeGitFixture(): string {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'dependency-manifest-'));
    const git = (...args: string[]) =>
      execFileSync('git', args, { cwd: root, encoding: 'utf8' });
    git('init', '-q', '-b', 'trunk');
    git('config', 'user.email', 'test@example.com');
    git('config', 'user.name', 'Test');
    fs.mkdirSync(path.join(root, 'gcp-dashboard'), { recursive: true });
    fs.writeFileSync(path.join(root, 'package.json'), '{}\n');
    fs.writeFileSync(path.join(root, 'README.md'), 'trunk\n');
    fs.writeFileSync(path.join(root, 'gcp-dashboard', 'package.json'), '{}\n');
    git('add', '-A');
    git('commit', '-q', '-m', 'trunk');
    git('checkout', '-q', '-b', 'feature');
    return root;
  }

  test('false when the branch touches nothing tracked', () => {
    const root = makeGitFixture();
    expect(dependencyManifestsChanged(root, ['gcp-dashboard'])).toBe(false);
  });

  test('false when the branch only touches an unrelated file', () => {
    const root = makeGitFixture();
    fs.writeFileSync(path.join(root, 'README.md'), 'feature\n');
    execFileSync('git', ['commit', '-aqm', 'unrelated'], {
      cwd: root,
      encoding: 'utf8',
    });
    expect(dependencyManifestsChanged(root, ['gcp-dashboard'])).toBe(false);
  });

  test("true when the branch touches the named package's package.json", () => {
    const root = makeGitFixture();
    fs.writeFileSync(
      path.join(root, 'gcp-dashboard', 'package.json'),
      '{"name":"gcp-dashboard"}\n'
    );
    execFileSync('git', ['commit', '-aqm', 'bump gcp-dashboard'], {
      cwd: root,
      encoding: 'utf8',
    });
    expect(dependencyManifestsChanged(root, ['gcp-dashboard'])).toBe(true);
  });

  test('false for a package.json change when that package is not in the list', () => {
    const root = makeGitFixture();
    fs.writeFileSync(
      path.join(root, 'gcp-dashboard', 'package.json'),
      '{"name":"gcp-dashboard"}\n'
    );
    execFileSync('git', ['commit', '-aqm', 'bump gcp-dashboard'], {
      cwd: root,
      encoding: 'utf8',
    });
    expect(dependencyManifestsChanged(root, [])).toBe(false);
  });
});

describe('unlinkIfSymlink', () => {
  test('removes a symlink target', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'unlink-symlink-'));
    const real = path.join(root, 'real');
    fs.mkdirSync(real);
    const link = path.join(root, 'link');
    fs.symlinkSync(real, link, 'junction');

    unlinkIfSymlink(link);

    expect(fs.existsSync(link)).toBe(false);
    expect(fs.existsSync(real)).toBe(true); // the target is untouched
  });

  test('leaves a real directory alone', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'unlink-real-'));
    const real = path.join(root, 'real');
    fs.mkdirSync(real);

    unlinkIfSymlink(real);

    expect(fs.existsSync(real)).toBe(true);
  });

  test('is a no-op when the target does not exist', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'unlink-missing-'));
    expect(() => unlinkIfSymlink(path.join(root, 'missing'))).not.toThrow();
  });
});
