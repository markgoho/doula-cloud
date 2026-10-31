import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { OPS_DEV_SERVER_PORT, OPS_PREVIEW_SERVER_PORT, OPS_URL } from '../app/e2e/ports.ts';

export default defineConfig({
	plugins: [
		sveltekit({
			compilerOptions: {
				runes: ({ filename }) => (filename.split(/[/\\]/).includes('node_modules') ? undefined : true)
			},
			adapter: adapter({ fallback: '200.html' })
		})
	],
	server: {
		port: OPS_DEV_SERVER_PORT,
		strictPort: true,
		proxy: { '/api': OPS_URL },
		fs: {
			// The stylesheet is app/'s (src/routes/+layout.svelte), as site/'s is.
			allow: [fileURLToPath(new URL('../app/src/lib/styles', import.meta.url))]
		}
	},
	preview: {
		port: OPS_PREVIEW_SERVER_PORT,
		strictPort: true,
		proxy: { '/api': OPS_URL }
	},
	test: {
		expect: { requireAssertions: true },
		coverage: {
			provider: 'v8',
			include: ['src/lib/**/*.{ts,svelte}'],
			thresholds: { 100: true }
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
