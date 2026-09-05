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
const PORT_STEP = 100;

function findPortOffset(): number {
	// import.meta.dir is Bun-only; vite.config.ts loads this file under
	// Node (Vite's own config loader), where it's undefined. import.meta.url
	// -> fileURLToPath is the portable way to get this file's own directory
	// under both runtimes.
	let directory = path.dirname(fileURLToPath(import.meta.url));
	for (;;) {
		const candidate = path.join(directory, '.port-offset');
		if (existsSync(candidate)) {
			const value = Number(readFileSync(candidate, 'utf8').trim());
			return Number.isSafeInteger(value) && value >= 0 ? value : 0;
		}
		const parent = path.dirname(directory);
		if (parent === directory) return 0;
		directory = parent;
	}
}

export const PORT_OFFSET = findPortOffset();

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
