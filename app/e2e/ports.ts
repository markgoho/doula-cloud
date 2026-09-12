// Single source of truth for the local stack's ports/hosts -- the same
// db-only podman-compose stack + host-process migrate/BFF/Firebase Auth
// emulator setup (see e2e/stack.ts) backs both the Playwright e2e run
// and `bun run dev:full` for interactive local use.
// Consumed by:
// - e2e/stack.ts, which passes E2E_API_PORT/E2E_EMULATOR_PORT straight
//   into the host-process BFF's and emulator's own env (PORT,
//   FIREBASE_AUTH_EMULATOR_HOST) rather than through compose
// - vite.config.ts's dev and preview proxies
// - playwright.config.ts's webServer env (build-time emulator host)
// - scripts/development-full.ts's dev-server env (same, for `bun run dev:full`)
// - staff-login.e2e.ts, to talk to the emulator and BFF directly
import { existsSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Every port below shifts by PORT_OFFSET * PORT_STEP so two worktrees can
// run the full stack at once (see docs/agents/worktree-flow.md). The
// offset comes from a `.port-offset` file at the worktree root, written by
// the worktree provisioning hook; walking up from this file's own
// directory finds it regardless of which script imports ports.ts. No file
// (the main checkout, and every CI run) means offset 0 -- today's exact
// ports, unchanged.
//
// 100 is deliberate: the tightest gap between two base ports below is
// 1000 (4173 -> 5173), so any offset under 1000 is collision-free. Ten
// concurrent worktrees (offsets 0-9) is ample.
export const PORT_STEP = 100;

// Finds `.port-offset` the same walk-up-from-here way for both the offset
// itself and the directory that holds it -- the latter is where
// e2eHeartbeatPath below writes the stack's liveness marker (#1193), so a
// worktree's `.port-offset` and its `.e2e-heartbeat` always sit side by
// side. `root` is undefined for the main checkout and every CI run, which
// have no `.port-offset` file to walk up to.
function findPortOffset(): { offset: number; root: string | undefined } {
	// import.meta.dir is Bun-only; vite.config.ts loads this file under
	// Node (Vite's own config loader), where it's undefined. import.meta.url
	// -> fileURLToPath is the portable way to get this file's own directory
	// under both runtimes.
	let directory = path.dirname(fileURLToPath(import.meta.url));
	for (;;) {
		const candidate = path.join(directory, '.port-offset');
		if (existsSync(candidate)) {
			const value = Number(readFileSync(candidate, 'utf8').trim());
			const offset = Number.isSafeInteger(value) && value >= 0 ? value : 0;
			return { offset, root: directory };
		}
		const parent = path.dirname(directory);
		if (parent === directory) return { offset: 0, root: undefined };
		directory = parent;
	}
}

const portOffset = findPortOffset();
export const PORT_OFFSET = portOffset.offset;
export const PORT_OFFSET_ROOT = portOffset.root;

function shift(port: number): number {
	return port + PORT_OFFSET * PORT_STEP;
}

export const E2E_API_HOST = '127.0.0.1';
export const E2E_API_PORT = shift(18_080);
export const E2E_EMULATOR_HOST = '127.0.0.1';
export const E2E_EMULATOR_PORT = shift(9099);
export const DB_HOST = '127.0.0.1';
export const DB_PORT = shift(15_432);
export const GCS_HOST = '127.0.0.1';
export const GCS_PORT = shift(14_443);
// The sandbox mailbox (e2e/mailbox.ts, #764) -- the Mailgun-shaped sink
// the BFF's MAILGUN_API_BASE points at, and the inbox a persona opens in
// a browser. 1000 above the BFF, keeping this file's collision-free gap.
export const MAILBOX_HOST = '127.0.0.1';
export const MAILBOX_PORT = shift(19_080);

export const DEV_SERVER_PORT = shift(5173);
export const PREVIEW_SERVER_PORT = shift(4173);

// The front-end origin each of the local BFF's two callers runs at --
// `vite dev`'s port for `bun run dev:full`, and `vite preview`'s
// (playwright.config.ts's webServer.port) for the Playwright stack. Each
// caller passes its own one of these into e2e/stack.ts's startStack, which
// the BFF's csrf.Wrap (api/internal/csrf) then requires a matching Origin
// header on state-changing requests.
export const DEV_SERVER_ORIGIN = `http://localhost:${DEV_SERVER_PORT}`;
export const PREVIEW_SERVER_ORIGIN = `http://localhost:${PREVIEW_SERVER_PORT}`;

// Every port the local stack binds, unshifted -- i.e. what offset 0 uses.
// Derived by undoing shift() rather than restating the numbers, so this
// list cannot drift from the definitions above and is correct no matter
// which worktree's .port-offset findPortOffset() happened to land on.
//
// The worktree provisioning hook (.claude/hooks/worktree-provision.ts)
// reads this to check whether an offset's ports are actually free on the
// machine before assigning it: "not claimed by another worktree" is not
// the same as "available" when an unrelated local process holds one of
// them (#927).
// The compose project every `podman compose`/`docker compose` call for
// this stack is scoped to (see e2e/stack.ts): one per worktree, so two
// worktrees' stacks never collide on container names. It lives here, next
// to the ports, because a worktree's PORT_OFFSET is the whole of its
// identity on this machine, and because a second consumer already needs
// it -- `.claude/hooks/e2e-stack-reap.ts` matches these names to find the
// stack a killed session left running, and would go silently inert if a
// rename happened here and not there.
export const E2E_COMPOSE_PROJECT_PREFIX = 'doula-cloud-e2e';
// Suffix only for a real worktree (offset > 0); at offset 0 (the main
// checkout, and CI) this is the bare prefix, exactly the implicit project
// name this stack has always used.
export function e2eComposeProject(offset: number): string {
	return `${E2E_COMPOSE_PROJECT_PREFIX}${offset ? `-${offset}` : ''}`;
}
// The compose file itself, named once for the same reason.
export const E2E_COMPOSE_FILE = 'compose.e2e.yaml';

// The liveness marker a `bun run test:e2e` or `bun run dev:full` run
// keeps fresh for as long as it is actually using the stack (#1193).
// `startStack`/`stopStack` (e2e/stack.ts) touch it on a timer at the
// worktree root, beside `.port-offset` -- not in `os.tmpdir()`, because
// that file is read by a *different* process than the one that writes
// it (`.claude/hooks/e2e-stack-reap.ts`, on a later SessionStart), and
// `$TMPDIR` is not guaranteed to agree between a hook's shell and the
// one a stack was started from. The worktree root is: it is the one
// path both sides can name without asking the environment, and it is
// gitignored next to `.port-offset` for the same reason that file is.
//
// Named once here, like E2E_COMPOSE_PROJECT_PREFIX above, so the writer
// and the reader cannot drift onto different filenames.
export function e2eHeartbeatPath(worktreeRoot: string): string {
	return path.join(worktreeRoot, '.e2e-heartbeat');
}
// How often a live stack must touch its heartbeat file to keep
// e2e-stack-reap.ts from treating it as orphaned once its worktree goes
// quiet by the ordinary (file-mtime) check. Read by both the writer
// (e2e/stack.ts's startHeartbeat) and the reader
// (.claude/hooks/e2e-stack-reap.ts's HEARTBEAT_STALE_MS, a multiple of
// this) so neither can set a staleness threshold shorter than the beat
// itself by accident.
export const E2E_HEARTBEAT_INTERVAL_MS = 60_000;

// The three host processes e2e/stack.ts tracks by pidfile -- the Firebase
// Auth emulator, the Go BFF and the sandbox mailbox (#1194). Named once
// here, the same reason E2E_COMPOSE_PROJECT_PREFIX is: `e2ePidfilePath`
// below is how both the writer (e2e/stack.ts, always at its own
// PORT_OFFSET) and the reader (.claude/hooks/e2e-stack-reap.ts, sweeping
// every offset on the machine) name the same file without either side
// spelling out os.tmpdir() naming twice.
export const E2E_PIDFILE_KINDS = ['firebase-emulator', 'api', 'mailbox'] as const;
export type E2EPidfileKind = (typeof E2E_PIDFILE_KINDS)[number];

// Where kind's pidfile lives for a given offset. Suffix omitted at
// offset 0, matching e2eComposeProject above -- the main checkout and CI
// keep today's exact filenames, and (just as important for the reaper)
// an offset-0 pidfile can never be *produced* by this function, so
// nothing built from it can ever name the main checkout's own process.
export function e2ePidfilePath(kind: E2EPidfileKind, offset: number): string {
	return path.join(tmpdir(), `${E2E_COMPOSE_PROJECT_PREFIX}-${kind}${offset ? `-${offset}` : ''}.pid`);
}

// The Go BFF binary startAPI builds and execs, offset-suffixed the same
// way its pidfile is (see API_BINARY_PATH, e2e/stack.ts). Exported so the
// pidfile reaper can reconstruct *any* offset's expected binary path --
// not just its own -- and use it as the one identifying fact that a pid
// naming an `api` pidfile is actually still running this repo's BFF and
// not some unrelated process the OS recycled that pid onto.
export function e2eApiBinaryPath(offset: number): string {
	return path.join(tmpdir(), `${E2E_COMPOSE_PROJECT_PREFIX}-api${offset ? `-${offset}` : ''}`);
}

export const BASE_PORTS: readonly number[] = [
	E2E_API_PORT,
	E2E_EMULATOR_PORT,
	DB_PORT,
	GCS_PORT,
	MAILBOX_PORT,
	DEV_SERVER_PORT,
	PREVIEW_SERVER_PORT,
].map(port => port - PORT_OFFSET * PORT_STEP);
