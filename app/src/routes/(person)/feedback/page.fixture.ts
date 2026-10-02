/*
 * The founder's list of Feedback, as the continuum check sees it (#1526).
 *
 * The values are the widest the screen ever holds. A route pattern has no
 * break opportunity a browser will take, a GitHub error is a sentence
 * with a URL in it, and the declared fixture has a row in each of the
 * two tables -- so the five-column "Issue not opened" table is measured,
 * not only the four-column list under it.
 */
import type { FeedbackSummary } from '#lib/founderFeedback.js';
import type { RouteFixture, RouteVariant } from '../../routeFixture.js';
import Page from './+page.svelte';
import type { FeedbackListData } from './+page.js';

const longRoute =
	'/(app)/practices/[practiceId]/engagements/[engagementId]/contract/invoices/[invoiceId]/payments/[paymentId]';

export const opened: FeedbackSummary = {
	id: 'feedback-1',
	kind: 'idea_or_request',
	routeId: '/(app)/practices/[practiceId]/clients',
	sentAt: '2026-10-01T09:00:00Z',
	issue: { state: 'opened', number: 12, url: 'https://github.com/markgoho/doula-cloud-feedback/issues/12', attempts: 0 }
};

export const waiting: FeedbackSummary = {
	id: 'feedback-2',
	kind: 'something_else',
	routeId: '/portal/(authenticated)/engagements/[engagementId]',
	sentAt: '2026-10-02T09:00:00Z',
	issue: { state: 'pending', attempts: 0 }
};

export const retrying: FeedbackSummary = {
	id: 'feedback-3',
	kind: 'not_working',
	routeId: longRoute,
	sentAt: '2026-10-03T09:00:00Z',
	issue: {
		state: 'retrying',
		attempts: 2,
		lastError:
			'feedback: create issue: POST https://api.github.com/repos/markgoho/doula-cloud-feedback/issues: 401 Bad credentials'
	}
};

export const deadLettered: FeedbackSummary = {
	id: 'feedback-4',
	kind: 'not_working',
	routeId: '/(person)/account',
	sentAt: '2026-10-04T09:00:00Z',
	issue: { state: 'dead_lettered', attempts: 5, lastError: 'feedback: create issue: 401 Bad credentials' }
};

export const data: FeedbackListData = {
	unopened: { items: [deadLettered, retrying], hasMore: false },
	all: { items: [deadLettered, retrying, waiting, opened], hasMore: true, nextCursor: 'cursor-1' }
};

/*
 * The screen on the day nothing has been sent: both tables say so in
 * words, and both headings stay.
 */
export const empty: RouteVariant = {
	name: 'The founder feedback list, with nothing sent yet',
	props: { data: { unopened: { items: [], hasMore: false }, all: { items: [], hasMore: false } } }
};

export const fixture: RouteFixture = {
	name: 'The founder feedback list',
	variants: [empty],
	component: Page,
	params: {},
	url: 'https://example.test/feedback',
	props: { data },
	// The first page comes from `load`, so the screen is there as soon as
	// it renders; the heading is what proves it is the screen and not an
	// error state.
	readyText: 'Feedback'
};
