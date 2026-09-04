/*
 * The approval screen, as the continuum check sees it (#570).
 *
 * This is the screen the baseline trial on #521 shipped: a fresh session,
 * unprompted, composed it from `RecordDetail`, `stack-l` and
 * `DescriptionList`, wrote no CSS at all, and put it 93px past its own
 * edge at 320px. Nothing in this repo could tell -- the check sweeps the
 * component demo registry, and a route is not in it.
 *
 * The Note carries the URL #530 measured, which is the value that breaks
 * it. That is deliberate and it is the point: a Practice pastes a referral
 * link into a free-text field, and #537's own finding is that a polite
 * fixture measures a screen nobody will ever see.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ApprovalDetail } from '#lib/engagementRequest.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

/*
 * #537's vocabulary, not this file's own invention: the hyphenated
 * double-barrelled name and the URL are the same values every style-guide
 * page carries, so the sweep is measuring one Practice's content rather
 * than 34 fixtures each guessing at what "long" means.
 */
export const detail: ApprovalDetail = {
	requestId: 'request-1',
	state: 'pending',
	kind: 'birth',
	dueDate: '2027-03-01',
	note: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
	requestedBy: 'staff-1',
	requestedByName: 'Anne-Marie Ochieng-Whitfield',
	requestedAt: '2026-08-01T10:00:00Z',
	client: {
		clientId: 'client-1',
		givenName: 'Persephone',
		familyName: 'Ochieng-Whitfield',
		preferredName: '',
		isNewToPractice: true
	},
	creditCost: 1,
	balance: 3,
	balanceAfter: 2,
	engagements: []
};

export const fixture: RouteFixture = {
	name: 'The approval screen',
	component: Page,
	params: { practiceId: 'practice-1', requestId: 'request-1' },
	url: 'https://example.test/practices/practice-1/engagement-requests/request-1',
	respond: () => jsonResponse(detail),
	// The Template draws a Skeleton until the load returns, so the heading
	// is what says the screen itself is on the page.
	readyText: 'Approve work with Persephone Ochieng-Whitfield'
};
