// Fast-forwards a main checkout's local `trunk` to `origin/trunk`.
// Shared by two callers: worktree-prune.ts's SessionStart `--merged` run
// (a backstop -- catches drift from a PR merged outside this session,
// e.g. through the GitHub UI, or while this session's own worktree was
// still checked out) and worktree-create.ts's ExitWorktree-removal path
// (the moment this repo's flow actually produces: PR merged, worktree
// removed, session back on trunk -- the point at which the commit that
// was just landed should already be there).
//
// Nobody commits directly to trunk under this repo's flow
// (docs/agents/worktree-flow.md) -- every landing arrives squash-merged
// through a PR -- so a clean fast-forward is the only outcome this
// expects. Anything else (HEAD not on trunk, a dirty tree, a local
// trunk carrying commits origin/trunk doesn't have) means the main
// checkout is in a state this did not anticipate, most likely someone
// working directly on trunk against convention, and it no-ops silently
// rather than guess or force anything -- the same fail-open choice
// gate-shared-index.sh makes for the same reason (worktree-flow.md's
// "A gate fails closed" section, which contrasts the two).
import { execFileSync } from 'node:child_process';

function runGit(sourceRoot: string, args: string[]): string {
	return execFileSync('git', ['-C', sourceRoot, ...args], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
}

function isDirty(sourceRoot: string): boolean {
	try {
		return runGit(sourceRoot, ['status', '--porcelain']).length > 0;
	} catch {
		return true; // fail closed -- treat "can't tell" as dirty
	}
}

/*
 * `git merge --ff-only` (not a bare `update-ref`, which would move the
 * branch tip without updating the checked-out files and index to
 * match) is what keeps this safe to call unconditionally: a no-op when
 * already current, a refusal rather than a guess the moment it isn't a
 * straight fast-forward. Logs one line when it actually advances the
 * ref; silent otherwise, including every no-op and every skipped case,
 * since both callers run unattended.
 */
export function syncTrunkToOrigin(sourceRoot: string): void {
	try {
		execFileSync('git', ['-C', sourceRoot, 'fetch', '--quiet', 'origin', 'trunk'], { stdio: ['ignore', 'pipe', 'pipe'] });
	} catch {
		return; // offline -- nothing to sync against
	}

	let head: string;
	try {
		head = runGit(sourceRoot, ['symbolic-ref', '--short', 'HEAD']);
	} catch {
		return; // detached HEAD, or HEAD unreadable -- leave it alone
	}
	if (head !== 'trunk') return; // main checkout isn't on trunk right now
	if (isDirty(sourceRoot)) return;

	const before = runGit(sourceRoot, ['rev-parse', 'trunk']);
	try {
		runGit(sourceRoot, ['merge', '--ff-only', 'origin/trunk']);
	} catch {
		// not a fast-forward -- local trunk has commits origin/trunk lacks,
		// an unexpected state for this flow; leave it for a human to see
		return;
	}
	const after = runGit(sourceRoot, ['rev-parse', 'trunk']);
	if (after !== before) console.log(`main checkout: trunk ${before.slice(0, 7)} -> ${after.slice(0, 7)}`);
}
