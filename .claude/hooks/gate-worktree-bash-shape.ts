#!/usr/bin/env bun
// PreToolUse gate on Bash (#1678): refuses the command shapes that Claude
// Code's own worktree isolation refuses as "too complex to verify that it
// stays inside the worktree", and names the form that passes.
//
// The harness's refusal text says nothing about what would pass, so each
// refusal used to cost one to three calls and a rewrite. This hook runs
// first and says what to write instead -- the same shape as
// gate-gh-run-watch.sh, which agents followed every time. It only nudges:
// the harness is still the boundary, so this hook fails open on anything
// it cannot read.
//
// The shapes apply only in a worktree-isolated session, recognized the way
// worktree-claim.ts and gate-worktree-cleanup.ts recognize one: the
// session's cwd is under `.claude/worktrees/`. `gh pr create --fill` is
// refused everywhere, because it copies a hard-wrapped commit body into the
// PR body wherever it runs.
//
// It is a textual scanner, not a shell parser. Quoted text is blanked
// before the shape checks run, so a shape named inside a quoted argument (a
// commit message, a grep pattern) is not a match.
import path from 'node:path';
import { readStdin } from './tracked-path.ts';

interface Stage {
  start: number;
  end: number;
}

// Replaces every character inside a single- or double-quoted span with a
// space, keeping the quote characters and the length, so offsets into the
// masked text are offsets into the raw text too. A `$(...)` is a command
// the shell runs even inside double quotes, so its text stays visible:
// `git commit -m "$(cat <<'EOF' ...` still shows its heredoc.
export function maskQuoted(command: string): string {
  let out = '';
  const stack: ('"' | "'" | '(')[] = [];

  for (let index = 0; index < command.length; index++) {
    const char = command[index] ?? '';
    const next = command[index + 1];
    const top = stack.at(-1);

    if (top === "'") {
      if (char === "'") stack.pop();
      out += char === "'" ? char : ' ';
    } else if (top === '"') {
      if (char === '\\' && next !== undefined) {
        out += '  ';
        index++;
      } else if (char === '$' && next === '(') {
        stack.push('(');
        out += '$(';
        index++;
      } else {
        if (char === '"') stack.pop();
        out += char === '"' ? char : ' ';
      }
    } else if (char === '\\' && next !== undefined) {
      out += char + next;
      index++;
    } else {
      if (char === "'" || char === '"') stack.push(char);
      else if (char === '(' && (top === '(' || command[index - 1] === '$'))
        stack.push('(');
      else if (char === ')' && top === '(') stack.pop();
      out += char;
    }
  }

  return out;
}

// The list and pipeline stages of the masked command, as offsets.
function stages(masked: string): Stage[] {
  const result: Stage[] = [];
  const separator = /\r?\n|;|\|\||\||&&|&/g;
  let start = 0;
  let match: RegExpExecArray | null;

  while ((match = separator.exec(masked))) {
    result.push({ start, end: match.index });
    start = separator.lastIndex;
  }
  result.push({ start, end: masked.length });

  return result;
}

// The words of a stage, after any leading `(`/`{` and `X=value` assignments.
function commandWords(text: string): string[] {
  const words = text.replace(/^[\s(){]*/, '').match(/\S+/g) ?? [];
  while (words.length > 0 && /^[A-Za-z_][A-Za-z0-9_]*=/.test(words[0] ?? ''))
    words.shift();
  return words;
}

// Read on the masked text, where a quoted delimiter (`<<'EOF'`) is left as
// its opening quote; `<<<` is a here-string, not a heredoc.
const HEREDOC = /(^|[^<])<<-?\s*['"A-Za-z_\\]/;
// The flag is found on the masked text, so a `--jq` quoted inside another
// argument is text; its filter is then read from the raw text at the same
// offset.
const JQ_FLAG = /(?:^|\s)(?:--jq|-q)(?:=|\s+)/g;
const QUOTED_FILTER = /^('[^']*'|"(?:[^"\\]|\\.)*")/;
const FILL = /^(--fill(-first|-verbose)?|-f)(=.*)?$/;

export const MESSAGES = {
  heredoc:
    'A heredoc (`<<EOF`): write the content to a file with the Write tool, then run the command on the file (`git commit -F <file>`, `gh pr create --body-file <file>`).',
  export:
    '`export X=...`: put the variable inline on the command that needs it, `X=value cmd`.',
  jq: 'A `--jq` filter with a `\\(...)` string template or a `{...}` object: join plain paths instead, `--jq \'.a + " " + .b\'` (`(.n | tostring)` for a number).',
  git: '`git -C <path>` or `cd <dir> && git ...`: run git in a separate call of its own. The Bash tool already starts in the worktree; name any other path absolutely.',
  sleep:
    '`sleep N` followed by a command: wait in a background call, `until <check>; do sleep N; done`, or poll with separate repeated calls.',
  fill: '`gh pr create --fill` copies the hard-wrapped commit body into the PR body. Write the body as unwrapped prose to a file with the Write tool and pass it with `gh pr create --title "..." --body-file <file>`.',
} as const;

export type Shape = keyof typeof MESSAGES;

export function inWorktree(cwd: string): boolean {
  return path
    .resolve(cwd)
    .includes(`${path.sep}.claude${path.sep}worktrees${path.sep}`);
}

// Every shape the command has. `worktree` decides whether the
// isolation-only shapes are checked at all.
export function shapesOf(command: string, worktree: boolean): Shape[] {
  const masked = maskQuoted(command);
  const all = stages(masked).map((stage) => ({
    raw: command.slice(stage.start, stage.end),
    text: masked.slice(stage.start, stage.end),
    words: commandWords(masked.slice(stage.start, stage.end)),
  }));
  const found = new Set<Shape>();

  for (const { words } of all) {
    if (words[0] === 'gh' && words[1] === 'pr' && words[2] === 'create') {
      if (words.some((word) => FILL.test(word))) found.add('fill');
    }
  }

  if (!worktree) return [...found];

  if (HEREDOC.test(masked)) found.add('heredoc');

  let sawCd = false;
  const nonEmpty = all.filter(({ words }) => words.length > 0);
  for (const [index, { raw, text, words }] of nonEmpty.entries()) {
    const verb = words[0];

    if (verb === 'export' && words.slice(1).some((word) => word.includes('=')))
      found.add('export');

    if (verb === 'git' && (sawCd || words[1] === '-C')) found.add('git');
    if (verb === 'cd') sawCd = true;

    if (verb === 'gh') {
      for (const flag of text.matchAll(JQ_FLAG)) {
        const offset = flag.index + flag[0].length;
        const filter = QUOTED_FILTER.exec(raw.slice(offset))?.[1] ?? '';
        if (/\\\(|\{/.test(filter)) found.add('jq');
      }
    }

    if (
      index === 0 &&
      verb === 'sleep' &&
      /^\d+(\.\d+)?[smhd]?$/.test(words[1] ?? '') &&
      nonEmpty.length > 1
    )
      found.add('sleep');
  }

  return [...found];
}

export function refusal(shapes: Shape[]): string {
  return [
    'Worktree isolation refuses this command shape as "too complex to verify", and its own message does not say what passes. Use the form that passes:',
    ...shapes.map((shape) => `- ${MESSAGES[shape]}`),
    'See docs/agents/worktree-flow.md.',
  ].join('\n');
}

async function main(): Promise<void> {
  const payload = JSON.parse((await readStdin()) || '{}') as Record<
    string,
    unknown
  >;
  const toolInput = payload['tool_input'] as Record<string, unknown> | null;
  const command = toolInput?.['command'];
  if (payload['tool_name'] !== 'Bash' || typeof command !== 'string') return;

  const cwd =
    typeof payload['cwd'] === 'string' && payload['cwd']
      ? payload['cwd']
      : process.cwd();
  const shapes = shapesOf(command, inWorktree(cwd));
  if (shapes.length === 0) return;

  process.stdout.write(
    JSON.stringify({
      hookSpecificOutput: {
        hookEventName: 'PreToolUse',
        permissionDecision: 'deny',
        permissionDecisionReason: refusal(shapes),
      },
    })
  );
}

// Fail open: this hook is a nudge in front of the harness's own refusal,
// so a hook that cannot run must never block a command.
if (import.meta.main) main().catch(() => process.exit(0));
