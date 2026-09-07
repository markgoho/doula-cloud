/**
 * `.claude/hooks/testdb-reap.ts` -- the SessionStart hook that reaps
 * orphaned testcontainers Postgres containers a killed `go test` process
 * leaves behind (#889). See that file's header and docs/testing.md's
 * "Reaping orphaned testcontainers" section for the full story.
 *
 * The reap decision (parseContainers/pickReapCandidates) is pure, so it's
 * imported and unit tested directly. main()'s fail-open behavior --
 * DOCKER_HOST unset, engine unreachable -- is exercised as a subprocess,
 * the same way scripts/gate-bash-write.test.ts covers gate-bash-write.ts.
 */
import { describe, expect, test } from "bun:test";
import { spawn } from "node:child_process";
import path from "node:path";
import { parseContainers, pickReapCandidates, REAP_THRESHOLD_MS, type ReapCandidate } from "../.claude/hooks/testdb-reap.ts";

const REPO_ROOT = path.resolve(import.meta.dir, "..");
const HOOK = path.join(REPO_ROOT, ".claude", "hooks", "testdb-reap.ts");

function invoke(env: Record<string, string | undefined>): Promise<{ exitCode: number; stdout: string }> {
	return new Promise((resolve, reject) => {
		const child = spawn("bun", [HOOK], { stdio: ["ignore", "pipe", "inherit"], env });
		let stdout = "";
		child.stdout.on("data", chunk => {
			stdout += chunk;
		});
		child.on("error", reject);
		child.on("close", code => resolve({ exitCode: code ?? 1, stdout }));
	});
}

function containerLabeled(labels: Record<string, string>, ageMs: number, nowMs: number): ReapCandidate {
	return { id: "abc123", name: "some_container", labels, createdAtMs: nowMs - ageMs };
}

describe("pickReapCandidates", () => {
	const now = Date.now();
	const labeled = { "org.testcontainers": "true", "org.testcontainers.lang": "go" };

	test("does not reap a container younger than the threshold", () => {
		const container = containerLabeled(labeled, REAP_THRESHOLD_MS - 1000, now);
		expect(pickReapCandidates([container], now)).toEqual([]);
	});

	test("reaps a labeled container older than the threshold", () => {
		const container = containerLabeled(labeled, REAP_THRESHOLD_MS + 1000, now);
		expect(pickReapCandidates([container], now)).toEqual([container]);
	});

	test("never reaps a container without the testcontainers label, however old", () => {
		const container = containerLabeled({}, REAP_THRESHOLD_MS * 100, now);
		expect(pickReapCandidates([container], now)).toEqual([]);
	});

	test("never reaps a container whose testcontainers label is not exactly \"true\"", () => {
		const container = containerLabeled({ "org.testcontainers": "false" }, REAP_THRESHOLD_MS * 100, now);
		expect(pickReapCandidates([container], now)).toEqual([]);
	});

	test("does not reap a container exactly at the threshold", () => {
		const container = containerLabeled(labeled, REAP_THRESHOLD_MS, now);
		expect(pickReapCandidates([container], now)).toEqual([]);
	});

	test("accepts a custom threshold", () => {
		const container = containerLabeled(labeled, 5000, now);
		expect(pickReapCandidates([container], now, 1000)).toEqual([container]);
	});

	test("only reaps the containers that qualify out of a mixed list", () => {
		const young = containerLabeled(labeled, 1000, now);
		const old = containerLabeled(labeled, REAP_THRESHOLD_MS + 1000, now);
		const unlabeled = containerLabeled({}, REAP_THRESHOLD_MS + 1000, now);
		expect(pickReapCandidates([young, old, unlabeled], now)).toEqual([old]);
	});
});

describe("parseContainers", () => {
	test("maps an engine ps --format json array into ReapCandidates", () => {
		const json = JSON.stringify([
			{
				Id: "deadbeef",
				Names: ["clever_name"],
				Labels: { "org.testcontainers": "true" },
				Created: 1000
			}
		]);
		expect(parseContainers(json)).toEqual([
			{ id: "deadbeef", name: "clever_name", labels: { "org.testcontainers": "true" }, createdAtMs: 1_000_000 }
		]);
	});

	test("falls back to the id when Names is absent, and to an empty label set when Labels is absent", () => {
		const json = JSON.stringify([{ Id: "deadbeef", Created: 0 }]);
		expect(parseContainers(json)).toEqual([{ id: "deadbeef", name: "deadbeef", labels: {}, createdAtMs: 0 }]);
	});
});

describe("testdb-reap hook (subprocess, fail-open behavior)", () => {
	test("does nothing when DOCKER_HOST is unset", async () => {
		const env = { ...process.env };
		delete env.DOCKER_HOST;
		const { exitCode, stdout } = await invoke(env);
		expect(exitCode).toBe(0);
		expect(stdout).toBe("");
	});

	test("fails open when the container engine is unreachable", async () => {
		const { exitCode, stdout } = await invoke({
			...process.env,
			DOCKER_HOST: "unix:///nonexistent/testdb-reap-test.sock",
			CONTAINER_ENGINE: "/nonexistent-binary-testdb-reap-test"
		});
		expect(exitCode).toBe(0);
		expect(stdout).toBe("");
	});
});
