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

function invoke(command: string, options: { cwd?: string } = {}): Promise<{ exitCode: number; stdout: string }> {
	return new Promise((resolve, reject) => {
		const child = spawn("bun", [HOOK], { stdio: ["pipe", "pipe", "inherit"], cwd: options.cwd });
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

	test("blocks a write verb chained after `&&`", async () => {
		const { exitCode } = await invoke(`true && sed -i 's/x/y/' ${TARGET}`);
		expect(exitCode).toBe(2);
	});

	test("blocks a write verb chained after `&`", async () => {
		const { exitCode } = await invoke(`true & cp /tmp/a.txt ${TARGET}`);
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

	// #702: the hook sees the raw command before the shell expands `$S`, so
	// the literal text `$S/c298.md` resolves as a relative path against the
	// hook's own cwd. Run these with cwd set to the main checkout -- the
	// exact scenario from #702, a session standing in main rather than a
	// worktree -- so `path.resolve` pins the target inside the checkout,
	// the same way it did for the reported bug. Before the fix,
	// `isTrackedInMainCheckout` then called that nonexistent path tracked;
	// the fix skips any target still holding a shell variable or command
	// substitution before it gets that far.
	test("allows a redirect target held in a shell variable ($VAR form)", async () => {
		const { exitCode } = await invoke("printf hi >> $S/c298.md", { cwd: SOURCE_ROOT });
		expect(exitCode).toBe(0);
	});

	test("allows a redirect target held in a shell variable (${VAR} form)", async () => {
		const { exitCode } = await invoke("printf hi >> ${S}/c298.md", { cwd: SOURCE_ROOT });
		expect(exitCode).toBe(0);
	});

	test("allows a redirect target held in a command substitution", async () => {
		const { exitCode } = await invoke("printf hi >> $(dirname /tmp/x)/c298.md", { cwd: SOURCE_ROOT });
		expect(exitCode).toBe(0);
	});

	test("still blocks a literal redirection to a tracked path (true positive unchanged)", async () => {
		const { exitCode } = await invoke(`echo hi >> ${TARGET}`);
		expect(exitCode).toBe(2);
	});

	test("still blocks a literal redirection to a quoted tracked path", async () => {
		const { exitCode } = await invoke(`echo hi > "${TARGET}"`);
		expect(exitCode).toBe(2);
	});

	// #677: a literal `>` sitting in prose (an HTML tag, a CSS combinator, an
	// arrow, a markdown blockquote marker) was read as a redirection
	// operator and the next word became a fabricated write target. Run with
	// cwd: SOURCE_ROOT, the same main-checkout scenario #702's tests above
	// simulate -- without it, the fabricated target would resolve under the
	// worktree and be excluded for an unrelated reason, hiding whether the
	// quote/heredoc fix actually worked.
	describe("#677: prose containing a literal '>' is not read as redirection", () => {
		test("an HTML tag in a commit message (#451 case 1)", async () => {
			const { exitCode } = await invoke('git commit -m "Add <Badge> next to price"', { cwd: SOURCE_ROOT });
			expect(exitCode).toBe(0);
		});

		test("a CSS child combinator in backticks inside a heredoc body (#451 case 2)", async () => {
			const command = [
				"gh issue edit 451 --body-file - <<'EOF'",
				"the rule is `.stack-l > * + *`",
				'EOF'
			].join('\n');
			const { exitCode } = await invoke(command, { cwd: SOURCE_ROOT });
			expect(exitCode).toBe(0);
		});

		test("a markdown blockquote marker opening a heredoc body", async () => {
			const command = [
				"gh issue comment 677 --body-file - <<'EOF'",
				'> *This was generated by AI during triage.*',
				'',
				'Some triage prose.',
				'EOF'
			].join('\n');
			const { exitCode } = await invoke(command, { cwd: SOURCE_ROOT });
			expect(exitCode).toBe(0);
		});

		test("an ASCII arrow inside a double-quoted echo argument", async () => {
			const { exitCode } = await invoke('echo "1085 -> Ready for human"', { cwd: SOURCE_ROOT });
			expect(exitCode).toBe(0);
		});
	});

	// #677 AC1: these gh subcommands never write to a local tracked path, so
	// nothing in their --body/--title/-m value should ever trigger a block.
	describe("#677: gh issue/pr write-invocations are exempt regardless of body content", () => {
		test("gh issue create with a prose-only body", async () => {
			const { exitCode } = await invoke('gh issue create --title "x" --body "just prose, no redirection"', {
				cwd: SOURCE_ROOT
			});
			expect(exitCode).toBe(0);
		});

		test("gh issue edit with an arrow in the body", async () => {
			const { exitCode } = await invoke('gh issue edit 677 --body "1085 -> Ready for human"', {
				cwd: SOURCE_ROOT
			});
			expect(exitCode).toBe(0);
		});

		test("gh issue comment with an arrow in the body", async () => {
			const { exitCode } = await invoke('gh issue comment 677 --body "1085 -> Ready for human"', {
				cwd: SOURCE_ROOT
			});
			expect(exitCode).toBe(0);
		});

		test("gh pr create with an HTML tag in the body", async () => {
			const { exitCode } = await invoke('gh pr create --title "x" --body "Add <Badge> next to price"', {
				cwd: SOURCE_ROOT
			});
			expect(exitCode).toBe(0);
		});

		test("gh pr edit with an HTML tag in the body", async () => {
			const { exitCode } = await invoke('gh pr edit 1 --body "Add <Badge> next to price"', {
				cwd: SOURCE_ROOT
			});
			expect(exitCode).toBe(0);
		});

		// AC1 only excuses characters *inside* the body/title/-m value, not the
		// whole invocation -- a real trailing redirect on the same command line
		// is a genuine write and must still block, whatever gh subcommand
		// precedes it.
		test("still blocks a real trailing redirect after a gh issue create call", async () => {
			const { exitCode } = await invoke(`gh issue create --title "x" --body "y" > ${TARGET}`, {
				cwd: SOURCE_ROOT
			});
			expect(exitCode).toBe(2);
		});
	});
});
