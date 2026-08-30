/**
 * Which of the four GOV.UK-aligned error states `templates/ErrorPage.svelte`
 * renders (#471). `unavailable` has no dedicated meaning in the HTTP spec
 * beyond 503 -- there is no way to tell "down for planned work" from "down
 * because it crashed" except by the status code the failing `load`
 * deliberately chooses.
 */
export type ErrorKind = 'notFound' | 'refused' | 'unavailable' | 'problem';

/** Maps the status SvelteKit hands a `+error.svelte` to the error state it
 * renders. Anything not named below -- 500, or any other unexpected status
 * -- is `problem`: an unplanned failure, not a permission or existence one. */
export function errorKindForStatus(status: number): ErrorKind {
	switch (status) {
		case 404: {
			return 'notFound';
		}
		case 403: {
			return 'refused';
		}
		case 503: {
			return 'unavailable';
		}
		default: {
			return 'problem';
		}
	}
}
