#!/usr/bin/env bun
// PreToolUse gate on Edit|Write: blocks a tracked file in the main
// checkout, so a worktree is the only place code changes for a unit of
// work land. Ported from cx-platform-ui's cx-platform-pre-tool-use.ts.
//
// Registered LAST, and only with explicit go-ahead -- see
// docs/agents/worktree-flow.md. Until it is registered, this file has no
// effect.
import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { findMainCheckoutRoot } from './worktree-root.ts';

// Resolved inside main(), not at module load: a throw out here exits 1,
// which the runtime treats as a NON-blocking error -- so a gate that
// crashed would wave through exactly what it exists to refuse (#569).
let SOURCE_ROOT = '';
let WORKTREES_ROOT = '';

function readStdin(): Promise<string> {
	return new Promise((resolve, reject) => {
		let data = '';
		process.stdin.setEncoding('utf8');
		process.stdin.on('data', chunk => {
			data += chunk;
		});
		process.stdin.on('end', () => resolve(data));
		process.stdin.on('error', reject);
	});
}

function isGitIgnored(filePath: string): boolean {
	try {
		execFileSync('git', ['-C', SOURCE_ROOT, 'check-ignore', '-q', filePath], {
			stdio: ['ignore', 'ignore', 'ignore']
		});
		return true;
	} catch {
		return false;
	}
}

async function main(): Promise<void> {
	SOURCE_ROOT = findMainCheckoutRoot(import.meta.dir);
	WORKTREES_ROOT = path.join(SOURCE_ROOT, '.claude', 'worktrees');

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
	if (toolName !== 'Edit' && toolName !== 'Write') process.exit(0);

	const toolInput = payload['tool_input'];
	if (!toolInput || typeof toolInput !== 'object' || Array.isArray(toolInput)) process.exit(0);

	const filePath = (toolInput as Record<string, unknown>)['file_path'];
	if (typeof filePath !== 'string') process.exit(0);

	const resolvedFile = path.resolve(filePath);

	// Allow if already in a worktree.
	if (resolvedFile.startsWith(WORKTREES_ROOT + path.sep)) process.exit(0);

	// Only applies to files inside the main checkout.
	if (!resolvedFile.startsWith(SOURCE_ROOT + path.sep)) process.exit(0);

	// Allow gitignored files -- local/private files (app/.env.local,
	// settings.local.json, scratch files) stay editable in main.
	if (isGitIgnored(resolvedFile)) process.exit(0);

	process.stdout.write(
		JSON.stringify({
			decision: 'block',
			reason:
				'Tracked files cannot be edited in the main checkout.\n' +
				'Use EnterWorktree to create a worktree first, then retry.\n\n' +
				'Branch format: <type>/<issue>-<description>\n' +
				'Example: fix/510-labeled-field-inline-row\n\n' +
				'See docs/agents/worktree-flow.md.'
		})
	);
	process.exit(2);
}

// Fail CLOSED (#569). Only exit code 2 blocks; every other non-zero code
// is a non-blocking error and the edit proceeds. A gate that cannot run
// has not decided the edit is safe, so it refuses rather than permits.
main().catch((error: unknown) => {
	process.stdout.write(
		JSON.stringify({
			decision: 'block',
			reason:
				'The worktree edit gate could not run, so it cannot say this edit is safe: ' +
				`${error instanceof Error ? error.message : String(error)}\n` +
				'Fix the gate, or edit inside a worktree via EnterWorktree.'
		})
	);
	process.exit(2);
});
