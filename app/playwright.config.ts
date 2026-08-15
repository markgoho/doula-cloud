import { defineConfig } from '@playwright/test';

export default defineConfig({
	globalSetup: './e2e/global-setup.ts',
	globalTeardown: './e2e/global-teardown.ts',
	webServer: { command: 'bun run build && bun run preview', port: 4173 },
	testMatch: '**/*.e2e.{ts,js}'
});
