/*
 * Confirming a changed Client sign-in address (#619), as the continuum
 * check sees it (#595).
 *
 * A token in the URL reaches the Continue button -- the only state that
 * mounts, since spending the link is behind a click the sweep never
 * simulates (ADR-0026's GET-then-POST shape: a GET spends nothing).
 * Nothing here fetches on mount.
 *
 * The new address (#2025) is rendered in the lede, so it is the longest
 * a Client could type. It is in the fragment, as the mail sends it, and
 * a `+` in it is what the link's own escaping has to survive.
 */
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import Page from './+page.svelte';

/*
 * #2025: the same screen from a link with no token in it -- one plain
 * sentence and the next step, in place of the lede and Continue.
 */
export const linkNotComplete: RouteVariant = {
	name: 'Confirming a changed Client sign-in address, from a link that is not complete',
	url: 'https://example.test/portal/(signed-out)/confirm-sign-in-address'
};

export const fixture: RouteFixture = {
	name: 'Confirming a changed Client sign-in address',
	component: Page,
	params: {},
	url: 'https://example.test/portal/(signed-out)/confirm-sign-in-address?token=address-change-token-1#email=anne-marie.ochieng-whitfield%2Bnew-address%40highland-midwifery-group.example.org',
	readyText: 'Confirm your sign-in address',
	variants: [linkNotComplete]
};
