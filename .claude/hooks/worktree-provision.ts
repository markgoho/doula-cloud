#!/usr/bin/env bun
// Shared provisioning logic for a doula-cloud worktree. Called from two
// places: worktree-create.ts (the WorktreeCreate hook, for EnterWorktree)
// and the PostToolUse Bash fallback matching `git worktree add` (so a
// worktree created by hand gets the same treatment). Every step here is
// idempotent -- safe to re-run on a worktree that already exists, which is
// exactly what the fallback path does on every `git worktree add`.
//
// Modeled on cx-platform-ui's cx-platform-worktree-create.ts /
// cx-platform-post-tool-use.ts, but kept in-repo (not
// ~/.claude/hooks/) so it is portable and reviewable rather than pinned to
// one machine's absolute paths.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { findMainCheckoutRoot } from './worktree-root.ts';

const SOURCE_ROOT = findMainCheckoutRoot(import.meta.dir);
const SOURCE_APP_ENV_LOCAL = path.join(SOURCE_ROOT, 'app', '.env.local');
const SOURCE_NODE_MODULES = path.join(SOURCE_ROOT, 'node_modules');
const WORKTREES_ROOT = path.join(SOURCE_ROOT, '.claude', 'worktrees');
const MAX_PORT_OFFSET = 9;

function log(message: string): void {
	process.stderr.write(`worktree-provision: ${message}\n`);
}

function runGit(args: string[], cwd = SOURCE_ROOT): string {
	return execFileSync('git', ['-C', cwd, ...args], {
		encoding: 'utf8',
		stdio: ['ignore', 'pipe', 'pipe']
	}).trim();
}

function ensureAppEnvLocal(worktreePath: string): string | null {
	if (!fs.existsSync(SOURCE_APP_ENV_LOCAL)) {
		return 'app/.env.local missing from main checkout -- see docs/environment.md (scripts/stripe-setup.sh)';
	}
	const target = path.join(worktreePath, 'app', '.env.local');
	if (fs.existsSync(target)) return 'left existing app/.env.local';
	fs.mkdirSync(path.dirname(target), { recursive: true });
	fs.copyFileSync(SOURCE_APP_ENV_LOCAL, target);
	return 'copied app/.env.local';
}

// A symlink pointing anywhere other than the canonical source path --
// including the SAME directory via a different path case (e.g.
// /Users/mgoho/github vs /Users/mgoho/Github on case-insensitive macOS) --
// breaks Vitest/Vite realpath-based module resolution. Repoint it. Safe:
// unlinking a symlink never touches the real node_modules it pointed at.
function ensureSymlink(target: string, source: string): string {
	if (!fs.existsSync(source)) return `left node_modules unlinked (no ${source})`;

	if (!fs.existsSync(target)) {
		fs.symlinkSync(source, target, 'junction');
		return 'symlinked node_modules';
	}

	try {
		const stat = fs.lstatSync(target);
		if (stat.isSymbolicLink()) {
			const current = fs.readlinkSync(target);
			if (current === source) return 'left existing node_modules symlink';
			fs.unlinkSync(target);
			fs.symlinkSync(source, target, 'junction');
			return `repaired node_modules symlink (was ${current})`;
		}
	} catch {
		// fall through
	}

	return 'left existing node_modules (real directory)';
}

// If the worktree's branch touches a dependency manifest relative to
// trunk, a live symlink into main's node_modules would be wrong for it --
// remove the symlink and do a real install instead. Re-checked on every
// provision (not just create), so a branch that adds a dependency later
// gets re-provisioned on its next `git worktree add` / EnterWorktree.
function dependencyManifestsChanged(worktreePath: string): boolean {
	try {
		const diff = runGit(
			['diff', '--name-only', 'trunk...HEAD', '--', 'package.json', 'bun.lock', 'app/package.json', 'app/bun.lock'],
			worktreePath
		);
		return diff.trim().length > 0;
	} catch {
		return false;
	}
}

function installReal(worktreePath: string, subdir: string, reason: string, messages: string[]): void {
	const target = path.join(worktreePath, subdir);
	try {
		if (fs.lstatSync(target).isSymbolicLink()) fs.unlinkSync(target);
	} catch {
		// nothing to unlink
	}
	// Never run bun install through a live symlink -- it would mutate
	// main's node_modules for every worktree sharing it at once.
	execFileSync('bun', ['install'], {
		cwd: subdir === 'app' ? path.join(worktreePath, 'app') : worktreePath,
		stdio: 'inherit'
	});
	messages.push(`${subdir === 'app' ? 'app/' : ''}node_modules: real bun install (${reason})`);
}

// app/node_modules is ALWAYS a real install, never symlinked -- unlike
// root. SvelteKit 3 writes generated per-checkout state into it
// (node_modules/$app/tsconfig.json, node_modules/$app/types; Vite's
// .vite-temp/ too), and TypeScript resolves that tsconfig's own location
// via its REAL path before applying its `rootDirs` entries. Through a
// symlink, every worktree's rootDirs collapses onto whichever checkout
// app/node_modules physically lives in, breaking `./$types` imports
// worktree-wide -- confirmed empirically: reproduces on a bare trunk
// checkout with zero other changes, in every symlinked worktree, absent
// under a real install (which is what CI already does, and CI is green).
// A live symlink here is also a write hazard independent of that bug:
// svelte-kit sync mutates node_modules/$app/* in place, so two worktrees
// sharing a symlinked app/node_modules would clobber each other's
// generated state, not just serve a stale read.
function ensureAppNodeModulesReal(worktreePath: string, changed: boolean, messages: string[]): void {
	const target = path.join(worktreePath, 'app', 'node_modules');
	let isSymlink = false;
	try {
		isSymlink = fs.lstatSync(target).isSymbolicLink();
	} catch {
		// doesn't exist yet -- installReal below creates it
	}
	if (fs.existsSync(target) && !isSymlink && !changed) {
		messages.push('left existing app/node_modules (real install)');
		return;
	}
	installReal(worktreePath, 'app', changed ? 'dependency manifest changed' : 'always real, see comment', messages);
}

function ensureNodeModules(worktreePath: string, messages: string[]): void {
	const changed = dependencyManifestsChanged(worktreePath);

	// Root: nothing generates per-checkout state into it, so a symlink is
	// safe and saves the disk -- unless this branch's own dependencies
	// changed, in which case it needs its own real install.
	if (changed) {
		installReal(worktreePath, '.', 'dependency manifest changed', messages);
	} else {
		messages.push(ensureSymlink(path.join(worktreePath, 'node_modules'), SOURCE_NODE_MODULES));
	}

	ensureAppNodeModulesReal(worktreePath, changed, messages);
}

function livePortOffsets(): Set<number> {
	const claimed = new Set<number>();
	let entries: string[] = [];
	try {
		entries = fs.readdirSync(WORKTREES_ROOT);
	} catch {
		return claimed;
	}
	for (const entry of entries) {
		const offsetFile = path.join(WORKTREES_ROOT, entry, '.port-offset');
		try {
			const value = Number.parseInt(fs.readFileSync(offsetFile, 'utf8').trim(), 10);
			if (Number.isInteger(value)) claimed.add(value);
		} catch {
			// no offset assigned yet -- doesn't claim anything
		}
	}
	return claimed;
}

function ensurePortOffset(worktreePath: string, messages: string[]): void {
	const offsetFile = path.join(worktreePath, '.port-offset');
	if (fs.existsSync(offsetFile)) {
		messages.push(`left existing port offset (${fs.readFileSync(offsetFile, 'utf8').trim()})`);
		return;
	}

	const claimed = livePortOffsets();
	let offset = 1;
	while (claimed.has(offset) && offset <= MAX_PORT_OFFSET) offset++;
	if (offset > MAX_PORT_OFFSET) {
		throw new Error(
			`all ${MAX_PORT_OFFSET} worktree port offsets are in use -- prune stale worktrees ` +
				`(worktree-prune.ts --dry-run) before creating another one`
		);
	}
	fs.writeFileSync(offsetFile, `${offset}\n`);
	messages.push(`assigned port offset ${offset}`);
}

export function provisionWorktree(worktreePath: string): void {
	const messages: string[] = [];
	const envMessage = ensureAppEnvLocal(worktreePath);
	if (envMessage) messages.push(envMessage);
	ensureNodeModules(worktreePath, messages);
	ensurePortOffset(worktreePath, messages);
	log(messages.join('; '));
}

export const WORKTREES_ROOT_PATH = WORKTREES_ROOT;
export const SOURCE_ROOT_PATH = SOURCE_ROOT;

// Below: standalone entry point when this file is run directly as a
// PostToolUse hook on Bash -- the fallback for a worktree created by hand
// (`git worktree add ...`) rather than through EnterWorktree, so it gets
// the same env/node_modules/port-offset treatment. worktree-create.ts
// (the WorktreeCreate hook) imports provisionWorktree() above instead of
// running this entry point.
function extractWorktreeAddPath(command: string): string | null {
	const tokens = command.trim().split(/\s+/);
	let i = 0;
	while (i < tokens.length && tokens[i] !== 'worktree') i++;
	i++; // skip 'worktree'
	while (i < tokens.length && tokens[i] !== 'add') i++;
	i++; // skip 'add'

	const flagsWithValue = new Set(['-b', '-B', '--reason', '--lock-reason']);
	while (i < tokens.length) {
		const token = tokens[i];
		if (flagsWithValue.has(token)) {
			i += 2;
		} else if (token.startsWith('-')) {
			i++;
		} else {
			return token;
		}
	}
	return null;
}

async function readStdin(): Promise<string> {
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

async function runAsPostToolUseHook(): Promise<void> {
	const raw = await readStdin();
	let payload: Record<string, unknown> = {};
	if (raw.trim()) {
		try {
			payload = JSON.parse(raw);
		} catch {
			process.exit(0);
		}
	}

	if (payload['tool_name'] !== 'Bash') process.exit(0);
	const toolInput = payload['tool_input'];
	if (!toolInput || typeof toolInput !== 'object') process.exit(0);
	const command = (toolInput as Record<string, unknown>)['command'];
	if (typeof command !== 'string' || !/\bgit\b[\s\S]*\bworktree\s+add\b/.test(command)) process.exit(0);

	const rawPath = extractWorktreeAddPath(command);
	if (!rawPath) {
		log('could not extract worktree path from command');
		process.exit(0);
	}
	const worktreePath = path.isAbsolute(rawPath) ? rawPath : path.resolve(SOURCE_ROOT, rawPath);
	if (!fs.existsSync(worktreePath)) {
		log(`worktree path not found: ${worktreePath}`);
		process.exit(0);
	}

	provisionWorktree(worktreePath);
}

if (import.meta.main) {
	runAsPostToolUseHook().catch(error => {
		log(`PostToolUse fallback failed: ${error instanceof Error ? error.message : String(error)}`);
		process.exit(0);
	});
}
