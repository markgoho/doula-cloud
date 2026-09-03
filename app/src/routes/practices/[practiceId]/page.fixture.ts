/*
 * The Practice landing hub, as the continuum check sees it (#595).
 *
 * A Doula role keeps `roster`/`credit`/`connect`/`requests` all
 * `undefined` (see `canReadRoster`/`canReadConnect` in
 * `practiceLanding.ts`), so only session, offers and the client-count
 * probe ever fetch -- the hub's title still carries #530's own URL as
 * the Practice's registered name, and an Offer's `terms` carries #537's
 * hyphenated double-barrelled name where a Practice writes free text
 * about who it is offering the work to.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Offer } from '#lib/offer.js';
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

const practiceName =
	'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake';

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
	name: 'The Practice landing hub',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1',
	respond: (path) => {
		if (path.endsWith('/session')) {
			return jsonResponse({ practiceName, roles: ['doula'] });
		}
		if (path.includes('/offers')) {
			return jsonResponse({ items: offers });
		}
		if (path.includes('/clients')) {
			return jsonResponse({ items: [{ clientId: 'client-1' }] });
		}
		if (path.includes('/push-subscriptions')) {
			return jsonResponse({});
		}
		throw new Error(`practices/[practiceId] fixture: unmatched fetch path ${path}`);
	},
	readyText: `Welcome to ${practiceName}`
};
