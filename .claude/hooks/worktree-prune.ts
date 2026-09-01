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
//   left alone and reported. Also fast-forwards the main checkout's own
//   local `trunk` to `origin/trunk` (see sync-trunk.ts) -- a backstop for
//   drift from a PR merged outside this session; worktree-create.ts's own
//   ExitWorktree-removal path is what actually keeps pace with this
//   session's own landings.
//
// Always ends with `git worktree prune` to drop metadata for worktrees
// whose directory is already gone.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { syncTrunkToOrigin } from './sync-trunk.ts';
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

/*
 * `onlyManaged` false includes the main checkout and any worktree made
 * outside `.claude/worktrees`, which the orphan sweep needs: a branch
 * checked out anywhere is somebody's, wherever that anywhere is.
 */
function listWorktrees(onlyManaged = true): WorktreeInfo[] {
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

	if (!onlyManaged) return entries;
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
	if (isLanded(pr)) return true;
	try {
		const merged = runGit(['branch', '--merged', 'origin/trunk', '--format=%(refname:short)']);
		return merged.split('\n').includes(branch);
	} catch {
		return false;
	}
}

/*
 * The PR for this branch, reported as "<state> <url>" -- but `MERGED`
 * only when the PR's head commit is the one this worktree has checked
 * out. `gh pr view` resolves by branch NAME, and a name outlives the
 * branch that carried it: reuse `fix/545-...` for new work and the old,
 * merged PR answers for it, which would delete a live worktree. It also
 * catches a follow-up commit pushed on top of a merged branch -- that
 * worktree has not finished either.
 */
function prState(branch: string, tip: string): string {
	try {
		const raw = execFileSync('gh', ['pr', 'view', branch, '--json', 'state,url,headRefOid'], {
			cwd: SOURCE_ROOT,
			encoding: 'utf8',
			stdio: ['ignore', 'pipe', 'ignore']
		});
		const parsed = JSON.parse(raw) as { state: string; url: string; headRefOid: string };
		if (parsed.state === 'MERGED' && tip !== parsed.headRefOid) {
			return `MERGED-ELSEWHERE ${parsed.url}`;
		}
		return `${parsed.state} ${parsed.url}`;
	} catch {
		return 'no PR';
	}
}

function isLanded(pr: string): boolean {
	return pr.split(' ')[0] === 'MERGED';
}

function tipOf(ref: string, cwd = SOURCE_ROOT): string {
	return runGit(['rev-parse', ref], cwd);
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

/*
 * Branches whose worktree is gone but which are still here.
 *
 * Removing a worktree does not take its branch with it, and neither
 * `ExitWorktree` nor `git worktree remove` can: `git branch -d` refuses a
 * squash-merged branch because its commits are not on trunk, which is the
 * same blind spot that made `--merged` inert. The worktree loop above only
 * ever sees branches that still have a worktree, so a branch orphaned by
 * an earlier removal is never collected by it and accumulates -- this
 * repo had roughly twenty when the sweep was written.
 *
 * A branch goes only on proof that nothing is lost with it:
 *
 *   - its tip is already an ancestor of `origin/trunk`, so it holds no
 *     commit trunk does not have; or
 *   - its PR is `MERGED` *at this exact tip*, which is the squash case.
 *
 * Everything else stays, and that is deliberate rather than incidental:
 * `research/baseline-521-harness` is kept on purpose by the map it
 * belongs to, and the `prototype/*` branches hold work that was never
 * pushed. None has a merged PR and none is an ancestor of trunk, so no
 * rule here can reach them.
 */
function sweepOrphanBranches(remove: boolean): void {
	const held = new Set(
		listWorktrees(false)
			.map(wt => wt.branch)
			.filter((branch): branch is string => branch !== null)
	);
	const ancestors = new Set(
		runGit(['branch', '--merged', 'origin/trunk', '--format=%(refname:short)']).split('\n')
	);
	const branches = runGit(['branch', '--format=%(refname:short)']).split('\n').filter(Boolean);

	for (const branch of branches) {
		if (branch === 'trunk' || held.has(branch)) continue;

		let why: string;
		if (ancestors.has(branch)) {
			why = 'no commits of its own';
		} else {
			/* Only branches that are NOT already on trunk need the network,
			   which keeps this to the squash-merged handful rather than one
			   `gh` call per local branch. */
			const pr = prState(branch, tipOf(branch));
			if (!isLanded(pr)) continue;
			why = pr;
		}

		if (!remove) {
			console.log(`orphan branch (would remove): ${branch}  ${why}`);
			continue;
		}
		try {
			runGit(['branch', '-D', branch]);
			console.log(`removed branch: ${branch} (${why})`);
		} catch {
			console.log(`skip (could not delete): ${branch}`);
		}
	}
}

function main(): void {
	const merged = process.argv.includes('--merged');
	if (merged) {
		// syncTrunkToOrigin does its own `fetch origin trunk` internally
		// (see sync-trunk.ts), which is also what the rest of this run
		// needs fresh for isMergedIntoTrunk/sweepOrphanBranches below.
		syncTrunkToOrigin(SOURCE_ROOT);
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
		const pr = wt.branch ? prState(wt.branch, tipOf('HEAD', wt.path)) : 'no branch';
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
			if (isLanded(pr)) {
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
	sweepOrphanBranches(merged);
}

main();
