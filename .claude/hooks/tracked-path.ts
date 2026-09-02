// Shared by gate-worktree-edit.ts and gate-bash-write.ts: reading a
// PreToolUse hook's stdin payload, and the worktree-boundary check both
// gates enforce identically (#573).
import { execFileSync } from 'node:child_process';
import path from 'node:path';

export function readStdin(): Promise<string> {
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

export function isGitIgnored(sourceRoot: string, filePath: string): boolean {
	try {
		execFileSync('git', ['-C', sourceRoot, 'check-ignore', '-q', filePath], {
			stdio: ['ignore', 'ignore', 'ignore']
		});
		return true;
	} catch {
		return false;
	}
}

// True if `filePath` is a tracked path in the main checkout: not already
// under `.claude/worktrees/`, inside the checkout, and not gitignored.
export function isTrackedInMainCheckout(filePath: string, sourceRoot: string, worktreesRoot: string): boolean {
	const resolvedFile = path.resolve(filePath);

	if (resolvedFile.startsWith(worktreesRoot + path.sep)) return false;
	if (!resolvedFile.startsWith(sourceRoot + path.sep)) return false;
	if (isGitIgnored(sourceRoot, resolvedFile)) return false;

	return true;
}
