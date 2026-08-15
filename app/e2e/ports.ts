// Single source of truth for the e2e stack's ports/hosts. Consumed by:
// - compose.e2e.yaml, via ${E2E_API_PORT}/${E2E_EMULATOR_PORT} env vars
//   global-setup.ts passes to `podman compose up` (with matching
//   defaults in the yaml itself, for anyone running it by hand)
// - vite.config.ts's preview proxy
// - playwright.config.ts's webServer env (build-time emulator host)
// - staff-login.e2e.ts, to talk to the emulator and BFF directly
export const E2E_API_HOST = '127.0.0.1';
export const E2E_API_PORT = 18080;
export const E2E_EMULATOR_HOST = '127.0.0.1';
export const E2E_EMULATOR_PORT = 9099;
