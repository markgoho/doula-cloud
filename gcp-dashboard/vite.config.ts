import { defineConfig } from 'vitest/config';
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
		// This scaffold ships no application logic yet -- #158 onward add
		// the +server.ts routes and lib code that will carry real specs.
		// Without this, a project with zero test files exits 1 and the
		// coverage gate below (vacuously 100% over an empty src/lib) never
		// gets the chance to report anything.
		passWithNoTests: true,
		coverage: {
			provider: 'v8',
			include: ['src/lib/**/*.{ts,svelte}'],
			thresholds: {
				100: true
			}
		},
		// No `client` (browser-mode) project yet, unlike app/vite.config.ts --
		// there is no Svelte component here to render a `.svelte.spec.ts`
		// against. `coverage.include` above still matches `src/lib/**/*.svelte`,
		// so the gate does not go quiet if one is added without this project:
		// an unexercised .svelte file reports 0% and fails the 100% threshold
		// rather than passing vacuously, forcing whoever adds the first real
		// component to add the client project too (verified empirically).
		projects: [
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
