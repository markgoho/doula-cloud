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
import path from 'node:path';
import { isTrackedInMainCheckout, readStdin } from './tracked-path.ts';
import { findMainCheckoutRoot } from './worktree-root.ts';

// One list/pipeline stage per entry, so a write in one stage of
// `A | tee file` or `A && sed -i ... file` is examined on its own --
// command-name detection (tee/sed/cp/mv/install/rsync) keys off the first
// token of a segment, so a write verb chained after `&&`/`&` that stayed
// in the same segment as its predecessor would never be seen as the
// segment's own command. Splits on every `&`, same as
// gate-shared-index.sh's `tr '\n;|&'` -- that does tear `2>&1` into two
// pieces, but the redirection regex below only needs a `>` with a target
// after it, which survives the split intact either side.
function segments(command: string): string[] {
	return command
		.split(/\r?\n|;|\|\||\||&&|&/)
		.map(segment => segment.replace(/^[\s(){]*/, '').trim())
		.filter(Boolean);
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
		targets.push(...tokens.slice(1).filter(token => !token.startsWith('-')));
	}

	if (command === 'sed' && tokens.some(token => token.startsWith('-i'))) {
		const last = tokens[tokens.length - 1];
		if (last && !last.startsWith('-')) targets.push(last);
	}

	if (command === 'cp' || command === 'mv' || command === 'install' || command === 'rsync') {
		const positional = tokens.slice(1).filter(token => !token.startsWith('-'));
		const destination = positional[positional.length - 1];
		if (positional.length > 1 && destination) targets.push(destination);
	}

	return targets.map(stripQuotes);
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

	const toolName = typeof payload['tool_name'] === 'string' ? payload['tool_name'] : '';
	if (toolName !== 'Bash') process.exit(0);

	const toolInput = payload['tool_input'];
	if (!toolInput || typeof toolInput !== 'object' || Array.isArray(toolInput)) process.exit(0);

	const command = (toolInput as Record<string, unknown>)['command'];
	if (typeof command !== 'string') process.exit(0);

	const candidates = segments(command).flatMap(writeTargets);
	if (candidates.length === 0) process.exit(0);

	// Only pay for a git subprocess once there is something to check.
	const sourceRoot = findMainCheckoutRoot(import.meta.dir);
	const worktreesRoot = path.join(sourceRoot, '.claude', 'worktrees');

	for (const candidate of candidates) {
		const resolvedFile = path.resolve(candidate);

		if (!isTrackedInMainCheckout(resolvedFile, sourceRoot, worktreesRoot)) continue;

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
