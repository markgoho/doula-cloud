import { availableParallelism } from 'node:os';
import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { E2E_API_HOST, E2E_API_PORT, DEV_SERVER_PORT, PREVIEW_SERVER_PORT } from './e2e/ports.ts';

// How many headless Chromium renderers the `client` project may open at
// once. Six rather than Vitest's own `Math.min(12, ncpu - 1)`, and
// clamped rather than constant -- see the `maxWorkers` comment below for
// both halves of that. `Math.max(..., 1)` only guards a single-core box,
// where `availableParallelism() - 1` would otherwise be zero.
const cpuBudget = Math.max(availableParallelism() - 1, 1);
const BROWSER_WORKERS = Math.min(6, cpuBudget);

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
					// Vitest sizes the browser pool as `Math.min(12, ncpu - 1)`
					// -- a guard against the main thread choking, with nothing
					// in it that asks what memory is free. On a 14-CPU laptop
					// that is 12 headless Chromium renderers and a ~7GB peak,
					// and every commit pays it (see scripts/hooks/pre-commit).
					// Two sessions committing at once exhausted 24GB of RAM and
					// the harness killed the commit -- #935. Six renderers peak
					// at ~4.9GB and finish *faster* than twelve, because twelve
					// oversubscribes 14 cores. `--maxWorkers` on the command
					// line cannot override this: the pool reads the project's
					// own config, not the root's.
					//
					// Clamped rather than constant so CI is untouched: its
					// 4-vCPU runner already resolves to 3, and a bare 6 would
					// *raise* the parallelism there.
					//
					// Lowering this further buys no third concurrent gate:
					// ~3.5GB per run is fixed cost no worker count removes.
					// scripts/gate-lock.ts is what coordinates sessions with
					// each other -- #936, and docs/testing.md's "Only one
					// session runs this step at a time".
					maxWorkers: BROWSER_WORKERS,
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
					// Vitest refuses two projects that share a groupOrder but
					// disagree on maxWorkers, so capping the browser project
					// above forces this one into its own group. That is a
					// second memory win, not just a formality: the Node forks
					// no longer overlap the Chromium renderers, so the run has
					// one peak instead of two stacked on each other.
					sequence: { groupOrder: 1 },
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
