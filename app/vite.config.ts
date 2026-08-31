import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { E2E_API_HOST, E2E_API_PORT, DEV_SERVER_PORT, PREVIEW_SERVER_PORT } from './e2e/ports.ts';

export default defineConfig({
	plugins: [
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) => filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},

			// Fully client-side SPA behind auth, no SSR -- see
			// src/routes/+layout.ts for the ssr = false that fallback mode
			// requires. `200.html` (not `index.html`) per adapter-static's
			// own guidance, since it's the name Firebase Hosting's
			// SPA-rewrite convention expects for a catch-all fallback.
			adapter: adapter({ fallback: '200.html' })
		})
	],
	// `vite dev` (server) and Playwright's webServer (`vite preview`) each
	// need their own proxy entry to reach the Go BFF container without
	// hitting CORS -- see e2e/ports.ts for where the port (and, for a
	// worktree, the offset) comes from.
	server: {
		port: DEV_SERVER_PORT,
		proxy: {
			'/api': `http://${E2E_API_HOST}:${E2E_API_PORT}`
		}
	},
	preview: {
		port: PREVIEW_SERVER_PORT,
		proxy: {
			'/api': `http://${E2E_API_HOST}:${E2E_API_PORT}`
		}
	},
	test: {
		expect: { requireAssertions: true },
		coverage: {
			provider: 'v8',
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
					include: [
						'src/**/*.svelte.{test,spec}.{js,ts}',
						'src/lib/primitives/**/*.{test,spec}.{js,ts}'
					],
					exclude: ['src/lib/server/**']
				}
			},

			{
				extends: './vite.config.ts',
				test: {
					name: 'server',
					environment: 'node',
					include: ['src/**/*.{test,spec}.{js,ts}'],
					exclude: [
						'src/**/*.svelte.{test,spec}.{js,ts}',
						'src/lib/primitives/**/*.{test,spec}.{js,ts}'
					]
				}
			}
		]
	}
});
