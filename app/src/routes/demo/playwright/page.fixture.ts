/*
 * The Playwright e2e smoke page, as the continuum check sees it (#595).
 *
 * Fully static, no fetch, no Practice-typed content -- it exists only so
 * the e2e suite has a real page to load.
 */
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'The Playwright e2e smoke page',
	component: Page,
	params: {},
	url: 'https://example.test/demo/playwright',
	readyText: 'Playwright e2e test demo'
};
