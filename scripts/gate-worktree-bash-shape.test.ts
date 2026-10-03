/**
 * Refusals from `.claude/hooks/gate-worktree-bash-shape.ts` (#1678): in a
 * worktree-isolated session, each command shape the harness refuses as
 * "too complex to verify" is refused first, with a message that names the
 * form that passes. `gh pr create --fill` is refused everywhere.
 */
import { describe, expect, test } from 'bun:test';
import { spawn } from 'node:child_process';
import path from 'node:path';
import { MESSAGES } from '../.claude/hooks/gate-worktree-bash-shape.ts';

const HOOK = path.resolve(
  import.meta.dir,
  '..',
  '.claude',
  'hooks',
  'gate-worktree-bash-shape.ts'
);
const WORKTREE = '/repo/.claude/worktrees/fix-1678-shapes';
const MAIN = '/repo';

function invoke(
  command: string,
  cwd: string
): Promise<{ exitCode: number; reason: string | undefined }> {
  return new Promise((resolve, reject) => {
    const child = spawn('bun', [HOOK], { stdio: ['pipe', 'pipe', 'inherit'] });
    let stdout = '';
    child.stdout.on('data', (chunk) => {
      stdout += chunk;
    });
    child.on('error', reject);
    child.on('close', (code) => {
      const output = stdout.trim() ? JSON.parse(stdout) : {};
      resolve({
        exitCode: code ?? 1,
        reason: output.hookSpecificOutput?.permissionDecisionReason,
      });
    });
    child.stdin.write(
      JSON.stringify({ tool_name: 'Bash', cwd, tool_input: { command } })
    );
    child.stdin.end();
  });
}

const SHAPES: [keyof typeof MESSAGES, string][] = [
  ['heredoc', "git commit -F - <<'EOF'\nA subject\nEOF"],
  ['export', 'export DOCKER_HOST=$(podman machine inspect) && go test ./...'],
  ['export', 'export TZ=UTC; bun test'],
  ['jq', 'gh pr view 12 --jq \'"\\(.number) \\(.state)"\''],
  ['jq', "gh pr view 12 --json state,mergeStateStatus --jq '{s: .state}'"],
  ['git', 'git -C /repo/.claude/worktrees/other status'],
  ['git', 'cd /repo/.claude/worktrees/other && git status'],
  ['sleep', 'sleep 60; gh pr view 12 --json state'],
];

describe('gate-worktree-bash-shape', () => {
  describe('in a worktree-isolated session', () => {
    for (const [shape, command] of SHAPES) {
      test(`refuses ${shape}: ${command.split('\n')[0]}`, async () => {
        const { exitCode, reason } = await invoke(command, WORKTREE);
        expect(exitCode).toBe(0);
        expect(reason).toContain(MESSAGES[shape]);
      });
    }

    test('names every shape a command has', async () => {
      const { reason } = await invoke(
        'export X=1; cd /tmp && git status',
        WORKTREE
      );
      expect(reason).toContain(MESSAGES.export);
      expect(reason).toContain(MESSAGES.git);
    });

    const PASSING = [
      'git commit -F /tmp/msg.txt',
      'TZ=UTC bun test',
      'gh pr view 12 --json state --jq \'.state + " " + .mergeStateStatus\'',
      'git status',
      'until gh pr view 12 --json state | grep -q MERGED; do sleep 60; done',
      // A shape named inside a quoted argument is text, not a shape.
      'git commit -m "explain why export X=1 and <<EOF are refused"',
      'grep -n "cd x && git -C y" docs/agents/worktree-flow.md',
      'cat <<<"a here-string is not a heredoc"',
    ];
    for (const command of PASSING) {
      test(`allows ${command}`, async () => {
        const { exitCode, reason } = await invoke(command, WORKTREE);
        expect(exitCode).toBe(0);
        expect(reason).toBeUndefined();
      });
    }
  });

  describe('in the main checkout', () => {
    for (const [shape, command] of SHAPES) {
      test(`does not refuse ${shape}: ${command.split('\n')[0]}`, async () => {
        const { exitCode, reason } = await invoke(command, MAIN);
        expect(exitCode).toBe(0);
        expect(reason).toBeUndefined();
      });
    }
  });

  describe('gh pr create --fill', () => {
    for (const cwd of [WORKTREE, MAIN]) {
      for (const flag of ['--fill', '--fill-first', '-f']) {
        test(`is refused with ${flag} in ${cwd}`, async () => {
          const { reason } = await invoke(
            `gh pr create ${flag} --base trunk`,
            cwd
          );
          expect(reason).toContain(MESSAGES.fill);
          expect(reason).toContain('--body-file');
          expect(reason).toContain('unwrapped');
        });
      }
    }

    test('gh pr create --body-file is allowed', async () => {
      const { reason } = await invoke(
        'gh pr create --title "x (#1)" --body-file /tmp/body.md',
        WORKTREE
      );
      expect(reason).toBeUndefined();
    });
  });

  test('fails open on a payload it cannot read', async () => {
    const child = spawn('bun', [HOOK], { stdio: ['pipe', 'pipe', 'inherit'] });
    let stdout = '';
    child.stdout.on('data', (chunk) => {
      stdout += chunk;
    });
    const exitCode = await new Promise<number>((resolve) => {
      child.on('close', (code) => resolve(code ?? 1));
      child.stdin.end('not json');
    });
    expect(exitCode).toBe(0);
    expect(stdout).toBe('');
  });
});
