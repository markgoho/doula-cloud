#!/usr/bin/env bun
// SessionStart hook: reaps the podman-compose e2e stack a killed worktree
// session leaves running. See docs/testing.md's "Reaping orphaned e2e
// stacks" section for the full story; the short version is below.
//
// app/e2e/stack.ts brings up app/compose.e2e.yaml under a per-worktree
// compose project name -- `doula-cloud-e2e-<PORT_OFFSET>` -- and tears it
// down again in stopStack. A killed session (an interrupted agent, a
// timeout, a cut-short Playwright run) never reaches that teardown, so
// its `db` and `gcs` containers, the project's network and its named
// volume sit on the shared 4GB Podman machine indefinitely. That is the
// same starvation #889 diagnosed and #782 most likely hit, and
// testdb-reap.ts does nothing about it: it only looks at containers
// labeled `org.testcontainers=true`, which a compose stack is not.
//
// The reap decision (groupStackProjects/pickReapProjects below) is pure
// and exported for direct unit testing. main() -- the engine invocation,
// the compose teardown, the fail-open catch -- is exercised as a
// subprocess instead; see scripts/e2e-stack-reap.test.ts.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { engineInvocation, parseContainers, type ReapCandidate } from './container-engine.ts';
import { findMainCheckoutRoot } from './worktree-root.ts';

// podman-compose 1.6.0 and Docker Compose v2 both stamp this key on every
// container they create (confirmed by starting app/compose.e2e.yaml on
// this machine and reading the labels back). podman-compose writes its
// own `io.podman.compose.project` alongside it; the docker-namespaced key
// is the one both engines agree on, so it is the one matched here.
const COMPOSE_PROJECT_LABEL = 'com.docker.compose.project';

/*
 * Only a worktree's stack, never the main checkout's.
 *
 * app/e2e/stack.ts suffixes the compose project name with PORT_OFFSET,
 * and only when that offset is non-zero -- offset 0 is the main checkout
 * and CI, whose project name is the bare `doula-cloud-e2e`. This pattern
 * therefore matches worktree stacks and nothing else, deliberately: the
 * main checkout has no `.port-offset` file to go quiet, it is where a
 * long interactive `bun run dev:full` runs, and it is the one place a
 * person is standing when they would notice their own database vanish.
 * Its stack is left alone.
 */
const WORKTREE_PROJECT_PATTERN = /^doula-cloud-e2e-(\d+)$/;

/*
 * Two clocks decide, and neither on its own.
 *
 * REAP_THRESHOLD_MS is the stack's own age, and it is the weaker of the
 * two: unlike a testcontainers Postgres, which backs one `go test`
 * process and lives minutes at most, an e2e stack legitimately outlives
 * any fixed budget -- `bun run dev:full` shares it and runs for as long
 * as somebody is working. So age alone must never authorize a reap. It is
 * here to cover the one window the quiet check cannot: a stack brought up
 * seconds ago inside a worktree whose files happen not to have been
 * touched since. 15 minutes, matching testdb-reap.ts, is far longer than
 * `compose up -d` plus the migrate (<=90s) and `go build` (<=120s) steps
 * that follow it.
 *
 * QUIET_MS is the real signal: has anything touched the worktree that
 * claims this stack's port offset lately. It is the same 30 minutes, and
 * the same reasoning, that worktree-prune.ts already trusts to delete a
 * whole worktree directory -- if that is enough to justify removing the
 * work, it is enough to justify removing the containers beside it.
 *
 * Deliberately NOT used as a liveness signal: whether the stack's host
 * BFF is up. app/e2e/stack.ts spawns it `detached` and `unref`s it, so it
 * outlives the session that started it -- an orphaned stack's BFF is
 * still listening on its port, which would make every orphan look alive
 * and this hook a no-op.
 */
export const REAP_THRESHOLD_MS = 15 * 60 * 1000;
export const QUIET_MS = 30 * 60 * 1000;

export interface StackProject {
	project: string;
	offset: number;
	// The newest container in the project, not the oldest: the whole
	// project is only as stale as its most recent sign of life.
	newestCreatedAtMs: number;
	containerIds: string[];
}

// Collapses a flat container list into the compose projects behind it,
// keeping only names app/e2e/stack.ts could have produced for a worktree.
export function groupStackProjects(containers: ReapCandidate[]): StackProject[] {
	const projects = new Map<string, StackProject>();
	for (const container of containers) {
		const name = container.labels[COMPOSE_PROJECT_LABEL];
		if (name === undefined) continue;
		const match = WORKTREE_PROJECT_PATTERN.exec(name);
		if (match === null) continue;

		const existing = projects.get(name);
		if (existing === undefined) {
			projects.set(name, {
				project: name,
				offset: Number(match[1]),
				newestCreatedAtMs: container.createdAtMs,
				containerIds: [container.id]
			});
			continue;
		}
		existing.containerIds.push(container.id);
		existing.newestCreatedAtMs = Math.max(existing.newestCreatedAtMs, container.createdAtMs);
	}
	return [...projects.values()];
}

// The one place that decides what gets torn down. Matches the project
// name itself rather than trusting the engine's own `--filter` flag to
// have done it -- a destructive command should not depend solely on a
// filter string built elsewhere staying correct.
export function pickReapProjects(
	projects: StackProject[],
	liveOffsets: ReadonlySet<number>,
	nowMs: number,
	thresholdMs: number = REAP_THRESHOLD_MS
): StackProject[] {
	return projects.filter(project => !liveOffsets.has(project.offset) && nowMs - project.newestCreatedAtMs > thresholdMs);
}

// Has anything touched this worktree lately? The worktree's own directory,
// the git dir its `.git` file points at, and that dir's index -- the same
// three worktree-prune.ts checks, read here without shelling out to git so
// a reaper never pays for a subprocess per worktree.
export function recentlyTouched(worktreePath: string, nowMs: number, quietMs: number = QUIET_MS): boolean {
	const candidates = [worktreePath];
	try {
		const pointer = fs.readFileSync(path.join(worktreePath, '.git'), 'utf8').trim();
		const gitDir = pointer.startsWith('gitdir:') ? pointer.slice('gitdir:'.length).trim() : path.join(worktreePath, '.git');
		candidates.push(gitDir, path.join(gitDir, 'index'));
	} catch {
		// No readable `.git` -- the directory itself still answers below.
	}
	for (const candidate of candidates) {
		try {
			if (nowMs - fs.statSync(candidate).mtimeMs < quietMs) return true;
		} catch {
			// A missing path tells us nothing -- keep checking the others.
		}
	}
	return false;
}

/*
 * The port offsets whose worktree is both present and recently touched.
 *
 * An offset missing from this set is one no live session can be using:
 * either no worktree carries that `.port-offset` at all (the session's
 * directory is gone), or the one that does has been quiet for QUIET_MS.
 * Mirrors worktree-provision.ts's own livePortOffsets, which reads the
 * same files to decide which offset a new worktree may claim.
 */
export function liveWorktreeOffsets(worktreesRoot: string, nowMs: number, quietMs: number = QUIET_MS): Set<number> {
	const live = new Set<number>();
	let entries: string[] = [];
	try {
		entries = fs.readdirSync(worktreesRoot);
	} catch {
		return live; // no worktrees directory -- nothing claims any offset
	}
	for (const entry of entries) {
		const worktreePath = path.join(worktreesRoot, entry);
		let offset: number;
		try {
			offset = Number.parseInt(fs.readFileSync(path.join(worktreePath, '.port-offset'), 'utf8').trim(), 10);
		} catch {
			continue; // no offset assigned yet -- claims nothing
		}
		if (!Number.isInteger(offset)) continue;
		if (recentlyTouched(worktreePath, nowMs, quietMs)) live.add(offset);
	}
	return live;
}

/*
 * Fails open, always -- the same rule, for the same reason, as
 * testdb-reap.ts's own main(): this runs on SessionStart, decides nothing
 * about the tool call that follows, and exists purely to tidy up. A
 * reaper that errors must never block or slow a session from starting.
 * Do not "fix" this into failing closed.
 */
function main(): void {
	try {
		const ps = engineInvocation(['ps', '-a', '--filter', `label=${COMPOSE_PROJECT_LABEL}`, '--format', 'json']);
		const psOutput = execFileSync(ps.binary, ps.argv, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });

		const mainCheckout = findMainCheckoutRoot(import.meta.dir);
		const now = Date.now();
		const live = liveWorktreeOffsets(path.join(mainCheckout, '.claude', 'worktrees'), now);
		const doomed = pickReapProjects(groupStackProjects(parseContainers(psOutput)), live, now);
		if (doomed.length === 0) return;

		/* `compose down -v` rather than `rm -f` on the container ids: the
		   containers are the smaller half of what a stack leaves behind.
		   Scoped to the project name, `down -v` also removes the project's
		   network (`<project>_default`) and its named `db-data` volume,
		   which nothing else on the machine would ever collect -- confirmed
		   against podman-compose 1.6.0 by starting the stack and reading
		   `network ls`/`volume ls` back on either side of it. The compose
		   file only names the services; the project name selects what goes. */
		const composeFile = path.join(mainCheckout, 'app', 'compose.e2e.yaml');
		const removed: string[] = [];
		for (const project of doomed) {
			try {
				const down = engineInvocation(['compose', '-p', project.project, '-f', composeFile, 'down', '-v', '-t', '2']);
				execFileSync(down.binary, down.argv, { stdio: ['ignore', 'pipe', 'pipe'] });
				removed.push(project.project);
			} catch {
				// One project's teardown failing must not stop the others.
			}
		}
		if (removed.length === 0) return;

		const minutes = Math.round(QUIET_MS / 60000);
		console.log(
			`e2e-stack-reap: tore down ${removed.length} orphaned e2e stack(s) -- ${removed.join(', ')} -- containers, network and volume (no worktree touched in the last ${minutes}m claims their port offset)`
		);
	} catch {
		// Engine unreachable, binary missing, malformed output, git absent --
		// none of it may ever surface as a blocked or slowed SessionStart.
		return;
	}
}

if (import.meta.main) main();
