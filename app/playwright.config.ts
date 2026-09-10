import { cpus } from 'node:os';
import { defineConfig } from '@playwright/test';
import { E2E_EMULATOR_HOST, E2E_EMULATOR_PORT, PREVIEW_SERVER_PORT } from './e2e/ports';

// Playwright's own default, restated so the clamp below can never raise
// parallelism anywhere: `workers` defaults to the string '50%', which
// resolveWorkers() turns into `Math.max(1, Math.floor(os.cpus().length / 2))`.
// `os.cpus().length` rather than `availableParallelism()` because that is
// what Playwright itself reads -- app/vite.config.ts mirrors Vitest's
// formula for the same reason, and the two formulas differ.
const playwrightDefaultWorkers = Math.max(Math.floor(cpus().length / 2), 1);
// Four, because each Playwright worker gets its **own** browser -- unlike
// Vitest's browser pool, where many renderers share one. Nothing in the
// '50%' default asks what memory is free, which is the same blindness
// #935 capped in the unit gate. Measured warm on a 14-CPU / 24GB machine,
// where the default resolves to 7 (#937):
//
//   7 (the default here) | 4.58 / 4.63 GB peak | 40.9s / 41.8s
//   4 (what we pin)      | 2.88 / 3.03 GB peak | 42.2s / 40.3s
//   3                    | 2.42 GB peak        | 47.0s
//
// Memory falls roughly linearly with the count -- about 0.55 GB a worker
// -- while wall time is flat from 7 down to 4 and only starts rising at
// 3. So four is the lowest count that costs nothing in speed, which is
// the same rule #935 picked six by. See docs/testing.md, "What the e2e
// suite costs, and why its workers are capped".
//
// Clamped rather than constant so CI is untouched: its 4-vCPU
// ubuntu-latest runner already resolves to 2, and a bare 4 would *raise*
// the parallelism there.
const WORKERS = Math.min(4, playwrightDefaultWorkers);

export default defineConfig({
	workers: WORKERS,
	globalSetup: './e2e/global-setup.ts',
	globalTeardown: './e2e/global-teardown.ts',
	// CI runs everything (Postgres, Firebase emulator, the Go BFF, and the
	// browser) on one shared runner; a genuine one-off scheduling/timing
	// stall under that contention shouldn't fail the whole build the way a
	// real regression should. trace/screenshot/video are captured only on
	// the failing attempt, so a true regression still leaves debuggable
	// evidence instead of silently passing on retry.
	retries: process.env.CI ? 2 : 0,
	// Default reporter ('list') never writes playwright-report/, so CI's
	// "Upload Playwright report" step had nothing to pick up -- add 'html'
	// so a failing run's trace/screenshot evidence is actually browsable
	// from the uploaded artifact, not just implied by retries.
	reporter: [['list'], ['html', { open: 'never' }]],
	use: {
		trace: 'on-first-retry',
		screenshot: 'only-on-failure'
	},
	webServer: {
		command: 'bun run build && bun run preview',
		port: PREVIEW_SERVER_PORT,
		// VITE_* vars are inlined at build time, so the emulator host has to
		// be set before `bun run build` runs, not just at request time.
		env: { VITE_FIREBASE_AUTH_EMULATOR_HOST: `${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}` }
	},
	testMatch: '**/*.e2e.{ts,js}'
});
