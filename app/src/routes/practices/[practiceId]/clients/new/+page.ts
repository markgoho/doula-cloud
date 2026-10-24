/*
 * `clients/new` is not a page any more (#466): intake is one question
 * per route, and this is the door to it (ADR-0017, #498). The search
 * links here on a miss; while a Practice holds no Client record, the
 * overview and the Clients list link here directly (#1609). It sends the
 * reader to the first question, and the screen that opened intake
 * (`from`) rides along in the query string so it still decides where
 * Back goes.
 *
 * ## Why the draft is cleared here
 *
 * This URL is the door, and nothing else is. A reader reaches it to
 * start a new Client rather than in the middle of one -- every question
 * has its own URL, and walking back through the sequence never passes
 * through this one.
 *
 * Without this, the second Client of the day was the first one again:
 * the draft is mirrored to sessionStorage, so a search for Anne after a
 * search for Sarah reopened Sarah's half-typed record.
 *
 * ## Why the search's values are not read here
 *
 * What the search typed is a Client's name, date of birth, email and
 * phone. Carried as query params (#498's first shape) they were in the
 * address bar, the browser history and any reload's request (#1829). The
 * search now hands them over in module memory (`#lib/intakeHandoff.ts`),
 * and the layout takes them as the name question mounts, not this load:
 * SvelteKit runs a load to preload a link the pointer rests on, before
 * the click that hands the values over.
 */
import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { intakeDraft } from '#lib/intakeDraft.svelte.js';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ params, url }) => {
	intakeDraft.scope = params.practiceId;
	intakeDraft.clear();
	redirect(
		307,
		`${resolve('/practices/[practiceId]/clients/new/name', { practiceId: params.practiceId })}${url.search}`
	);
};
