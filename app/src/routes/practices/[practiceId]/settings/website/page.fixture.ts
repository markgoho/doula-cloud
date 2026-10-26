/*
 * The Website settings screen, as the continuum check sees it (#595).
 *
 * A `mode: 'hosted'` Practice that has already answered lands on the
 * "saved" step, whose `SummaryList` shows exactly the free text this
 * screen collects: what she offers and her cancellation policy each
 * carry #530's own URL, since a Practice could paste a referral link
 * into either.
 *
 * Two sessions, three trees (#928). `isOwner` decides three things here, and one of
 * them is additive: anybody else reads a Notice in the saved frame, where
 * an Owner's Change button is, saying the setting is not hers to change
 * (#1971). The other two only withhold: the Change button, and, for a
 * Practice that has not answered, the questions themselves (#1968), so a
 * non-Owner never meets a Save or Publish page she cannot press. That
 * undeclared non-Owner tree draws a sentence no other branch draws ("Not
 * answered yet"), so it is swept as its own variant (#1971) rather than
 * taken as a subset of the answered one.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeWebsite } from '#lib/website.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../../routeFixture.js';
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
 * Owner's -- with the Change button gone and a Notice in its place,
 * so this branch inherits `respond` deliberately rather than by omission.
 * Its own new line is that Notice, which is this repo's copy.
 */
export const nonOwner: RouteVariant = {
	name: 'The Website settings screen, as a non-Owner',
	pageData: practiceSession(['admin'])
};

/*
 * A Practice that has not answered yet, as the server sends it: every
 * field empty, `mode: 'undeclared'`.
 */
export const undeclaredWebsite: PracticeWebsite = {
	mode: 'undeclared',
	ownUrl: '',
	serviceDescription: '',
	cancellationPolicy: '',
	updatedBy: '',
	updatedAt: '',
	pageState: '',
	pageCheckedAt: '',
	pageCheckDetail: '',
	pageUrl: ''
};

/*
 * A non-Owner at a Practice that has not answered (#1968, #1971). No
 * Owner reaches this tree: an Owner with no answer is on the questions.
 * Its own `respond`, since the unanswered body is the branch's whole point.
 */
export const nonOwnerUnanswered: RouteVariant = {
	name: 'The Website settings screen, as a non-Owner, not answered yet',
	pageData: practiceSession(['admin']),
	respond: () => jsonResponse(undeclaredWebsite)
};

export const fixture: RouteFixture = {
	name: 'The Website settings screen, as an Owner',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/website',
	pageData: practiceSession(['owner']),
	respond: () => jsonResponse(website),
	readyText: 'Your website',
	variants: [nonOwner, nonOwnerUnanswered]
};
