/*
 * The Client-portal login screen, as the continuum check sees it (#595).
 *
 * The on-mount probe (#283) finds no portal session, so the screen is the
 * form: the only thing this route draws. A live session is never this
 * screen -- the probe sends her to her Engagement or to `/` (#312) -- so
 * a fixture that answered with one sent the whole drag surface to `/`,
 * and the sweep measured a screen with no form on it (#2050). The spec
 * owns that redirect, with a session of its own.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import Page from './+page.svelte';

/*
 * #757: the same screen after her session ended, which is a different
 * tree -- the notice above the form, from the URL's own flag.
 */
export const afterSessionEnded: RouteVariant = {
	name: 'The Client-portal login screen, after a session ended',
	url: 'https://example.test/portal/(signed-out)/login?sessionEnded=true'
};

export const fixture: RouteFixture = {
	name: 'The Client-portal login screen',
	component: Page,
	params: {},
	url: 'https://example.test/portal/(signed-out)/login',
	respond: () => jsonResponse('no matching portal session', 404),
	readyText: 'Sign in',
	variants: [afterSessionEnded]
};
