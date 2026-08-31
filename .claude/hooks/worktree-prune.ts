#!/usr/bin/env bun
// Reusable worktree pruner -- run by hand, not registered as a hook.
//
//   bun .claude/hooks/worktree-prune.ts [--dry-run|--merged]
//
// --dry-run (default): list every worktree under .claude/worktrees with
//   its branch, PR state, dirty flag, and size. Removes nothing.
// --merged: remove a worktree only when its branch is merged into trunk
//   AND its tree is clean. A branch merged into trunk can never lose work
//   by being deleted; anything else (uncommitted changes, or committed
//   work not yet merged -- including a local-only branch never pushed) is
//   left alone and reported.
//
// Always ends with `git worktree prune` to drop metadata for worktrees
// whose directory is already gone.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { findMainCheckoutRoot } from './worktree-root.ts';

const SOURCE_ROOT = findMainCheckoutRoot(import.meta.dir);
const WORKTREES_ROOT = path.join(SOURCE_ROOT, '.claude', 'worktrees');

type WorktreeInfo = {
	path: string;
	branch: string | null;
	locked: boolean;
};

function runGit(args: string[], cwd = SOURCE_ROOT): string {
	return execFileSync('git', ['-C', cwd, ...args], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
}

function listWorktrees(): WorktreeInfo[] {
	const raw = runGit(['worktree', 'list', '--porcelain']);
	const entries: WorktreeInfo[] = [];
	let current: Partial<WorktreeInfo> | null = null;

	for (const line of raw.split('\n')) {
		if (line.startsWith('worktree ')) {
			if (current?.path) entries.push({ path: current.path, branch: current.branch ?? null, locked: current.locked ?? false });
			current = { path: line.slice('worktree '.length), branch: null, locked: false };
		} else if (line.startsWith('branch ')) {
			if (current) current.branch = line.slice('branch '.length).replace('refs/heads/', '');
		} else if (line === 'locked' || line.startsWith('locked ')) {
			if (current) current.locked = true;
		}
	}
	if (current?.path) entries.push({ path: current.path, branch: current.branch ?? null, locked: current.locked ?? false });

	return entries.filter(entry => path.resolve(entry.path).startsWith(`${WORKTREES_ROOT}${path.sep}`));
}

function isDirty(worktreePath: string): boolean {
	try {
		return runGit(['status', '--porcelain'], worktreePath).length > 0;
	} catch {
		return true; // fail closed -- treat "can't tell" as dirty
	}
}

function isMergedIntoTrunk(branch: string): boolean {
	try {
		const merged = runGit(['branch', '--merged', 'trunk', '--format=%(refname:short)']);
		return merged.split('\n').includes(branch);
	} catch {
		return false;
	}
}

function prState(branch: string): string {
	try {
		const raw = execFileSync('gh', ['pr', 'view', branch, '--json', 'state,url'], {
			cwd: SOURCE_ROOT,
			encoding: 'utf8',
			stdio: ['ignore', 'pipe', 'ignore']
		});
		const parsed = JSON.parse(raw) as { state: string; url: string };
		return `${parsed.state} ${parsed.url}`;
	} catch {
		return 'no PR';
	}
}

function dirSize(worktreePath: string): string {
	try {
		return execFileSync('du', ['-sh', worktreePath], { encoding: 'utf8' }).split('\t')[0]?.trim() ?? '?';
	} catch {
		return '?';
	}
}

function main(): void {
	const merged = process.argv.includes('--merged');
	const worktrees = listWorktrees();

	if (worktrees.length === 0) {
		console.log('no worktrees under .claude/worktrees');
	}

	for (const wt of worktrees) {
		const dirty = isDirty(wt.path);
		const branch = wt.branch ?? '(detached)';
		const size = dirSize(wt.path);
		const pr = wt.branch ? prState(wt.branch) : 'no branch';
		const mergedFlag = wt.branch ? isMergedIntoTrunk(wt.branch) : false;

		if (!merged) {
			console.log(
				`${wt.path}  branch=${branch}  locked=${wt.locked}  dirty=${dirty}  merged=${mergedFlag}  pr=${pr}  size=${size}`
			);
			continue;
		}

		if (wt.locked) {
			console.log(`skip (locked): ${wt.path}`);
			continue;
		}
		if (dirty) {
			console.log(`skip (dirty): ${wt.path}`);
			continue;
		}
		if (!wt.branch || !mergedFlag) {
			console.log(`skip (not merged into trunk): ${wt.path}`);
			continue;
		}

		try {
			runGit(['worktree', 'remove', wt.path]);
		} catch {
			runGit(['worktree', 'remove', '--force', wt.path]);
		}
		try {
			runGit(['branch', '-d', wt.branch]);
		} catch {
			// remote-tracking or already-gone branch -- not fatal
		}
		console.log(`removed: ${wt.path} (branch ${wt.branch})`);
	}

	runGit(['worktree', 'prune']);
}

main();
