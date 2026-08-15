import { defineConfig } from '@playwright/test';

export default defineConfig({
	globalSetup: './e2e/global-setup.ts',
	globalTeardown: './e2e/global-teardown.ts',
	webServer: {
		command: 'bun run build && bun run preview',
		port: 4173,
		// VITE_* vars are inlined at build time, so the emulator host has to
		// be set before `bun run build` runs, not just at request time.
		env: { VITE_FIREBASE_AUTH_EMULATOR_HOST: '127.0.0.1:9099' }
	},
	testMatch: '**/*.e2e.{ts,js}'
});
