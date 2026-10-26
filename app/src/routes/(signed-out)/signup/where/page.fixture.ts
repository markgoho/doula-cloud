/*
 * Setting up a Practice: where she works (#2049). The state select and
 * the timezone select, whose longest option, "Mountain time, no daylight
 * saving (Phoenix)", is what sizes the column.
 */
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Setting up a Practice: where you work',
	component: Page,
	params: {},
	url: 'https://example.test/signup/where',
	readyText: 'Where do you work?'
};
