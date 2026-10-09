/*
 * The Practice name screen, as the continuum check sees it (#1540).
 *
 * The name it opens on is deliberately long and unbroken enough to
 * stress a 320px column (ADR-0025): a business name a doula might really
 * register, with a long word in it. The screen reads `isOwner` and
 * renders two trees from it, so it declares both (#913): the Owner's
 * field and Save button, and the read-only name anyone else meets by
 * typing the address. An Admin's tree is the Doula's, so it is left out.
 */
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const hostileName =
	'Riverside Birth, Postpartum and Lactation Support Collective of the Finger Lakes Region';

export const asDoula: RouteVariant = {
	name: 'Practice name, as a Doula',
	pageData: practiceSession(['doula'], { practiceName: hostileName })
};

export const fixture: RouteFixture = {
	name: 'Practice name',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/name',
	pageData: practiceSession(['owner'], { practiceName: hostileName }),
	readyText: 'Practice name',
	variants: [asDoula]
};
