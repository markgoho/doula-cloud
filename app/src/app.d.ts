// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
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
