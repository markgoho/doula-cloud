/*
 * Setting up a Practice: its name (#2049). One optional box under a
 * two-sentence hint, which is the longest thing on the page.
 */
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Setting up a Practice: its name',
	component: Page,
	params: {},
	url: 'https://example.test/signup/practice',
	readyText: 'What is your Practice called?'
};
