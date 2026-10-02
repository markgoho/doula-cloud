/*
 * One piece of Feedback, as the continuum check sees it (#1526).
 *
 * Everything on this page but its headings is something a person or a
 * machine typed: the free text, the full URL, the route pattern, a
 * Practice's name and a sender's. So the declared fixture carries #537's
 * vocabulary -- a pasted URL with no break opportunity inside the text
 * and as the page address, and a hyphenated double-barreled name.
 */
import type { FeedbackPiece } from '#lib/founderFeedback.js';
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './+page.svelte';

export const piece: FeedbackPiece = {
	id: 'feedback-1',
	kind: 'not_working',
	text: 'The invoice total is wrong on this page.\nI checked it against https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake and it does not match.',
	pageUrl:
		'/practices/6f1c2c0e-58c5-4d39-9c2b-0e7c1f0a9b11/engagements/0b8e7d2a-41f6-4c1d-8a55-9d3f2e6c7b10/contract/invoices',
	routeId: '/(app)/practices/[practiceId]/engagements/[engagementId]/contract/invoices',
	appBuild: 'abc1234',
	screenWidth: 390,
	browser: 'Safari 18',
	sentAt: '2026-10-20T08:15:00Z',
	role: 'owner, admin, doula',
	practiceName: 'Riverside Doula Collective',
	sender: { kind: 'staff', name: 'Anne-Marie Ochieng-Whitfield', email: 'anne-marie@example.test' },
	issue: { state: 'opened', number: 12, url: 'https://github.com/markgoho/doula-cloud-feedback/issues/12', attempts: 0 }
};

/*
 * The other tree this page renders: a Client's piece with nothing typed,
 * no Practice in context, and an issue that never opened -- so the empty
 * sentence, the "none" values and the last error are measured too.
 */
export const unopenedClientPiece: FeedbackPiece = {
	...piece,
	text: '',
	role: 'Client',
	practiceName: undefined,
	sender: { kind: 'client', email: 'priya.signin@example.test' },
	issue: {
		state: 'dead_lettered',
		attempts: 5,
		lastError:
			'feedback: create issue: POST https://api.github.com/repos/markgoho/doula-cloud-feedback/issues: 401 Bad credentials'
	}
};

export const fromAClientWithNoIssue: RouteVariant<RouteParameters> = {
	name: 'One piece of Feedback, from a Client, whose issue did not open',
	props: { data: { piece: unopenedClientPiece } }
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'One piece of Feedback',
	variants: [fromAClientWithNoIssue],
	component: Page,
	params: { feedbackId: 'feedback-1' },
	url: 'https://example.test/feedback/feedback-1',
	props: { data: { piece } },
	// The title ends in the instant the piece was sent, printed in the
	// reader's own zone, so only its fixed start is named here.
	readyText: 'Feedback sent'
};
