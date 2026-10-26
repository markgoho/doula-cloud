// Shared seam for the two SessionStart reapers that talk to the local
// container engine -- testdb-reap.ts (testcontainers Postgres containers,
// #889) and e2e-stack-reap.ts (podman-compose e2e stacks, #1066). Both
// need the same two things: which binary to exec, and how to point it at
// the right engine. Neither should carry its own copy of either answer.

import { execFileSync } from 'node:child_process';

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
  return raw.map((entry) => ({
    id: entry.Id,
    name: entry.Names?.[0] ?? entry.Id,
    labels: entry.Labels ?? {},
    createdAtMs: entry.Created * 1000,
  }));
}

// Local dev defaults to Podman (docs/testing/e2e.md); CI sets
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
 * theoretical: DOCKER_HOST is set by hand on the command that runs
 * `go test` (docs/testing/api.md says so), and a hook inherits the login
 * environment, not that shell's, so the reaper never ran. (A login
 * profile can export a DOCKER_HOST of its own, and for another engine;
 * execEngine below is what copes with that.) `podman ps` with no `--url` uses
 * the default `podman machine` connection, which is the same engine, so
 * the honest guard is no guard at all: invoke the engine and let the
 * fail-open catch handle a machine that isn't there.
 */
export function engineInvocation(args: string[]): {
  binary: string;
  argv: string[];
} {
  const dockerHost = process.env.DOCKER_HOST;
  return {
    binary: engineBinary(),
    argv: dockerHost ? ['--url', dockerHost, ...args] : args,
  };
}

export type EngineRunner = (binary: string, argv: string[]) => string;

function runCaptured(binary: string, argv: string[]): string {
  return execFileSync(binary, argv, {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  });
}

/*
 * Runs the engine with `args` and returns its stdout: first through
 * engineInvocation (with `--url` when DOCKER_HOST is set), then, if that
 * fails, once more with no `--url` at all, so the engine falls back to
 * its own default connection. Throws the last failure when both fail.
 *
 * The fallback is not hypothetical (#2096). This Mac's login profile
 * exports DOCKER_HOST for another engine's socket (a Colima Docker
 * socket), and a SessionStart hook inherits that profile. `podman --url`
 * at that socket fails, the reaper failed open as designed, and nothing
 * reaped for days while bare `podman ps` reached the right engine all
 * along: 91 `org.testcontainers=true` containers, the oldest 23 hours
 * old, observed live on 2026-10-26.
 */
export function execEngine(
  args: string[],
  run: EngineRunner = runCaptured
): string {
  const first = engineInvocation(args);
  try {
    return run(first.binary, first.argv);
  } catch (error) {
    if (first.argv.length === args.length) throw error;
    return run(first.binary, args);
  }
}
