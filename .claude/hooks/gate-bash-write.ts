#!/usr/bin/env bun
// PreToolUse gate on Bash: blocks a Bash command that writes to a tracked
// path in the main checkout, the same boundary gate-worktree-edit.ts
// enforces for Edit/Write (#573).
//
// This only recognizes a fixed set of shell write patterns (redirection,
// `tee`, `sed -i`, `cp`/`mv`/`install`/`rsync`, `dd of=`). A command that
// writes some other way -- an opaque pipeline, a script invoked by name, a
// scripting-language one-liner, a compiled binary -- is not caught. That
// gap is accepted on purpose; see docs/agents/worktree-flow.md's
// Enforcement section.
//
// Decision (#702): a target holding an unexpanded shell variable or command
// substitution (`$S/c298.md`, `${S}/c298.md`, `$(cmd)/c298.md`) is skipped
// rather than resolved. This hook sees the command string before the shell
// expands it, so `path.resolve` would pin that literal text to the working
// directory -- producing a path that resolves inside the checkout by
// accident, which `isTrackedInMainCheckout` then calls tracked because it
// never consults git's index at all. The alternative -- teaching
// `isTrackedInMainCheckout` to consult the index -- was rejected here: it is
// a wider change (it also decides how an untracked new file in the checkout
// is treated, a question #702 does not need answered) for a narrower
// ticket. Skipping is consistent with this file's already-declared
// incomplete-by-construction posture and costs nothing extra. What it gives
// up: a caller that deliberately hides a tracked-checkout path behind a
// variable to dodge the gate goes undetected -- accepted, the same as the
// gaps above, because the GitHub trunk ruleset (docs/agents/worktree-flow.md)
// is the real, unconditional boundary; this hook is a local nudge on top of
// it, not a substitute for it.
//
// Decision (#677): the redirection scan used to run over the raw command
// text with no notion of quoting or heredocs, so a literal `>` sitting in
// prose -- an HTML tag, a CSS child combinator, an arrow in an ordinary
// sentence, a markdown blockquote marker starting a heredoc body -- was read
// as a redirection operator and the next word became a fabricated "write
// target". This also false-blocked a `gh issue create`/`edit`/`comment` or
// `gh pr create`/`edit` invocation over characters inside its own
// `--body`/`--title`/`-m` value, even though none of those commands write
// to a local tracked path at all. Two changes, same textual-not-a-shell-
// parser posture as the gaps above: (1) a heredoc body (between a
// `<<DELIM` introducer and the line that is exactly its closing delimiter)
// is stripped before scanning, since it is stdin content and never a write
// target regardless of what characters it holds; (2) a `>` is only treated
// as an operator when it is not inside a single- or double-quoted span, so
// `echo "1085 -> Ready"` and a `--body "..."` value both scan clean. A
// command-name allowlist for `gh issue`/`gh pr` was considered and rejected:
// it would have to skip the whole segment's candidate collection to reach
// every reported case, which would also swallow a real trailing redirect on
// the same command line (`gh issue create --body "x" > CLAUDE.md`) -- a
// genuine write this hook exists to catch. Masking covers every value that
// is actually quoted or heredoc'd, which is the only way such a value can
// hold a literal `>` and still mean what it says in real shell syntax;
// anything else is not this ticket's problem to solve. What this still
// gives up, on purpose: quoting is not unwound anywhere else in this file
// (see `tokenize` below), so a real write target that is itself quoted and
// contains a `;`/`|`/`&` that `segments` splits on is still missed -- the
// same accepted gap as before, now stated for the masking pass too.
//
// Decision (#680): a relative write target is resolved against the
// directory a leading `cd` in the same command would put a real shell in,
// not against the hook process's own cwd. `path.resolve(candidate)` binds a
// relative candidate to `process.cwd()`, and a Bash tool call's cwd is
// reset to the main checkout root before every invocation -- so
// `cd <worktree> && sed -i '' '...' <relative-path>` was measured against
// the main checkout and blocked, even though the write genuinely lands
// inside the worktree. That is the inverse of the gaps above: a write this
// hook does recognize, mislocated. `segments` already yields the stages in
// order, so a stage whose first token is `cd` now updates a tracked base
// directory for every later stage, chained `cd a && cd b` included, and a
// candidate resolves against that base. Two guards keep this from ever
// weakening the gate. A base is only honored when it lands inside the main
// checkout root -- the checkout itself or the worktree pool under it -- so
// a `cd` into a directory this hook knows nothing about (`cd /tmp`) cannot
// move a relative candidate out of reach; #680's report sketched a
// pool-only rule, and honoring the whole checkout root is strictly
// stricter, since a `cd` into main keeps blocking whichever checkout the
// hook process itself happens to stand in. And a base that cannot be
// established at all -- a bare `cd`, `cd -`, `cd ~...`, or an argument
// still holding an unexpanded variable or command substitution -- falls
// back to today's behavior rather than guessing, with only a later
// absolute `cd` restoring tracking.
//
// The separator between two stages decides whether a base survives, and
// only `&&` does. That is the one separator under which a shell is
// *guaranteed* to be standing in the new directory when the next stage
// runs: a `cd` that fails short-circuits the chain, so the write never
// happens at all. Every other separator would hand this hook a base a real
// shell need not be in -- `cd <worktree> || <write> <relative-path>` runs
// the write precisely when the `cd` failed; `cd <worktree> & <write> ...`
// and `cd <worktree> | true && <write> ...` run the `cd` in a subshell
// whose directory dies with it; `cd <nonexistent> ; <write> ...` leaves
// the shell exactly where it was. All of those write to the main checkout
// for real, so none of them may resolve anywhere but the fallback. An
// unquoted parenthesis anywhere in the command disables `cd` tracking for
// that command outright, for the same reason and one this file cannot see
// past: `segments` strips a leading `(`, so `(cd <worktree>) && <write>
// <relative-path>` would otherwise read as a `cd` that outlives its
// subshell. What this still gives up, on purpose: `pushd`/`popd` are not
// modeled, and a command holding an unquoted parenthesis or joining its
// stages with anything but `&&` gets no `cd` tracking even where a shell
// would have kept the directory -- a write blocked that need not have
// been, which is this file's safe direction to err. It stays a textual
// scanner, not a shell parser.
import path from 'node:path';
import { isTrackedInMainCheckout, readStdin } from './tracked-path.ts';
import { findMainCheckoutRoot } from './worktree-root.ts';

// Removes a heredoc body (`<<DELIM ... DELIM`, `<<'DELIM' ... DELIM`,
// `<<-DELIM ... \tDELIM`) from the raw command before anything else scans
// it. A heredoc body is stdin content, never a write target, no matter what
// it contains -- but `segments` (below) splits on every newline, so left in
// place a body's own lines get read as separate fake commands. Multiple
// heredocs in one command are each found and stripped in turn. An
// unterminated heredoc (no line matching the delimiter) stops the scan at
// its introducer -- nothing after it can be attributed reliably, and this
// hook fails closed elsewhere, not here: no candidates surviving just means
// nothing blocks, the same "not caught" gap already accepted in the header.
function stripHeredocBodies(command: string): string {
  const introducer = /<<-?\s*(['"]?)([A-Za-z_][A-Za-z0-9_]*)\1/g;
  let result = '';
  let cursor = 0;
  let match: RegExpExecArray | null;

  while ((match = introducer.exec(command))) {
    const delimiter = match[2] ?? '';
    const introEnd = introducer.lastIndex;
    result += command.slice(cursor, introEnd);

    const bodyStart = command.indexOf('\n', introEnd);
    if (bodyStart === -1) {
      cursor = introEnd;
      break;
    }

    const closing = new RegExp(
      `^\\t*${delimiter.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*$`,
      'm'
    );
    const rest = command.slice(bodyStart + 1);
    const closeMatch = closing.exec(rest);
    if (!closeMatch) {
      cursor = bodyStart + 1;
      break;
    }

    result += '\n';
    cursor = bodyStart + 1 + closeMatch.index + closeMatch[0].length;
    introducer.lastIndex = cursor;
  }

  return result + command.slice(cursor);
}

// Blanks a `>` that sits inside a single- or double-quoted span, so the
// redirection scan below cannot read it as an operator. Everything else --
// including a `>` that precedes a quoted target, e.g. `echo hi > "f.md"` --
// passes through untouched, so a genuinely quoted write target is still
// recognized. Double-quote escaping (`\"`, `\\`) is tracked only well enough
// to find the real closing quote; single quotes have no escape mechanism in
// shell, so none is needed there.
function maskQuotedRedirectionChars(command: string): string {
  let out = '';
  let quote: '"' | "'" | null = null;

  for (let i = 0; i < command.length; i++) {
    const char = command[i] ?? '';

    if (quote === '"' && char === '\\' && i + 1 < command.length) {
      out += char + command[i + 1];
      i++;
      continue;
    }

    if (quote !== null) {
      if (char === quote) quote = null;
      out += char === '>' ? '_' : char;
      continue;
    }

    if (char === "'" || char === '"') quote = char;
    out += char;
  }

  return out;
}

// One stage of a list or pipeline, with the separator that ends it -- `&&`,
// `||`, `|`, `&`, `;`, a newline, or `''` at the end of the command. Only
// `&&` carries a `cd`'s working directory into the next stage (#680), so
// the separator has to survive the split rather than being discarded with
// it.
interface Stage {
  text: string;
  terminator: string;
}

// One list/pipeline stage per entry, so a write in one stage of
// `A | tee file` or `A && sed -i ... file` is examined on its own --
// command-name detection (tee/sed/cp/mv/install/rsync) keys off the first
// token of a segment, so a write verb chained after `&&`/`&` that stayed
// in the same segment as its predecessor would never be seen as the
// segment's own command. Splits on every `&`, same as
// gate-shared-index.sh's `tr '\n;|&'` -- that does tear `2>&1` into two
// pieces, but the redirection regex below only needs a `>` with a target
// after it, which survives the split intact either side.
function segments(command: string): Stage[] {
  // The capture group keeps each separator in the split output, so the
  // parts alternate stage, separator, stage, separator, ..., stage. An
  // empty stage is kept rather than filtered out, so the separator that
  // follows it is not lost with it; it contributes no write target and no
  // `cd` either way.
  const parts = command.split(/(\r?\n|;|\|\||\||&&|&)/);
  const stages: Stage[] = [];

  for (let index = 0; index < parts.length; index += 2) {
    stages.push({
      text: (parts[index] ?? '').replace(/^[\s(){]*/, '').trim(),
      terminator: parts[index + 1] ?? '',
    });
  }

  return stages;
}

// True when the command holds a `(` or `)` outside every quoted span, using
// the same quote tracking as `maskQuotedRedirectionChars`. A parenthesis
// means a subshell (or a command substitution whose text survived), and
// `segments` cannot tell where one ends -- so no `cd` in such a command is
// honored at all (#680).
function hasUnquotedParenthesis(command: string): boolean {
  let quote: '"' | "'" | null = null;

  for (let index = 0; index < command.length; index++) {
    const char = command[index] ?? '';

    if (quote === '"' && char === '\\' && index + 1 < command.length) {
      index++;
      continue;
    }

    if (quote !== null) {
      if (char === quote) quote = null;
      continue;
    }

    if (char === "'" || char === '"') quote = char;
    else if (char === '(' || char === ')') return true;
  }

  return false;
}

function tokenize(segment: string): string[] {
  // A plain whitespace split -- quoting is not unwound, so a quoted path
  // containing whitespace is missed. Accepted gap, see the file header.
  return segment.match(/\S+/g) ?? [];
}

function isSkippableTarget(target: string): boolean {
  return (
    target.startsWith('&') ||
    /^\d+$/.test(target) ||
    target === '/dev/null' ||
    target === '/dev/stdout' ||
    target === '/dev/stderr'
  );
}

function stripQuotes(target: string): string {
  return target.replace(/^(['"])(.*)\1$/, '$2');
}

// True for a target that still holds a shell variable or command
// substitution (`$VAR`, `${VAR}`, `$(cmd)`, backticks) -- this hook sees the
// command before the shell expands any of these, so the text is not the
// real path (#702). See the decision note in the file header.
function hasUnexpandedVariable(target: string): boolean {
  return /\$\{|\$\(|\$[A-Za-z_]|`/.test(target);
}

// Candidate write targets for one pipeline/list stage. Over-collecting is
// fine -- every candidate still has to resolve inside the main checkout
// and outside gitignore to actually block anything.
function writeTargets(segment: string): string[] {
  const targets: string[] = [];

  for (const match of segment.matchAll(/\d*(>>?)(?!&)\s*([^\s|;&]+)/g)) {
    const target = match[2] ?? '';
    if (target && !isSkippableTarget(target)) targets.push(target);
  }

  for (const match of segment.matchAll(/(?:^|\s)of=(\S+)/g)) {
    if (match[1]) targets.push(match[1]);
  }

  const tokens = tokenize(segment);
  const command = tokens[0];

  if (command === 'tee') {
    targets.push(...tokens.slice(1).filter((token) => !token.startsWith('-')));
  }

  if (command === 'sed' && tokens.some((token) => token.startsWith('-i'))) {
    const last = tokens[tokens.length - 1];
    if (last && !last.startsWith('-')) targets.push(last);
  }

  if (
    command === 'cp' ||
    command === 'mv' ||
    command === 'install' ||
    command === 'rsync'
  ) {
    const positional = tokens
      .slice(1)
      .filter((token) => !token.startsWith('-'));
    const destination = positional[positional.length - 1];
    if (positional.length > 1 && destination) targets.push(destination);
  }

  return targets
    .map(stripQuotes)
    .filter((target) => !hasUnexpandedVariable(target));
}

// A write target together with the directory it should resolve against
// (#680). `base` is `null` when a `cd` in the command made the working
// directory unknowable; see `resolveCandidate` for what that falls back to.
interface Candidate {
  target: string;
  base: string | null;
}

// The argument of a `cd` stage: the path string when it can be trusted,
// `null` when the stage is a `cd` whose destination cannot be established
// (bare `cd`, `cd -`, `cd ~...`, or an argument still holding a shell
// variable or command substitution), and `undefined` when the stage is not
// a `cd` at all.
function cdArgument(segment: string): string | null | undefined {
  const tokens = tokenize(segment);
  if (tokens[0] !== 'cd') return undefined;

  const argument = tokens.slice(1).find((token) => !token.startsWith('-'));
  if (!argument) return null;

  const stripped = stripQuotes(argument);
  if (stripped.startsWith('~') || hasUnexpandedVariable(stripped)) return null;

  return stripped;
}

// Walks the stages in order, carrying the working directory a real shell is
// guaranteed to be standing in. A `cd` stage is scanned for write targets
// too before it moves the base -- a redirection on a `cd` line takes effect
// in the directory the shell is leaving, not the one it is entering -- and
// any separator but `&&` drops the base back to the fallback, because past
// one of those the shell need not be where the `cd` aimed. See the file
// header for why each of those cases is a real main-checkout write.
function collectCandidates(scannable: string): Candidate[] {
  const honorCd = !hasUnquotedParenthesis(scannable);
  const fallback = process.cwd();
  const candidates: Candidate[] = [];
  let base: string | null = fallback;

  for (const stage of segments(scannable)) {
    for (const target of writeTargets(stage.text))
      candidates.push({ target, base });

    const argument = honorCd ? cdArgument(stage.text) : undefined;
    if (argument === null) base = null;
    else if (argument !== undefined) {
      if (path.isAbsolute(argument)) base = path.resolve(argument);
      else if (base !== null) base = path.resolve(base, argument);
    }

    if (stage.terminator !== '&&') base = fallback;
  }

  return candidates;
}

// Where a candidate actually lands. An absolute target ignores the base
// entirely. A relative one uses the tracked base only when that base is
// inside the main checkout root; anything else falls back to resolving
// against the hook process's own cwd, which is what this file did before
// #680.
function resolveCandidate(candidate: Candidate, sourceRoot: string): string {
  if (path.isAbsolute(candidate.target)) return path.resolve(candidate.target);

  const base = candidate.base;
  const insideCheckout =
    base !== null &&
    (base === sourceRoot || base.startsWith(sourceRoot + path.sep));

  return insideCheckout
    ? path.resolve(base, candidate.target)
    : path.resolve(candidate.target);
}

function block(reason: string): never {
  process.stdout.write(JSON.stringify({ decision: 'block', reason }));
  process.exit(2);
}

async function main(): Promise<void> {
  const raw = await readStdin();
  let payload: Record<string, unknown> = {};

  if (raw.trim()) {
    try {
      payload = JSON.parse(raw);
    } catch {
      process.exit(0);
    }
  }

  const toolName =
    typeof payload['tool_name'] === 'string' ? payload['tool_name'] : '';
  if (toolName !== 'Bash') process.exit(0);

  const toolInput = payload['tool_input'];
  if (!toolInput || typeof toolInput !== 'object' || Array.isArray(toolInput))
    process.exit(0);

  const command = (toolInput as Record<string, unknown>)['command'];
  if (typeof command !== 'string') process.exit(0);

  const scannable = maskQuotedRedirectionChars(stripHeredocBodies(command));
  const candidates = collectCandidates(scannable);
  if (candidates.length === 0) process.exit(0);

  // Only pay for a git subprocess once there is something to check.
  const sourceRoot = findMainCheckoutRoot(import.meta.dir);
  const worktreesRoot = path.join(sourceRoot, '.claude', 'worktrees');

  for (const candidate of candidates) {
    const resolvedFile = resolveCandidate(candidate, sourceRoot);

    if (!isTrackedInMainCheckout(resolvedFile, sourceRoot, worktreesRoot))
      continue;

    block(
      `This Bash command would write to ${path.relative(sourceRoot, resolvedFile)}, a tracked path in the main checkout.\n` +
        'Use EnterWorktree to create a worktree first, then retry.\n\n' +
        'Branch format: <type>/<issue>-<description>\n' +
        'Example: fix/510-labeled-field-inline-row\n\n' +
        'See docs/agents/worktree-flow.md.'
    );
  }

  process.exit(0);
}

// Fail CLOSED, same as gate-worktree-edit.ts (#569): a gate that could not
// run has not decided the command is safe, so it refuses rather than
// permits.
main().catch((error: unknown) => {
  block(
    'The worktree Bash-write gate could not run, so it cannot say this command is safe: ' +
      `${error instanceof Error ? error.message : String(error)}\n` +
      'Fix the gate, or run this command inside a worktree via EnterWorktree.'
  );
});
