/**
 * `.claude/hooks/gate-golangci-lint-cache.sh` -- the PreToolUse gate on
 * a Bash `golangci-lint run`. It refuses two things: a run without a
 * worktree-scoped GOLANGCI_LINT_CACHE (#587), and a run by a local
 * golangci-lint that is not the version ci.yml pins (#1409). The second
 * exists because Homebrew upgrades golangci-lint on its own schedule: on
 * 2026-09-20 this machine ran 2.13.2 against CI's v2.12.2, and a local
 * lint reported findings CI did not, so no local result was evidence of
 * what CI would say.
 *
 * Each test builds a throwaway repo holding a ci.yml, puts a fake
 * `golangci-lint` on PATH that prints a chosen version, and feeds the hook
 * the same stdin payload Claude Code sends.
 */
import { afterAll, describe, expect, test } from 'bun:test';
import { execFileSync, spawnSync } from 'node:child_process';
import {
  chmodSync,
  mkdirSync,
  mkdtempSync,
  realpathSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

const REPO_ROOT = path.resolve(import.meta.dir, '..');
const HOOK = path.join(
  REPO_ROOT,
  '.claude',
  'hooks',
  'gate-golangci-lint-cache.sh'
);

const CI_YML = `jobs:
  api:
    steps:
      - name: Lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.13.2
          working-directory: api
`;

const LINT =
  'GOLANGCI_LINT_CACHE="$(git rev-parse --show-toplevel)/api/.golangci-cache" golangci-lint run';

const fixtures: string[] = [];
afterAll(() => {
  for (const dir of fixtures) rmSync(dir, { recursive: true, force: true });
});

/** A repo whose ci.yml pins v2.13.2, and a PATH whose golangci-lint says `localVersion`. */
function fixture(localVersion: string | null): {
  repo: string;
  env: NodeJS.ProcessEnv;
} {
  const base = realpathSync(
    mkdtempSync(path.join(tmpdir(), 'gate-golangci-lint-test-'))
  );
  fixtures.push(base);
  const repo = path.join(base, 'repo');
  mkdirSync(path.join(repo, '.github', 'workflows'), { recursive: true });
  writeFileSync(path.join(repo, '.github', 'workflows', 'ci.yml'), CI_YML);
  execFileSync('git', ['init', '-q', repo]);

  const bin = path.join(base, 'bin');
  mkdirSync(bin);
  if (localVersion !== null) {
    const fake = path.join(bin, 'golangci-lint');
    writeFileSync(fake, `#!/bin/sh\necho "${localVersion}"\n`);
    chmodSync(fake, 0o755);
  }
  // Only the fixture's bin plus the system dirs the hook needs (jq, git,
  // grep, awk): a real golangci-lint elsewhere on the developer's PATH
  // must not answer for the fake one.
  const jqDir = path.dirname(
    execFileSync('which', ['jq'], { encoding: 'utf8' }).trim()
  );
  const gitDir = path.dirname(
    execFileSync('which', ['git'], { encoding: 'utf8' }).trim()
  );
  return {
    repo,
    env: {
      ...process.env,
      PATH: [bin, jqDir, gitDir, '/usr/bin', '/bin'].join(':'),
    },
  };
}

/** What the hook decides for `command`, and the reason it gives if it refuses. */
function gate(
  command: string,
  repo: string,
  env: NodeJS.ProcessEnv
): { decision: string; reason: string } {
  const result = spawnSync('bash', [HOOK], {
    cwd: repo,
    env,
    input: JSON.stringify({ tool_name: 'Bash', tool_input: { command } }),
    encoding: 'utf8',
  });
  expect(result.status).toBe(0);
  const out = JSON.parse(result.stdout || '{}').hookSpecificOutput;
  return {
    decision: out?.permissionDecision ?? 'allow',
    reason: out?.permissionDecisionReason ?? '',
  };
}

describe('the version ci.yml pins', () => {
  test('a matching local golangci-lint is allowed', () => {
    const { repo, env } = fixture('2.13.2');
    expect(gate(LINT, repo, env).decision).toBe('allow');
  });

  test('a newer local golangci-lint is refused, naming both versions', () => {
    const { repo, env } = fixture('2.14.0');
    const { decision, reason } = gate(LINT, repo, env);
    expect(decision).toBe('deny');
    expect(reason).toContain('2.14.0');
    expect(reason).toContain('2.13.2');
  });

  test('an older local golangci-lint is refused too', () => {
    const { repo, env } = fixture('2.12.2');
    expect(gate(LINT, repo, env).decision).toBe('deny');
  });

  test('no golangci-lint on PATH is left to fail on its own', () => {
    const { repo, env } = fixture(null);
    expect(gate(LINT, repo, env).decision).toBe('allow');
  });

  test('a command that is not `golangci-lint run` is never checked', () => {
    const { repo, env } = fixture('2.14.0');
    expect(gate('golangci-lint version', repo, env).decision).toBe('allow');
    expect(gate('go test ./...', repo, env).decision).toBe('allow');
  });
});

describe('the scoped cache (#587)', () => {
  test('a matching version without GOLANGCI_LINT_CACHE is still refused', () => {
    const { repo, env } = fixture('2.13.2');
    const { decision, reason } = gate('golangci-lint run', repo, env);
    expect(decision).toBe('deny');
    expect(reason).toContain('GOLANGCI_LINT_CACHE');
  });
});
