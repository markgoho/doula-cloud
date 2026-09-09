import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';
import adapter from '@sveltejs/adapter-node';
import { sveltekit } from '@sveltejs/kit/vite';

export default defineConfig({
	plugins: [
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) => filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},

			// adapter-node, not app/'s adapter-static -- this project's
			// +server.ts routes call the BigQuery/Monitoring APIs
			// server-side and need a real Node process to run in, not a
			// prerendered static bundle. See README.md for how it runs.
			adapter: adapter()
		})
	],
	test: {
		expect: { requireAssertions: true },
		coverage: {
			provider: 'v8',
			// src/routes/** stays out of this include, same convention
			// app/vite.config.ts documents (see docs/testing.md's Coverage
			// section): it is exercised by the `client`/`server` projects
			// below, just not folded into the 100% requirement. See
			// README.md's Testing section for why.
			include: ['src/lib/**/*.{ts,svelte}'],
			thresholds: {
				100: true
			}
		},
		projects: [
			{
				extends: './vite.config.ts',
				test: {
					name: 'client',
					browser: {
						enabled: true,
						provider: playwright(),
						instances: [{ browser: 'chromium', headless: true }]
					},
					include: ['src/**/*.svelte.{test,spec}.{js,ts}']
				}
			},

			{
				extends: './vite.config.ts',
				test: {
					name: 'server',
					environment: 'node',
					include: ['src/**/*.{test,spec}.{js,ts}'],
					exclude: ['src/**/*.svelte.{test,spec}.{js,ts}']
				}
			}
		]
	}
});
