/**
 * `.claude/hooks/worktree-prune.ts` -- the cleanup pruner from
 * docs/agents/worktree-flow.md. Covers #1058: a registered worktree whose
 * directory is gone (`rm -rf` instead of `git worktree remove`) used to
 * crash the whole run with an unhandled `execFileSync` failure the moment
 * `runGit` tried `git -C <that path> rev-parse HEAD`, taking the report
 * for every other worktree down with it.
 *
 * The pruner resolves its own root via `findMainCheckoutRoot(import.meta.dir)`
 * (worktree-root.ts), which asks `git worktree list` from wherever the
 * running copy of the script physically sits -- so testing this for real
 * means giving it its own copy of the three hook files inside a disposable
 * git repo, not importing functions against this repo's own worktrees
 * (several other Claude Code sessions may be using those at the same
 * time). Each test builds a fresh throwaway repo with its own bare
 * "origin" remote (so `origin/trunk` resolves and `sweepOrphanBranches`'s
 * unguarded `branch --merged origin/trunk` call has something to look at)
 * and a copy of worktree-prune.ts, sync-trunk.ts, and worktree-root.ts.
 */
import { describe, expect, test } from "bun:test";
import { execFileSync, spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const REPO_ROOT = path.resolve(import.meta.dir, "..");
const HOOK_FILES = ["worktree-prune.ts", "sync-trunk.ts", "worktree-root.ts"];

function git(cwd: string, args: string[]): string {
	return execFileSync("git", ["-C", cwd, ...args], { encoding: "utf8" }).trim();
}

/** A disposable repo with its own origin remote and its own copy of the hook files. */
function makeFixture(): { root: string; cleanup: () => void } {
	const base = mkdtempSync(path.join(tmpdir(), "worktree-prune-test-"));
	const origin = path.join(base, "origin.git");
	const root = path.join(base, "fixture");
	git(base, ["init", "--bare", "-q", origin]);
	git(base, ["init", "-q", root]);
	git(root, ["config", "user.email", "test@example.com"]);
	git(root, ["config", "user.name", "Test User"]);
	git(root, ["symbolic-ref", "HEAD", "refs/heads/trunk"]);
	execFileSync("sh", ["-c", `echo hello > ${JSON.stringify(path.join(root, "README.md"))}`]);
	git(root, ["add", "README.md"]);
	git(root, ["commit", "-q", "-m", "init"]);
	git(root, ["remote", "add", "origin", origin]);
	git(root, ["push", "-q", "-u", "origin", "trunk"]);

	execFileSync("mkdir", ["-p", path.join(root, ".claude", "hooks")]);
	for (const file of HOOK_FILES) {
		execFileSync("cp", [path.join(REPO_ROOT, ".claude", "hooks", file), path.join(root, ".claude", "hooks", file)]);
	}

	return { root, cleanup: () => rmSync(base, { recursive: true, force: true }) };
}

function addWorktree(root: string, name: string, branch: string): string {
	const worktreePath = path.join(root, ".claude", "worktrees", name);
	git(root, ["worktree", "add", "-q", worktreePath, "-b", branch]);
	return worktreePath;
}

function makeStale(worktreePath: string): void {
	rmSync(worktreePath, { recursive: true, force: true });
}

/*
 * Inherits stderr (like scripts/testdb-reap.test.ts and
 * scripts/gate-bash-write.test.ts) so a genuine crash's real error text
 * surfaces in the test runner's own output instead of being swallowed --
 * the whole point of this fix is that a crash should never go
 * unexplained.
 */
function invoke(root: string, flag: "--dry-run" | "--merged"): Promise<{ exitCode: number; stdout: string }> {
	return new Promise((resolve, reject) => {
		const child = spawn("bun", [path.join(root, ".claude", "hooks", "worktree-prune.ts"), flag], {
			cwd: root,
			stdio: ["ignore", "pipe", "inherit"]
		});
		let stdout = "";
		child.stdout.on("data", chunk => {
			stdout += chunk;
		});
		child.on("error", reject);
		child.on("close", code => resolve({ exitCode: code ?? 1, stdout }));
	});
}

describe("worktree-prune (stale registration, #1058)", () => {
	test("--dry-run reports a stale worktree instead of crashing, and still reports a healthy one", async () => {
		const { root, cleanup } = makeFixture();
		try {
			const healthy = addWorktree(root, "healthy-wt", "healthy-branch");
			const stale = addWorktree(root, "stale-wt", "stale-branch");
			makeStale(stale);

			const { exitCode, stdout } = await invoke(root, "--dry-run");

			expect(exitCode).toBe(0);
			expect(stdout).toContain(`${stale}  branch=stale-branch  STALE`);
			expect(stdout).toContain(healthy);
			expect(stdout).not.toContain("execFileSync");
		} finally {
			cleanup();
		}
	});

	test("--merged clears the stale registration without the caller running `git worktree prune` by hand", async () => {
		const { root, cleanup } = makeFixture();
		try {
			const stale = addWorktree(root, "stale-wt", "stale-branch");
			makeStale(stale);

			const { exitCode, stdout } = await invoke(root, "--merged");

			expect(exitCode).toBe(0);
			expect(stdout).toContain(`${stale}  branch=stale-branch  STALE`);

			const remaining = git(root, ["worktree", "list", "--porcelain"]);
			expect(remaining).not.toContain(stale);
		} finally {
			cleanup();
		}
	});

	test("a worktree whose directory exists is never treated as stale", async () => {
		const { root, cleanup } = makeFixture();
		try {
			const healthy = addWorktree(root, "healthy-wt", "healthy-branch");

			const { exitCode, stdout } = await invoke(root, "--dry-run");

			expect(exitCode).toBe(0);
			expect(stdout).toContain(healthy);
			expect(stdout).not.toContain("STALE");
		} finally {
			cleanup();
		}
	});
});
