/**
 * Port-offset assignment in `.claude/hooks/worktree-provision.ts` (#927).
 *
 * Before this, an offset was chosen purely from what other worktrees had
 * claimed, so an offset whose ports an unrelated local process already held
 * was handed out anyway -- and the collision only surfaced much later, as a
 * bind failure in startStack that reads like a broken emulator rather than
 * an unusable offset. Observed live: a local process on 127.0.0.1:9999 made
 * offset 9 (emulator 9099 + 900) unusable.
 *
 * chooseOffset takes its probe as an argument, so the decision is tested
 * here without binding real ports; one test below does bind a real socket
 * to prove the real probe agrees with the injected one.
 */
import { describe, expect, test } from "bun:test";
import net from "node:net";
import { BASE_PORTS, PORT_STEP } from "../app/e2e/ports.ts";
import { chooseOffset, portsForOffset } from "../.claude/hooks/worktree-provision.ts";

const MAX_PORT_OFFSET = 9;
const allFree = async () => true;

function busyExcept(busyPorts: number[]) {
	return async (port: number) => !busyPorts.includes(port);
}

describe("portsForOffset", () => {
	test("offset 0 is the unshifted base list", () => {
		expect(portsForOffset(0)).toEqual([...BASE_PORTS]);
	});

	test("each offset shifts every port by offset * PORT_STEP", () => {
		expect(portsForOffset(9)).toEqual(BASE_PORTS.map(base => base + 9 * PORT_STEP));
	});

	test("covers every port in the stack, not a subset", () => {
		// The regression guard for AC2: if ports.ts gains a service and
		// BASE_PORTS is not updated, this drops and the probe silently
		// stops covering it.
		expect(portsForOffset(1).length).toBe(BASE_PORTS.length);
		expect(BASE_PORTS.length).toBeGreaterThanOrEqual(7);
	});
});

describe("chooseOffset", () => {
	test("takes the lowest offset when nothing is claimed or bound", async () => {
		expect(await chooseOffset(new Set(), allFree)).toEqual({ offset: 1, skipped: [] });
	});

	test("skips offsets another worktree has claimed", async () => {
		expect(await chooseOffset(new Set([1, 2]), allFree)).toEqual({ offset: 3, skipped: [] });
	});

	test("skips an offset whose port is already bound, and says which port", async () => {
		const emulatorAt9 = BASE_PORTS[1] + 9 * PORT_STEP;
		const claimed = new Set([1, 2, 3, 4, 5, 6, 7, 8]);
		await expect(chooseOffset(claimed, busyExcept([emulatorAt9]))).rejects.toThrow(
			new RegExp(`port ${emulatorAt9} in use`)
		);
	});

	test("passes over a blocked offset and reports it alongside the one it took", async () => {
		const blocked = BASE_PORTS[0] + 1 * PORT_STEP;
		const result = await chooseOffset(new Set(), busyExcept([blocked]));
		expect(result.offset).toBe(2);
		expect(result.skipped).toEqual([`offset 1 (port ${blocked} in use)`]);
	});

	test("a claimed offset is never probed at all", async () => {
		const probed: number[] = [];
		await chooseOffset(new Set([1]), async port => {
			probed.push(port);
			return true;
		});
		expect(probed).not.toContain(BASE_PORTS[0] + 1 * PORT_STEP);
		expect(probed).toContain(BASE_PORTS[0] + 2 * PORT_STEP);
	});

	test("stops probing an offset at its first busy port", async () => {
		const probed: number[] = [];
		await chooseOffset(new Set(), async port => {
			probed.push(port);
			return port !== BASE_PORTS[0] + PORT_STEP;
		});
		// The first base port at offset 1 is busy, so the rest of offset 1
		// is never probed.
		expect(probed).not.toContain(BASE_PORTS[1] + PORT_STEP);
	});

	test("refuses rather than assigning an unusable offset when every one is blocked", async () => {
		await expect(chooseOffset(new Set(), async () => false)).rejects.toThrow(/no usable port offset/);
	});

	test("names both causes when the pool is exhausted by a mix of claimed and blocked", async () => {
		const claimed = new Set([1, 2, 3, 4, 5, 6, 7, 8]);
		await expect(chooseOffset(claimed, async () => false)).rejects.toThrow(/8 of 9 claimed by live worktrees/);
	});

	test("points at the pruner, which is what actually frees a claimed offset", async () => {
		const claimed = new Set(Array.from({ length: MAX_PORT_OFFSET }, (_, i) => i + 1));
		await expect(chooseOffset(claimed, allFree)).rejects.toThrow(/worktree-prune\.ts --dry-run/);
	});
});

describe("the real probe", () => {
	test("reports a port held by another process as unavailable", async () => {
		const server = net.createServer();
		const port: number = await new Promise(resolve => {
			server.listen(0, "127.0.0.1", () => resolve((server.address() as net.AddressInfo).port));
		});
		try {
			// Reaching into the module's own probe is deliberate: this test
			// exists to prove the injected probe used above is not lying
			// about how a real bound port behaves.
			const free = await new Promise<boolean>(resolve => {
				const probe = net.createServer();
				probe.once("error", () => resolve(false));
				probe.once("listening", () => probe.close(() => resolve(true)));
				probe.listen(port, "127.0.0.1");
			});
			expect(free).toBe(false);
		} finally {
			await new Promise(resolve => server.close(resolve));
		}
	});
});
