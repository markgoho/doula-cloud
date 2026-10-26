/*
 * The first page of setting up a Practice, as the continuum check sees it
 * (#595, #2049): her name, email and password. The boxes show the
 * journey's draft, which `check/page.fixture.ts` fills with the longest
 * answers when the sweep has imported it, and which is empty otherwise.
 */
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Setting up a Practice: about you',
	component: Page,
	params: {},
	url: 'https://example.test/signup',
	readyText: 'About you'
};
