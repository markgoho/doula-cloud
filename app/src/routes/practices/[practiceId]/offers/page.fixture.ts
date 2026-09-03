/*
 * A Staff member's own Offers inbox, as the continuum check sees it
 * (#595).
 *
 * The same shape as the Practice hub's own Offers block, on its own
 * route: `terms` and `targetName` carry #537's vocabulary.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Offer } from '#lib/offer.js';
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

const offers: Offer[] = [
	{
		offerId: 'offer-1',
		state: 'offered',
		clientFirstInitial: 'P',
		clientArea: 'Rochester, NY',
		dueDate: '2027-03-01',
		amountCents: 450_000,
		terms: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
		employmentType: 'contractor',
		offeredAt: '2026-08-01T00:00:00Z',
		expiresAt: '2026-08-08T00:00:00Z',
		targetName: 'Anne-Marie Ochieng-Whitfield'
	}
];

export const fixture: RouteFixture = {
	name: "A Staff member's own Offers inbox",
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/offers',
	respond: () => jsonResponse({ items: offers }),
	readyText: 'Your offers'
};
