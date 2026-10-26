/*
 * Redeeming a Client sign-in link, as the continuum check sees it (#595).
 *
 * A token in the URL reaches the Continue button -- the only state that
 * mounts, since spending the link is behind a click the sweep never
 * simulates (#617's GET-then-POST shape: a GET spends nothing). Nothing
 * here fetches on mount.
 *
 * The link's address (#2025) is rendered in the lede, so it is the
 * longest a Client could have. It is in the fragment, as the mail sends
 * it, and a `+` in it is what the link's own escaping has to survive.
 */
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import Page from './+page.svelte';

/*
 * #2025: the same screen from a link with no token in it -- one plain
 * sentence and the next step, in place of the lede and Continue.
 */
export const linkNotComplete: RouteVariant = {
	name: 'Redeeming a Client sign-in link that is not complete',
	url: 'https://example.test/portal/(signed-out)/sign-in'
};

export const fixture: RouteFixture = {
	name: 'Redeeming a Client sign-in link',
	component: Page,
	params: {},
	url: 'https://example.test/portal/(signed-out)/sign-in?token=magic-link-token-1#email=anne-marie.ochieng-whitfield%2Bportal%40highland-midwifery-group.example.org',
	readyText: 'Sign in',
	variants: [linkNotComplete]
};
