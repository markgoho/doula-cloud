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
// - scripts/dev-full.ts's dev-server env (same, for `bun run dev:full`)
// - staff-login.e2e.ts, to talk to the emulator and BFF directly
export const E2E_API_HOST = '127.0.0.1';
export const E2E_API_PORT = 18_080;
export const E2E_EMULATOR_HOST = '127.0.0.1';
export const E2E_EMULATOR_PORT = 9099;
