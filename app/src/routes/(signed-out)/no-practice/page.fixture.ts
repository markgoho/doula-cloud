/*
 * The screen a signed-in identity with no Practice lands on, as the
 * continuum check sees it (#745).
 *
 * Its own probe is answered with the 404 that puts a person here in the
 * first place: a live session that resolves to no staff row. Nothing on
 * this screen is a Practice's own free text -- it is the one screen that
 * exists precisely because there is no Practice behind it -- so the
 * hostile content ADR-0025 asks for has nowhere to go here.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'The no-Practice landing screen',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/no-practice',
	respond: () => jsonResponse('no matching staff account', 404),
	readyText: 'Your account is not part of a Practice'
};
