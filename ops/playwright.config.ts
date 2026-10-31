import { defineConfig } from '../app/e2e/playwrightTest';
import { E2E_EMULATOR_HOST, E2E_EMULATOR_PORT, OPS_PREVIEW_SERVER_PORT } from '../app/e2e/ports';
import { OPS_TENANT_ID } from '../app/e2e/stack';

// The stack is app/'s, brought up by import (e2e/global-setup.ts), so
// there is one stack definition. One worker: the ops suite is small, and
// it shares the stack with nothing else in its own run.
export default defineConfig({
	workers: 1,
	globalSetup: './e2e/global-setup.ts',
	globalTeardown: './e2e/global-teardown.ts',
	retries: process.env.CI ? 2 : 0,
	reporter: [['list'], ['html', { open: 'never' }]],
	use: {
		baseURL: `http://localhost:${OPS_PREVIEW_SERVER_PORT}`,
		trace: 'on-first-retry',
		screenshot: 'only-on-failure'
	},
	webServer: {
		command: 'bun run build && bun run preview',
		port: OPS_PREVIEW_SERVER_PORT,
		env: {
			VITE_FIREBASE_AUTH_EMULATOR_HOST: `${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`,
			VITE_OPS_TENANT_ID: OPS_TENANT_ID
		}
	},
	testMatch: '**/*.e2e.ts'
});
