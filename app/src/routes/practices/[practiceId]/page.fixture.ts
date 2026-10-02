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
 * That is one of the two trees the populated hub draws, and it is the
 * smaller one (#928). `canReadRoster`/`canReadConnect` are the same
 * Owner-or-Admin pair, so an Owner and an Admin read the identical hub
 * and a Doula reads a hub with no `secondary` rail at all -- `hasSecondary`
 * withholds the region rather than drawing it empty. The Admin's tree is
 * the Owner's tree, so a variant for her would mount a screen already
 * measured.
 *
 * Three subjects are declared (#1621): the Doula's hub (the base), the
 * Owner's hub (`asOwner`), and the empty Practice of an Owner who has no
 * Client yet (`asOwnerWithNoClient`). The first two differ by session and
 * share one `respond`. The third differs by what the Practice holds, so
 * it has a `respond` of its own; the comment beside it says which way
 * that went and why the Doula's empty tree is not a fourth subject.
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

/*
 * Each read the hub makes on a Practice that has a Client, answered by
 * path. It is a named function and not a property of `fixture` because
 * two subjects call it (#1621): the base fixture, and through it
 * `asOwner`, use it as their whole `respond`; `asOwnerWithNoClient`
 * below answers one path itself and hands each other path to it.
 */
function respondForPopulatedPractice(path: string): Response {
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
	// Said plainly, since these answers are shared with the Doula's
	// session, which is the base fixture: a Doula never receives a
	// Membership row in production -- activitygate's membership Rule
	// admits an Owner and an Admin only -- so under her session this
	// row is a layout subject rather than a screen she would meet.
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
}

/*
 * The empty Practice: the first screen a new Owner sees after signup, and
 * a tree that neither subject above draws (#1621). `hasAnyClient` reads
 * no Client, so `OverviewHub` draws its `empty` snippet in place of
 * `primary`, `secondary` and `feed`: the level-1 heading, one sentence
 * and one link. The hostile content of this screen is the heading, so
 * the session keeps the fixture's own `practiceName`.
 *
 * This variant is a state of the data and not a second caller. The empty
 * state replaces the whole tree, so one mount cannot hold an empty
 * Practice and a populated one, and ADR-0025's row-set rule (an array's
 * empty and non-empty state) is met by a variant here.
 *
 * Which way `respond` went: this variant carries its own, because the one
 * answer that differs is in `respond`, and a variant's `respond` is a
 * replacement and not a merge. It answers `/clients` itself and hands
 * each other path to the base's function. The hub makes the other eight
 * reads for an Owner on an empty Practice too (`/offers`, `/staff`,
 * `/billing`, `/payments/connect`, `/engagement-requests`, `/activity`,
 * `/messages/awaiting-reply`, `/push-subscriptions`), and `block()`
 * swallows an unanswered one. The empty state draws none of them today.
 * #1612 puts the Credit balance into it, and hides that sentence when
 * `/billing` cannot be read; a narrower `respond` here would then mount a
 * shorter screen than the Owner's and the sweep would stay green (#928).
 * No second copy of the eight answers exists: a path that a later ticket
 * adds to the base is answered here with no second edit, and an unmatched
 * path still throws the base's error.
 *
 * The match is on the end of the path (`/clients`, then a query or
 * nothing), not on `includes('/clients')`, so a later path below
 * `/clients/` goes to the base and is not answered as an empty list.
 *
 * No variant for a Doula on an empty Practice, on purpose. The `empty`
 * snippet does not read the session, so her tree is this tree. After
 * #1612 her sentence has no balance, which makes her tree a strict subset
 * of this one.
 */
export const asOwnerWithNoClient: RouteVariant = {
	name: 'The Practice landing hub, as an Owner with no Client yet',
	pageData: practiceSession(['owner'], { practiceName }),
	respond: (path) =>
		/\/clients(\?|$)/.test(path) ? jsonResponse({ items: [] }) : respondForPopulatedPractice(path)
};

export const fixture: RouteFixture = {
	name: 'The Practice landing hub, as a Doula',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1',
	pageData: practiceSession(['doula'], { practiceName }),
	respond: respondForPopulatedPractice,
	readyText: `Welcome to ${practiceName}`,
	variants: [asOwner, asOwnerWithNoClient]
};
