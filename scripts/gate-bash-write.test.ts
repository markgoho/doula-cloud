/**
 * Blocking behavior for `.claude/hooks/gate-bash-write.ts` -- the
 * PreToolUse gate that blocks a Bash command writing to a tracked path
 * in the main checkout, the Bash-side counterpart to gate-worktree-edit.ts
 * (#573). See docs/agents/worktree-flow.md's Enforcement section for what
 * write patterns this recognizes and what it deliberately does not.
 */
import { describe, expect, test } from "bun:test";
import { spawn } from "node:child_process";
import path from "node:path";
import { findMainCheckoutRoot } from "../.claude/hooks/worktree-root.ts";

const REPO_ROOT = path.resolve(import.meta.dir, "..");
const SOURCE_ROOT = findMainCheckoutRoot(REPO_ROOT);
const HOOK = path.join(REPO_ROOT, ".claude", "hooks", "gate-bash-write.ts");
const TARGET = path.join(SOURCE_ROOT, "CLAUDE.md");

function invoke(command: string): Promise<{ exitCode: number; stdout: string }> {
	return new Promise((resolve, reject) => {
		const child = spawn("bun", [HOOK], { stdio: ["pipe", "pipe", "inherit"] });
		let stdout = "";
		child.stdout.on("data", chunk => {
			stdout += chunk;
		});
		child.on("error", reject);
		child.on("close", code => resolve({ exitCode: code ?? 1, stdout }));
		child.stdin.write(JSON.stringify({ tool_name: "Bash", tool_input: { command } }));
		child.stdin.end();
	});
}

describe("gate-bash-write", () => {
	test("blocks `sed -i` on a tracked file in the main checkout", async () => {
		const { exitCode, stdout } = await invoke(`sed -i 's/x/y/' ${TARGET}`);
		expect(exitCode).toBe(2);
		expect(JSON.parse(stdout).decision).toBe("block");
	});

	test("blocks shell redirection into a tracked file", async () => {
		const { exitCode } = await invoke(`echo hi > ${TARGET}`);
		expect(exitCode).toBe(2);
	});

	test("blocks `tee` writing to a tracked file after a pipe", async () => {
		const { exitCode } = await invoke(`echo hi | tee ${TARGET}`);
		expect(exitCode).toBe(2);
	});

	test("blocks `cp` whose destination is a tracked file", async () => {
		const { exitCode } = await invoke(`cp /tmp/a.txt ${TARGET}`);
		expect(exitCode).toBe(2);
	});

	test("allows redirection outside the repo", async () => {
		const { exitCode } = await invoke("echo hi > /tmp/gate-bash-write-scratch.txt");
		expect(exitCode).toBe(0);
	});

	test("allows redirection to /dev/null", async () => {
		const { exitCode } = await invoke(`npm test > /dev/null 2>&1`);
		expect(exitCode).toBe(0);
	});

	test("allows a read-only command with no recognized write pattern", async () => {
		const { exitCode, stdout } = await invoke(`cat ${TARGET}`);
		expect(exitCode).toBe(0);
		expect(stdout.trim()).toBe("");
	});

	test("allows writing into a worktree path", async () => {
		const worktreeFile = path.join(SOURCE_ROOT, ".claude", "worktrees", "some-branch", "CLAUDE.md");
		const { exitCode } = await invoke(`echo hi > ${worktreeFile}`);
		expect(exitCode).toBe(0);
	});

	test("allows writing to a gitignored file", async () => {
		const ignored = path.join(SOURCE_ROOT, "app", ".env.local");
		const { exitCode } = await invoke(`echo hi > ${ignored}`);
		expect(exitCode).toBe(0);
	});
});
