// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	/**
	 * The app's build stamp (#1527): `vite.config.ts`'s own `define` swaps
	 * this identifier for a literal string at build time -- the deployed
	 * commit (`GITHUB_SHA`) in CI, `'dev'` everywhere else, including every
	 * spec. Declared inside this block, not at the file's own top level:
	 * the file ends in `export {}`, which makes every top-level `declare`
	 * a module-scoped export rather than a real global -- `#lib/build.ts`
	 * is the only reader, and it never imports one.
	 */
	declare const __APP_BUILD__: string;

	namespace App {
		/**
		 * What `error()` throws and a `+error.svelte` reads back off
		 * `page.error`. SvelteKit's own shape is `{ message }`; `code`
		 * is #918's addition -- the machine-readable reason a refusal
		 * carried, drawn from `apierr.ForbiddenCodes` for a 403, so the
		 * error page can render the state matching the reason rather
		 * than one state for every refusal alike.
		 *
		 * Optional, because most failures have no code to carry: a 404,
		 * a 500, anything SvelteKit itself throws. `errorKindForStatus`
		 * treats an absent code as the role refusal it always was.
		 */
		interface Error {
			message: string;
			code?: string;
		}
		// interface Locals {}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}
}

export {};
