/**
 * Every hook command in `.claude/settings.json` must find its script from
 * any working directory (#569).
 *
 * A hook command inherits the session's cwd, and this repo makes `app/`
 * the ordinary place to stand -- `bun run check`, `bun run lint` and
 * every component live there. Registered with a relative path, a hook
 * resolved against `app/`, found no `.claude/` and died with `Module not
 * found`. That is not a loud failure: only exit code 2 blocks, so every
 * other non-zero code is a non-blocking error and the action proceeds.
 * The two gates therefore permitted exactly what they exist to refuse,
 * and a line in the transcript was the only sign.
 *
 * The path is anchored with `git rev-parse --show-toplevel` rather than
 * `$CLAUDE_PROJECT_DIR`, which the hooks documentation recommends for
 * this symptom. Measured from a `Stop` hook while the session ran inside
 * `.claude/worktrees/fix-569-hook-paths`:
 *
 *   pwd=/Users/mgoho/Github/doula-cloud/.claude/worktrees/fix-569-hook-paths
 *   CLAUDE_PROJECT_DIR=/Users/mgoho/Github/doula-cloud
 *
 * The variable is set, and it names the MAIN checkout while the session
 * is in a worktree -- so it would have run main's copy of a hook against
 * a worktree session, which is the self-location bug #555 already fixed
 * once. `--show-toplevel` names the checkout the session is actually in,
 * whose own committed copy is the one that should run.
 */

import { describe, expect, test } from "bun:test";
import { execFile } from "node:child_process";
import { readFile, stat } from "node:fs/promises";
import path from "node:path";
import { promisify } from "node:util";

const run = promisify(execFile);

interface HookEntry {
	type?: string;
	command?: string;
}
interface HookMatcher {
	hooks?: HookEntry[];
}

const REPO_ROOT = path.resolve(import.meta.dir, "..");

async function registeredCommands(): Promise<{ event: string; command: string }[]> {
	const raw = await readFile(path.join(REPO_ROOT, ".claude", "settings.json"), "utf8");
	const settings = JSON.parse(raw) as { hooks?: Record<string, HookMatcher[]> };
	return Object.entries(settings.hooks ?? {}).flatMap(([event, matchers]) =>
		matchers.flatMap((matcher) =>
			(matcher.hooks ?? [])
				.map((hook) => hook.command)
				.filter((command): command is string => typeof command === "string")
				.map((command) => ({ event, command }))
		)
	);
}

/**
 * The quoted path argument -- the only part of a hook command that has to
 * resolve to a file. Everything else is the runner and its flags.
 */
function scriptExpression(command: string): string | undefined {
	return /"([^"]*\.claude\/hooks\/[^"]*)"/.exec(command)?.[1];
}

describe("hook registration", () => {
	test("at least one hook is registered", async () => {
		expect((await registeredCommands()).length).toBeGreaterThan(0);
	});

	test("every hook command names its script inside .claude/hooks", async () => {
		const missing = (await registeredCommands())
			.filter(({ command }) => scriptExpression(command) === undefined)
			.map(({ event, command }) => `${event}: ${command}`);
		expect(missing).toEqual([]);
	});

	test("no hook command uses a path relative to the working directory", async () => {
		const relative = (await registeredCommands())
			.filter(({ command }) => /(^|\s)(bun|bash|sh|node)\s+\.claude\//.test(command))
			.map(({ event, command }) => `${event}: ${command}`);
		expect(relative).toEqual([]);
	});

	/*
	 * The real assertion, and the one that would have caught this: resolve
	 * each command's path expression in a shell whose cwd is `app/` -- the
	 * directory the failure was recorded in -- and require a file there.
	 * `sh -c` is what the runtime itself spawns for a command with no
	 * `args` key, so the expansion under test is the one that ships.
	 */
	test("every hook script resolves from app/, not just from the repo root", async () => {
		const commands = await registeredCommands();
		const unresolved: string[] = [];

		for (const { event, command } of commands) {
			const expression = scriptExpression(command);
			if (expression === undefined) continue;
			const { stdout } = await run("sh", ["-c", `printf %s "${expression}"`], {
				cwd: path.join(REPO_ROOT, "app")
			});
			const resolved = stdout.trim();
			if (!path.isAbsolute(resolved)) {
				unresolved.push(`${event}: resolved to a relative path -- ${resolved}`);
				continue;
			}
			const found = await stat(resolved).then(
				(entry) => entry.isFile(),
				() => false
			);
			if (!found) unresolved.push(`${event}: no file at ${resolved}`);
		}

		expect(unresolved).toEqual([]);
	});

	/*
	 * Resolution has to name THIS checkout. Every worktree carries its own
	 * committed copy of every hook file, so a hook that resolved to the
	 * main checkout would run main's code against a worktree session --
	 * the inversion #555 recorded, where the edit gate blocked edits
	 * inside worktrees and allowed them in main.
	 */
	test("resolution follows the checkout the command runs in", async () => {
		const commands = await registeredCommands();
		const expression = scriptExpression(commands[0]!.command);
		expect(expression).toBeDefined();

		const { stdout } = await run("sh", ["-c", `printf %s "${expression}"`], {
			cwd: path.join(REPO_ROOT, "app")
		});
		expect(stdout.trim().startsWith(REPO_ROOT + path.sep)).toBe(true);
	});
});
