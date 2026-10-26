/*
 * Your account's name question (#2044), as the continuum check sees it.
 * The fields fill from the same session the summary reads, with its
 * hyphenated double-barreled name (#537).
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../routeFixture.js';
import { session } from '../page.fixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Your account, changing your name',
	component: Page,
	params: {},
	url: 'https://example.test/account/name',
	respond: () => jsonResponse(session),
	readyText: 'What is your name?'
};
