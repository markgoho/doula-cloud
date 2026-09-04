/*
 * A Staff member's own Offers inbox, as the continuum check sees it
 * (#595).
 *
 * The same shape as the Practice hub's own Offers block, on its own
 * route: `terms` and `targetName` carry #537's vocabulary.
 *
 * Two Offers, not one (#720): `isOpen()` is the only thing this screen
 * branches on -- Client/Area/Due date and the Accept/Decline buttons
 * render only while an Offer is open -- so a fixture holding a single
 * open Offer never shows what a decided one looks like. The second
 * Offer is 'superseded', the longest of the five terminal labels
 * ("Taken by someone else"), with neither `amountCents` nor `terms`
 * set, so the Fee row's undefined-amount text and the absent Terms row
 * are both on screen too.
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
	},
	{
		offerId: 'offer-2',
		state: 'superseded',
		clientFirstInitial: 'A',
		clientArea: 'Rochester, NY',
		dueDate: '2027-02-01',
		employmentType: 'contractor',
		offeredAt: '2026-07-01T00:00:00Z',
		expiresAt: '2026-07-08T00:00:00Z',
		decidedAt: '2026-07-05T00:00:00Z',
		targetName: 'Persephone Ochieng-Whitfield'
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
