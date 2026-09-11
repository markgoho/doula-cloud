/*
 * The Practice landing hub, as the continuum check sees it (#595).
 *
 * A Doula role keeps `roster`/`credit`/`connect`/`requests` all
 * `undefined` (see `canReadRoster`/`canReadConnect` in
 * `practiceLanding.ts`), so on her branch only offers, the client-count
 * probe, the awaiting-reply roll-up and the activity feed ever fetch --
 * `respond` below answers the Owner's four blocks as well, for the reason
 * written beside them. The hub's title still carries #530's own URL as
 * the Practice's registered name, and an Offer's `terms` carries #537's
 * hyphenated double-barreled name where a Practice writes free text
 * about who it is offering the work to.
 *
 * That is one of the two trees this route draws, and it is the smaller
 * one (#928). `canReadRoster`/`canReadConnect` are the same
 * Owner-or-Admin pair, so an Owner and an Admin read the identical hub
 * and a Doula reads a hub with no `secondary` rail at all -- `hasSecondary`
 * withholds the region rather than drawing it empty. Two sessions, not
 * three: the Admin's tree is the Owner's tree, so a third variant would
 * mount a screen already measured.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Offer } from '#lib/offer.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../routeFixture.js';
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

/*
 * The hub with its `secondary` rail drawn -- Requests, Your people,
 * Credits and Getting paid, none of which a Doula is asked about and none
 * of which had ever been mounted at any width (#928). What it renders
 * that the base does not is this repo's own copy plus counts, so its
 * hostile content is in the counts themselves: the plural
 * expired-invitation Badge, the "30+ waiting" roll-up, the plural
 * Stripe-requirements sentence and the longest of the five Connect Badge
 * labels. Every one of those is chosen in `respond` below, where the
 * branch actually reads it.
 *
 * Owner rather than Admin only because a name has to say one of them. The
 * gates are `canReadRoster`/`canReadConnect`, both Owner-or-Admin, so the
 * two roles draw the same tree and the Admin's is not declared separately.
 */
export const asOwner: RouteVariant = {
	name: 'The Practice landing hub, as an Owner',
	// `isContractor` is not read here at all, so a contractor Doula and an
	// employee Doula meet the same hub -- the shared helper's own default
	// (`false`) is never overridden by either branch.
	pageData: practiceSession(['owner'], { practiceName })
};

export const fixture: RouteFixture = {
	name: 'The Practice landing hub, as a Doula',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1',
	pageData: practiceSession(['doula'], { practiceName }),
	respond: (path) => {
		if (path.includes('/offers')) {
			return jsonResponse({ items: offers });
		}
		// #455: the roll-up of Engagements whose thread's latest Message
		// came from the Client. One row long enough (the same double-barreled
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
		//
		// The second row is #1148's own state of the What column: a
		// Membership row, whose text is the action plus the person it
		// happened to, where every other subject kind's is the action
		// alone. Two rows because the column renders two ways, per this
		// repo's fixture rule -- and this one's name is long on purpose,
		// so the sweep measures a roster change against a real Practice's
		// longest member rather than a polite one.
		//
		// Said plainly, since this `respond` is shared with the Doula
		// variant below: a Doula never receives a Membership row in
		// production -- activitygate's membership Rule admits an Owner
		// and an Admin only -- so under that variant this row is a
		// layout subject rather than a screen she would meet.
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
					},
					{
						subjectKind: 'membership',
						subjectId: 'staff-2',
						subjectName: 'Persephone Abernathy-Okonkwo',
						action: 'employment_type_changed',
						actorKind: 'staff',
						actorName: 'Marguerite Vandenberg-Whitfield',
						createdAt: '2026-07-30T00:00:00Z'
					}
				],
				hasMore: false
			});
		}
		/*
		 * The four blocks only an Owner or Admin reaches (#928). They are
		 * answered on the base fixture rather than on the variant above,
		 * because `loadPracticeLanding` never asks for any of them under a
		 * Doula's roles -- and because `block()` in `practiceLanding.ts`
		 * SWALLOWS a throw and renders "Could not load ..." instead. A
		 * variant carrying its own narrower `respond` that missed one of
		 * these would mount a smaller rail than it declared and the sweep
		 * would stay green, which is the failure this fixture is here to
		 * stop.
		 */
		if (path.includes('/staff')) {
			return jsonResponse({
				// A 14-doula agency is the pilot's own size, and four
				// invitations that have run out is what puts the Badge on the
				// block at all -- its plural label is the roster block's
				// longest line.
				members: Array.from({ length: 14 }, () => ({})),
				invitations: {
					items: [
						{ expired: true },
						{ expired: true },
						{ expired: true },
						{ expired: true },
						{ expired: false },
						{ expired: false }
					]
				}
			});
		}
		if (path.includes('/engagement-requests')) {
			// A page's worth and more behind it, which is what makes the
			// Badge read "30+ waiting" -- the widest this label ever gets
			// (`RequestHealth.hasMore` exists for exactly this).
			return jsonResponse({
				items: Array.from({ length: 30 }, (_, index) => ({ requestId: `request-${index}` })),
				hasMore: true
			});
		}
		if (path.includes('/payments/connect')) {
			// `onboarding_incomplete` is the longest of the five Badge
			// labels this hub can draw, and three outstanding requirements
			// is what puts the plural sentence above it.
			return jsonResponse({
				status: 'onboarding_incomplete',
				cardPaymentsStatus: 'inactive',
				payoutsStatus: 'inactive',
				requirementsDue: [
					'individual.verification.document',
					'external_account',
					'business_profile.url'
				]
			});
		}
		if (path.includes('/billing')) {
			return jsonResponse({ balance: 1284, ledger: { items: [], hasMore: false } });
		}
		throw new Error(`practices/[practiceId] fixture: unmatched fetch path ${path}`);
	},
	readyText: `Welcome to ${practiceName}`,
	variants: [asOwner]
};
