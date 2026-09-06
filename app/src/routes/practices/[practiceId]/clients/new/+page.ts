/*
 * `clients/new` is not a page any more (#466): intake is one question
 * per route, and this is the door the search that fronts it already
 * links to (ADR-0017, #498). It sends the reader to the first question
 * and carries the search's own query string with it, so what was typed
 * into the search still seeds the draft.
 */
import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ params, url }) => {
	redirect(
		307,
		`${resolve('/practices/[practiceId]/clients/new/name', { practiceId: params.practiceId })}${url.search}`
	);
};
