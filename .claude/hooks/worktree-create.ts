#!/usr/bin/env bun
// WorktreeCreate hook -- fires for both EnterWorktree (payload carries
// `name`) and ExitWorktree / worktree removal (payload carries
// `worktree_path`). Modeled on cx-platform-ui's
// cx-platform-worktree-create.ts, which is the empirical source for this
// payload shape (verified against ~/Github/cx-platform-ui/.claude/settings.local.json).
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { syncTrunkToOrigin } from './sync-trunk.ts';
import { provisionWorktree, WORKTREES_ROOT_PATH, SOURCE_ROOT_PATH } from './worktree-provision.ts';

type JsonObject = Record<string, unknown>;

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

function asObject(value: unknown): JsonObject | null {
	return value && typeof value === 'object' && !Array.isArray(value) ? (value as JsonObject) : null;
}

function getString(value: unknown): string | null {
	return typeof value === 'string' && value.trim() ? value.trim() : null;
}

function getNestedString(payload: JsonObject, key: string): string | null {
	const direct = getString(payload[key]);
	if (direct) return direct;
	for (const nestedKey of ['hookSpecificInput', 'input', 'payload', 'context']) {
		const nested = asObject(payload[nestedKey]);
		if (!nested) continue;
		const value = getString(nested[key]);
		if (value) return value;
	}
	return null;
}

function sanitizeSlug(value: string): string {
	return value
		.replace(/[^A-Za-z0-9._-]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.slice(0, 80);
}

function log(message: string): void {
	process.stderr.write(`worktree-create: ${message}\n`);
}

function runGit(args: string[]): string {
	return execFileSync('git', ['-C', SOURCE_ROOT_PATH, ...args], {
		encoding: 'utf8',
		stdio: ['ignore', 'pipe', 'pipe']
	}).trim();
}

function createWorktree(name: string): string {
	const slug = sanitizeSlug(name);
	if (!slug) throw new Error('missing usable worktree name');

	fs.mkdirSync(WORKTREES_ROOT_PATH, { recursive: true });
	const worktreePath = path.join(WORKTREES_ROOT_PATH, slug);

	if (path.resolve(worktreePath) === SOURCE_ROOT_PATH) {
		throw new Error('refusing to treat source repo as worktree');
	}

	if (!fs.existsSync(worktreePath)) {
		runGit(['worktree', 'add', worktreePath]);
	}

	provisionWorktree(worktreePath);
	return worktreePath;
}

function removeWorktree(worktreePathInput: string): void {
	const worktreePath = path.resolve(worktreePathInput);
	if (!worktreePath.startsWith(`${WORKTREES_ROOT_PATH}${path.sep}`)) {
		throw new Error(`refusing to remove unmanaged worktree: ${worktreePath}`);
	}
	if (!fs.existsSync(worktreePath)) {
		log(`worktree already removed: ${worktreePath}`);
		return;
	}
	runGit(['worktree', 'remove', worktreePath]);

	// This is the moment this repo's flow actually produces: a worktree
	// just landed via its PR and ExitWorktree is putting the session back
	// on trunk (docs/agents/worktree-flow.md). The commit that PR just
	// added should already be here, not wait for the next SessionStart.
	syncTrunkToOrigin(SOURCE_ROOT_PATH);
}

async function main(): Promise<void> {
	const raw = await readStdin();
	let payload: JsonObject = {};

	if (raw.trim()) {
		try {
			payload = asObject(JSON.parse(raw)) ?? {};
		} catch {
			throw new Error('received non-JSON hook payload');
		}
	}

	const worktreePath = getNestedString(payload, 'worktree_path');
	if (worktreePath) {
		removeWorktree(worktreePath);
		return;
	}

	const name = getNestedString(payload, 'name');
	if (!name) throw new Error('missing worktree name');

	const createdPath = createWorktree(name);
	process.stdout.write(`${createdPath}\n`);
}

main().catch(error => {
	log(`failed: ${error instanceof Error ? error.message : String(error)}`);
	process.exit(1);
});
