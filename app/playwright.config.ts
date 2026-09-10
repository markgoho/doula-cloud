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
// #935 capped in the unit gate. Measured warm on a 14-CPU / 24 GB
// machine, where the default resolves to 7 (#937): memory falls roughly
// linearly with the count -- 4.6 GB peak at seven, 2.9 GB at four -- while
// wall time is flat from 7 down to 4 and only starts rising at 3. Four is
// the lowest count that costs nothing in speed, which is the same rule
// #935 picked six by. The table, the method and the caveats live in
// docs/testing.md, "What the e2e suite costs, and why its workers are
// capped" -- one copy, so re-measuring cannot leave two that disagree.
//
// Clamped rather than constant so CI is untouched: Playwright's own '50%'
// already resolves to 2 on the 4-vCPU ubuntu-latest runner, and a bare 4
// would *raise* the parallelism there. (Vitest reads a different formula
// and resolves to 3 on the same runner -- see app/vite.config.ts.)
const E2E_WORKERS = Math.min(4, playwrightDefaultWorkers);

export default defineConfig({
	workers: E2E_WORKERS,
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
