// Shared seam for the two SessionStart reapers that talk to the local
// container engine -- testdb-reap.ts (testcontainers Postgres containers,
// #889) and e2e-stack-reap.ts (podman-compose e2e stacks, #1066). Both
// need the same two things: which binary to exec, and how to point it at
// the right engine. Neither should carry its own copy of either answer.

// The subset of `podman ps --format json` / `docker ps --format json`'s
// per-container object the reapers read. `Created` is a Unix timestamp in
// seconds (confirmed against a live podman-machine-default container);
// `Labels` is an object, not the comma-joined string some engines print
// in their plain-text `--format` output.
interface EnginePs {
	Id: string;
	Names?: string[];
	Labels?: Record<string, string>;
	Created: number;
}

export interface ReapCandidate {
	id: string;
	name: string;
	labels: Record<string, string>;
	createdAtMs: number;
}

export function parseContainers(json: string): ReapCandidate[] {
	const raw = JSON.parse(json) as EnginePs[];
	return raw.map(entry => ({
		id: entry.Id,
		name: entry.Names?.[0] ?? entry.Id,
		labels: entry.Labels ?? {},
		createdAtMs: entry.Created * 1000
	}));
}

// Local dev defaults to Podman (docs/testing.md); CI sets
// CONTAINER_ENGINE=docker. Same variable app/e2e/stack.ts reads, so a
// machine configured for one is configured for both.
export function engineBinary(): string {
	return process.env.CONTAINER_ENGINE ?? 'podman';
}

/*
 * The engine invocation, with `--url` in front of `args` only when
 * DOCKER_HOST actually names a socket.
 *
 * testdb-reap.ts used to return early when DOCKER_HOST was unset, on the
 * reading that an unset variable means "no local container engine
 * configured". That is not true of Podman, and the cost was not
 * theoretical: DOCKER_HOST is exported by hand into the shell that runs
 * `go test` (docs/testing.md says so), never from a login profile, so a
 * hook -- which inherits the login environment, not that shell's -- never
 * saw it and the reaper had never run. `podman ps` with no `--url` uses
 * the default `podman machine` connection, which is the same engine, so
 * the honest guard is no guard at all: invoke the engine and let the
 * fail-open catch handle a machine that isn't there.
 */
export function engineInvocation(args: string[]): { binary: string; argv: string[] } {
	const dockerHost = process.env.DOCKER_HOST;
	return {
		binary: engineBinary(),
		argv: dockerHost ? ['--url', dockerHost, ...args] : args
	};
}
