#!/usr/bin/env bun
/*
 * `Stop` hook: a session must not park on a worktree whose work has
 * landed.
 *
 * `worktree-flow.md` says "ExitWorktree once a PR shows MERGED is the
 * normal path", and a habit with nothing to trigger it is not a path. The
 * pruner is the disk-level backstop and runs at `SessionStart`, but it
 * cannot move a live session out of a directory it is standing in -- only
 * the session can, and only if something tells it to. This is that
 * something.
 *
 * It blocks exactly one situation: the session sits in a worktree, its
 * tree is clean, and its branch's PR reads MERGED. Anything else -- work
 * in progress, an open PR, no PR at all, `gh` unreachable -- exits 0 and
 * says nothing, so the ordinary turn is untouched. The `gh` call is
 * reached only after two local checks have already passed, which is why
 * this is not a per-turn network cost.
 */
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

function git(args: string[], cwd: string): string {
	return execFileSync('git', ['-C', cwd, ...args], {
		encoding: 'utf8',
		stdio: ['ignore', 'pipe', 'ignore']
	}).trim();
}

function inWorktree(cwd: string): boolean {
	return path.resolve(cwd).includes(`${path.sep}.claude${path.sep}worktrees${path.sep}`);
}

/*
 * One nudge per session, ever.
 *
 * A `Stop` hook that blocks is re-entered when the session stops again, so
 * an unconditional block is an infinite loop whenever the session cannot
 * act on it -- and it sometimes cannot: `ExitWorktree` only removes a
 * worktree *this* session created with `EnterWorktree`, and is a no-op on
 * one entered by path or left behind by another session. The sentinel
 * bounds the whole thing to a single message; if it is ignored, the
 * `SessionStart` pruner cleans up afterwards anyway.
 */
function alreadyNudged(sessionId: string, branch: string): boolean {
	const sentinel = path.join(
		os.tmpdir(),
		`claude-worktree-cleanup-${sessionId.replaceAll(/[^\w-]/g, '')}-${branch.replaceAll(/[^\w-]/g, '')}`
	);
	if (fs.existsSync(sentinel)) return true;
	try {
		fs.writeFileSync(sentinel, '');
	} catch {
		// cannot write a sentinel -- better to stay silent than to loop
		return true;
	}
	return false;
}

function main(): void {
	const cwd = process.cwd();
	if (!inWorktree(cwd)) return;

	let sessionId = 'unknown';
	try {
		const input = fs.readFileSync(0, 'utf8');
		sessionId = (JSON.parse(input) as { session_id?: string }).session_id ?? 'unknown';
	} catch {
		// no stdin payload -- the sentinel still bounds this to one nudge
	}

	let branch: string;
	try {
		if (git(['status', '--porcelain'], cwd).length > 0) return;
		branch = git(['rev-parse', '--abbrev-ref', 'HEAD'], cwd);
	} catch {
		return; // not a worktree we can read -- say nothing
	}
	if (!branch || branch === 'HEAD' || branch === 'trunk') return;

	/*
	 * `gh pr view <branch>` resolves by branch NAME, and a name outlives
	 * the branch that carried it. Reuse one -- start fresh work on
	 * `fix/545-...` after that PR merged -- and the old, merged PR answers
	 * for the new branch. Requiring the PR's head commit to be the one
	 * checked out here settles it: the same name on a different commit is
	 * different work, and a follow-up commit pushed on top of a merged
	 * branch means this worktree has not finished either.
	 */
	let merged: { state: string; headRefOid: string };
	try {
		const raw = execFileSync('gh', ['pr', 'view', branch, '--json', 'state,headRefOid'], {
			cwd,
			encoding: 'utf8',
			stdio: ['ignore', 'pipe', 'ignore']
		});
		merged = JSON.parse(raw) as { state: string; headRefOid: string };
	} catch {
		return; // no PR, or gh unreachable -- not this hook's business
	}
	if (merged.state !== 'MERGED') return;
	try {
		if (git(['rev-parse', 'HEAD'], cwd) !== merged.headRefOid) return;
	} catch {
		return;
	}
	if (alreadyNudged(sessionId, branch)) return;

	console.log(
		JSON.stringify({
			decision: 'block',
			reason:
				`The PR for \`${branch}\` is MERGED and this worktree is clean, but the session is still ` +
				`standing in it. Call ExitWorktree with action "remove" to delete the worktree and its ` +
				`branch and return to the main checkout. Use action "keep" instead -- and say why -- if ` +
				`the worktree still has a job: a follow-up commit on this same branch, or a stack whose ` +
				`upper layer is still open while this merged layer is checked out. This is said once ` +
				`per session; ignoring it is safe, the SessionStart pruner clears it later.`
		})
	);
}

main();
