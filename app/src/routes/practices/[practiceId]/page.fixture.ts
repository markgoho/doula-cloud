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

export const practiceName =
	'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake';

export const offers: Offer[] = [
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
	pageData: {
		session: { practiceId: 'practice-1', practiceName, roles: ['doula'], isContractor: false }
	},
	respond: (path) => {
		if (path.includes('/offers')) {
			return jsonResponse({ items: offers });
		}
		// #455: the roll-up of Engagements whose thread's latest Message
		// came from the Client. One row long enough (the same double-barrelled
		// name #537 already fixtures elsewhere) to exercise the block's own
		// overflow handling down to 320px (ADR-0024/0025).
		if (path.includes('/messages/awaiting-reply')) {
			return jsonResponse({
				items: [
					{
						engagementId: 'engagement-1',
						clientName: 'Anne-Marie Ochieng-Whitfield',
						lastMessageAt: '2026-08-01T00:00:00Z'
					}
				],
				hasMore: false
			});
		}
		if (path.includes('/clients')) {
			return jsonResponse({ items: [{ clientId: 'client-1' }] });
		}
		if (path.includes('/push-subscriptions')) {
			return jsonResponse({});
		}
		// #486: the Recent-activity feed, with one row long enough to
		// exercise the ledger's own overflow handling (ADR-0024/0025) --
		// #530's own URL again, this time as the diff a Practice's own
		// edit produced.
		if (path.includes('/activity')) {
			return jsonResponse({
				items: [
					{
						subjectKind: 'engagement',
						subjectId: 'engagement-1',
						action: 'contract_sent',
						actorKind: 'staff',
						actorName: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
						createdAt: '2026-08-01T00:00:00Z'
					}
				],
				hasMore: false
			});
		}
		throw new Error(`practices/[practiceId] fixture: unmatched fetch path ${path}`);
	},
	readyText: `Welcome to ${practiceName}`
};
