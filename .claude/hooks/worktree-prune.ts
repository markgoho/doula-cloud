#!/usr/bin/env bun
// The worktree pruner. Registered as a `SessionStart` hook (`--merged`),
// and runnable by hand at any time.
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

/*
 * Has this branch landed on trunk?
 *
 * Two ways, and this repo's own flow only ever produces the second. A
 * squash merge writes a NEW commit, so the branch tip never becomes an
 * ancestor of trunk and `git branch --merged` never lists it -- which
 * made this check, and therefore `--merged`, inert for every branch
 * landed the documented way. A `MERGED` PR is the authority; the
 * ancestor test still covers a branch merged or rebased by hand, and
 * costs nothing when `gh` is unreachable.
 */
function isMergedIntoTrunk(branch: string, pr: string): boolean {
	if (pr.startsWith('MERGED')) return true;
	try {
		const merged = runGit(['branch', '--merged', 'origin/trunk', '--format=%(refname:short)']);
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

/*
 * Has anything touched this worktree recently?
 *
 * Several sessions share this repo, so `--merged` at SessionStart runs
 * over worktrees other live sessions are standing in -- and a session
 * whose PR has just merged sits in a clean, landed worktree for as long
 * as it takes to write the summary. Removing that directory out from
 * under it breaks it in a way that is very hard to read from the inside.
 * A worktree touched in the last half hour is treated as somebody's, and
 * the next session's run picks it up once it has actually gone quiet.
 */
const QUIET_MS = 30 * 60 * 1000;

function recentlyTouched(worktreePath: string): boolean {
	const candidates = [worktreePath];
	try {
		const gitDir = runGit(['rev-parse', '--absolute-git-dir'], worktreePath);
		candidates.push(gitDir, path.join(gitDir, 'index'));
	} catch {
		return true; // cannot tell -- fail closed and leave it alone
	}
	const now = Date.now();
	for (const candidate of candidates) {
		try {
			if (now - fs.statSync(candidate).mtimeMs < QUIET_MS) return true;
		} catch {
			// missing path tells us nothing -- keep checking the others
		}
	}
	return false;
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
	if (merged) {
		try {
			runGit(['fetch', '--quiet', 'origin', 'trunk']);
		} catch {
			// offline -- the PR state still answers for anything landed by PR
		}
	}
	const worktrees = listWorktrees();

	if (worktrees.length === 0) {
		console.log('no worktrees under .claude/worktrees');
	}

	for (const wt of worktrees) {
		/* First, before anything else touches this worktree: `git status`
		   rewrites the index it is being measured by, so asking after the
		   dirty check makes every worktree look freshly touched forever. */
		const touched = recentlyTouched(wt.path);
		const dirty = isDirty(wt.path);
		const branch = wt.branch ?? '(detached)';
		const size = dirSize(wt.path);
		const pr = wt.branch ? prState(wt.branch) : 'no branch';
		const mergedFlag = wt.branch ? isMergedIntoTrunk(wt.branch, pr) : false;

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
		if (path.resolve(process.cwd()).startsWith(path.resolve(wt.path))) {
			console.log(`skip (this session is standing in it): ${wt.path}`);
			continue;
		}
		if (touched) {
			console.log(`skip (touched in the last 30 minutes): ${wt.path}`);
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
			/* `-d` refuses a squash-merged branch for the same reason
			   `--merged` never listed it: its commits are not on trunk. The
			   PR says the work landed, so force it -- and only then. */
			if (pr.startsWith('MERGED')) {
				try {
					runGit(['branch', '-D', wt.branch]);
				} catch {
					// already gone, or checked out elsewhere -- not fatal
				}
			}
		}
		console.log(`removed: ${wt.path} (branch ${wt.branch})`);
	}

	runGit(['worktree', 'prune']);
}

main();
