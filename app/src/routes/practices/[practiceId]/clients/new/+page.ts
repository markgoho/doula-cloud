/*
 * `clients/new` is not a page any more (#466): intake is one question
 * per route, and this is the door the search that fronts it already
 * links to (ADR-0017, #498). It sends the reader to the first question
 * and carries the search's own query string with it, so what was typed
 * into the search still seeds the draft.
 *
 * ## Why the draft is cleared here
 *
 * This URL is the door, and nothing else is. A reader reaches it by
 * searching and finding nobody, so arriving here is the start of a new
 * Client rather than the middle of one -- every question has its own
 * URL, and walking back through the sequence never passes through this
 * one.
 *
 * Without this, the second Client of the day was the first one again:
 * `intakeDraft.start` seeds the query string only into a draft with
 * nothing in it, and the draft is mirrored to sessionStorage, so a
 * search for Anne after a search for Sarah reopened Sarah's half-typed
 * record. Clearing at the door is what makes "the empty state carries
 * whatever was typed" true on the second attempt as well as the first.
 */
import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { intakeDraft } from '#lib/intakeDraft.svelte.js';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ params, url }) => {
	intakeDraft.practiceId = params.practiceId;
	intakeDraft.clear();
	redirect(
		307,
		`${resolve('/practices/[practiceId]/clients/new/name', { practiceId: params.practiceId })}${url.search}`
	);
};
