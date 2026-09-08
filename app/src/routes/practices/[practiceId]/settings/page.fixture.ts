/*
 * The Settings hub, as the continuum check sees it (#595).
 *
 * A fixed nav (see the route's own header comment: "a way in, not a
 * settings design") -- no Practice-typed content, every label and
 * description is this repo's own copy. It reads `isOwner` and
 * `isOwnerOrAdmin` (#606, #744) to decide which entries to include, so
 * the list it shows depends on who is looking.
 *
 * Two of the three lists are declared (#913): the Owner's, which is the
 * widest, and a Doula's, which is the narrowest and the only one that
 * withholds the Getting paid entry. The Admin's sits between them and is
 * a strict subset of the Owner's -- it has the same Getting paid and
 * Blocked email addresses entries and none of the Owner-only three -- so
 * it realizes no entry the two branches below do not already measure,
 * and it is left undeclared rather than swept as a third copy.
 */
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import Page from './+page.svelte';

function session(roles: string[]) {
	return {
		session: {
			practiceId: 'practice-1',
			practiceName: 'Riverside Doula Collective',
			roles,
			isContractor: false
		}
	};
}

/*
 * The hub with the Getting paid entry withheld -- four entries rather
 * than eight, and the branch nothing had ever mounted. Its own longest
 * line is the Client Fields description, which the Owner's list holds
 * too; what differs here is which entries exist, not what any of them
 * says.
 */
export const asDoula: RouteVariant = {
	name: 'The Settings hub, as a Doula',
	pageData: session(['doula'])
};

export const fixture: RouteFixture = {
	name: 'The Settings hub, as an Owner',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings',
	pageData: session(['owner']),
	readyText: 'Settings',
	variants: [asDoula]
};
