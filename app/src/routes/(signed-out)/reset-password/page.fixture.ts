/*
 * Reset-password, as the continuum check sees it (#595).
 *
 * A short form with no fetch on mount and no Practice-typed content:
 * the token it reads off the URL is never rendered. The link's email
 * (#1813) is rendered into a visible, read-only Email field, the username
 * a password manager saves the new password against; a `+` in it is what
 * the link's own escaping has to survive.
 * It is in the fragment, not the query string (#1827), as the mail sends it.
 */
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Reset your password',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/reset-password?token=reset-token-1#email=anne-marie%2Breset%40example.test',
	readyText: 'Reset your password'
};
