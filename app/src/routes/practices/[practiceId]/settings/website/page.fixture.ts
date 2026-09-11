/*
 * The Website settings screen, as the continuum check sees it (#595).
 *
 * A `mode: 'hosted'` Practice that has already answered lands on the
 * "saved" step, whose `DescriptionList` shows exactly the free text this
 * screen collects: what she offers and her cancellation policy each
 * carry #530's own URL, since a Practice could paste a referral link
 * into either.
 *
 * Two sessions (#928). `isOwner` decides three things here, and one of
 * them is additive: anybody else reads a Notice above the whole screen
 * saying the setting is not hers to change, which no Owner branch draws.
 * The other two only withhold -- the Change button and the Publish
 * button's enabled state.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeWebsite } from '#lib/website.js';
import type { RouteFixture, RouteVariant } from '../../../../routeFixture.js';
import { practiceSession } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const website: PracticeWebsite = {
	mode: 'hosted',
	ownUrl: '',
	serviceDescription: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
	cancellationPolicy: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
	updatedBy: 'Anne-Marie Ochieng-Whitfield',
	updatedAt: '2026-08-01T00:00:00Z',
	pageState: 'live',
	pageCheckedAt: '2026-08-01T00:00:00Z',
	pageCheckDetail: '',
	pageUrl: 'https://doula.cloud/p/riverside-doula-collective'
};

/*
 * Everyone who is not an Owner, Admin included. She reads the same saved
 * answers -- the two pasted URLs above are hers to read as much as the
 * Owner's -- with the Change button gone and a Notice added above them,
 * so this branch inherits `respond` deliberately rather than by omission.
 * Its own new line is that Notice, which is this repo's copy.
 */
export const nonOwner: RouteVariant = {
	name: 'The Website settings screen, as a non-Owner',
	pageData: practiceSession(['admin'])
};

export const fixture: RouteFixture = {
	name: 'The Website settings screen, as an Owner',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/website',
	pageData: practiceSession(['owner']),
	respond: () => jsonResponse(website),
	readyText: 'Your website',
	variants: [nonOwner]
};
