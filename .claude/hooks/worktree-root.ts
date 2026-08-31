// Finds the main checkout's absolute path -- correctly, regardless of
// which checkout's own copy of a hook script is executing.
//
// Every worktree carries an identical copy of every hook file (they're
// tracked, committed files), so a hook can never trust ITS OWN location
// (import.meta.dir) to mean "the main checkout": running from inside a
// worktree, that resolves to the worktree itself, not main. Confirmed
// empirically: with that approach, gate-worktree-edit.ts blocked edits
// INSIDE a worktree and allowed them in the real main checkout --
// backwards -- because it treated whichever checkout it happened to run
// from as "home".
//
// `git worktree list` is shared repo-wide (git tracks it centrally in
// .git/worktrees, not per-checkout) and always lists the primary/main
// checkout first, regardless of which checkout the command runs from --
// so it's the one reliable way to ask "where is main" from anywhere in
// the repo.
import { execFileSync } from 'node:child_process';

export function findMainCheckoutRoot(cwd: string): string {
	const output = execFileSync('git', ['-C', cwd, 'worktree', 'list', '--porcelain'], {
		encoding: 'utf8',
		stdio: ['ignore', 'pipe', 'pipe']
	});
	const firstLine = output.split('\n').find(line => line.startsWith('worktree '));
	if (!firstLine) {
		throw new Error('could not determine the main checkout root from `git worktree list`');
	}
	return firstLine.slice('worktree '.length).trim();
}
