import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';

export default defineConfig({
	plugins: [
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) => filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},

			// Every route is prerendered (src/routes/+layout.ts), so the
			// build is a folder of HTML files and nothing else: no
			// fallback page, because an unknown path on doula.cloud has to
			// be a real 404. The #443 probe depends on that -- a catch-all
			// that answered 200 for a missing Practice page would be
			// indistinguishable from a live one without the marker check.
			//
			// `strict: false` because a build with no Practice pages leaves
			// /p/[slug] with nothing to prerender, which strict mode refuses.
			// handleUnseenRoutes below still refuses every other route that
			// goes unbuilt.
			adapter: adapter({ strict: false }),

			prerender: {
				// '*' is every route without a parameter, plus whatever each
				// dynamic route's entries() names. sitemap.xml is added by
				// hand because no page links to it for the crawler to find.
				entries: ['*', '/sitemap.xml'],

				// A build with no Practice pages is a correct build: a local
				// one and a PR preview have no database credential, and the
				// sync script leaves practice-pages/ empty on purpose
				// (docs/environment.md). So /p/[slug] may go unbuilt. Any
				// other route that goes unbuilt is still an error.
				handleUnseenRoutes: ({ routes, message }) => {
					if (routes.some((route) => route !== '/p/[slug]')) throw new Error(message);
				}
			}
		})
	],
	server: {
		fs: {
			// The site's stylesheet is app/'s own (src/routes/+layout.svelte
			// imports app/src/lib/styles/app.css), so dev has to be allowed
			// to serve the sibling tree it lives in. A build reads it
			// through Vite's module graph and needs no permission.
			allow: [fileURLToPath(new URL('../app/src/lib/styles', import.meta.url))]
		}
	},
	test: {
		expect: { requireAssertions: true },
		coverage: {
			provider: 'v8',
			// src/routes/** stays out of this include, the same convention
			// app/ and gcp-dashboard/ follow (docs/testing.md's Coverage
			// section): route specs still run in the projects below, they
			// are just not folded into the 100% requirement.
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
