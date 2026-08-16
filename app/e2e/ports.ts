// Single source of truth for the local stack's ports/hosts -- the same
// podman-compose + Firebase Auth emulator setup backs both the
// Playwright e2e run and `bun run dev:full` for interactive local use.
// Consumed by:
// - compose.e2e.yaml, via ${E2E_API_PORT}/${E2E_EMULATOR_PORT} env vars
//   stack.ts passes to `podman compose up` (with matching defaults in
//   the yaml itself, for anyone running it by hand)
// - vite.config.ts's dev and preview proxies
// - playwright.config.ts's webServer env (build-time emulator host)
// - scripts/dev-full.ts's dev-server env (same, for `bun run dev:full`)
// - staff-login.e2e.ts, to talk to the emulator and BFF directly
export const E2E_API_HOST = '127.0.0.1';
export const E2E_API_PORT = 18_080;
export const E2E_EMULATOR_HOST = '127.0.0.1';
export const E2E_EMULATOR_PORT = 9099;
