import { error } from '@sveltejs/kit';
import { parseRefusal } from './formErrors.js';

/**
 * Which of the GOV.UK-aligned error states `templates/ErrorPage.svelte`
 * renders (#471). `unavailable` has no dedicated meaning in the HTTP spec
 * beyond 503 -- there is no way to tell "down for planned work" from "down
 * because it crashed" except by the status code the failing `load`
 * deliberately chooses.
 *
 * Three of these are a 403 (#918). A status code carries no cause, so
 * keying the page off it alone made every refusal assert the one cause
 * 403 cannot name -- a role -- including the two the BFF already answers
 * that are nothing to do with the reader's role. The cause travels in
 * `APIError.code` instead, and `apierr.ForbiddenCodes` is the closed set
 * of reasons a 403 may carry (docs/api-design.md section 7, rule 6).
 */
export type ErrorKind =
	| 'notFound'
	| 'refused'
	| 'practiceLocked'
	| 'secondFactor'
	| 'unavailable'
	| 'problem';

/**
 * The 403 reason codes this app renders a state of its own for, mapped
 * onto that state. Deliberately not exhaustive over `apierr.Code`: it
 * holds only what `apierr.ForbiddenCodes` holds, and `FORBIDDEN` is
 * absent because it is the fallback below rather than an entry here --
 * one place decides what an unnamed refusal looks like, and an
 * unrecognized code has to land there too.
 */
const kindByRefusalCode: Record<string, ErrorKind> = {
	PRACTICE_PENDING_DELETION: 'practiceLocked',
	MFA_REQUIRED: 'secondFactor'
};

/** Maps the status SvelteKit hands a `+error.svelte`, and the refusal code
 * the failing `load` carried through with it, to the error state it
 * renders. Anything not named below -- 500, or any other unexpected status
 * -- is `problem`: an unplanned failure, not a permission or existence one.
 *
 * A 403 reads its reason from `code`. An absent code, or one this app does
 * not recognize, falls back to `refused` -- the role refusal, which is what
 * every 403 said before #918 and is still the commonest one by far. That
 * fallback is what lets a `load` that has nothing to say about a refusal go
 * on saying nothing. */
export function errorKindForStatus(status: number, code?: string): ErrorKind {
	switch (status) {
		case 404: {
			return 'notFound';
		}
		case 403: {
			return (code && kindByRefusalCode[code]) || 'refused';
		}
		case 503: {
			return 'unavailable';
		}
		default: {
			return 'problem';
		}
	}
}

/**
 * What a `load` says instead of `error(403, 'not permitted to read this')`
 * (#918).
 *
 * That sentence threw away the only thing that knew why the read was
 * refused. This reads the BFF's own `{code, message}` envelope and hands
 * both to SvelteKit, so the `+error.svelte` above can ask
 * `errorKindForStatus` for the state matching the reason rather than the
 * state matching the status. Always throws.
 *
 * `opaque5xx: false` because the caller has already decided this response
 * is a refusal, not a service failure -- there is no 5xx to collapse, and
 * the overload it selects is the one that cannot return `undefined`.
 *
 * `message` is what SvelteKit needs `App.Error` to carry, not what the
 * page shows: `ErrorPage` renders copy per kind and never the server's
 * prose. A refusal whose body carries no message keeps the sentence the
 * loaders used to throw, so nothing that reads `page.error.message` -- a
 * dev overlay, a log line -- gets an empty string.
 */
export async function refuseRead(response: Response): Promise<never> {
	const parsed = await parseRefusal(response, { opaque5xx: false });
	error(response.status, {
		message: parsed.message ?? 'not permitted to read this',
		code: parsed.code
	});
}
