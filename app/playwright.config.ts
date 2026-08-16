import { defineConfig } from '@playwright/test';
import { E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './e2e/ports';

export default defineConfig({
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
		port: 4173,
		// VITE_* vars are inlined at build time, so the emulator host has to
		// be set before `bun run build` runs, not just at request time.
		env: { VITE_FIREBASE_AUTH_EMULATOR_HOST: `${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}` }
	},
	testMatch: '**/*.e2e.{ts,js}'
});
