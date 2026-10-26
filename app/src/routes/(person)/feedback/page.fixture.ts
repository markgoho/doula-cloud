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

/*
 * Typical pilot data (#1981), the screen this route's two table floors are
 * measured on. Feedback from a 14-doula agency's first weeks names the
 * screens its people use every day -- the Clients list, an Engagement, the
 * schedule, the portal's Engagement -- and a failure to open its issue is
 * the one GitHub answer this repo has seen, a credentials refusal, without
 * a URL in it. The longest route pattern and the error with a URL in it
 * stay on the hostile screen above.
 */
function typicalPiece(index: number, routeId: string, kind: FeedbackSummary['kind']): FeedbackSummary {
	return {
		id: `feedback-${index}`,
		kind,
		routeId,
		sentAt: `2026-10-${String(4 + index).padStart(2, '0')}T15:00:00Z`,
		issue: { state: 'opened', number: 40 + index, url: `https://github.com/markgoho/doula-cloud-feedback/issues/${40 + index}`, attempts: 0 }
	};
}

const typicalUnopened: FeedbackSummary = {
	...typicalPiece(1, '/(app)/practices/[practiceId]/engagements/[engagementId]', 'not_working'),
	issue: { state: 'dead_lettered', attempts: 5, lastError: 'feedback: create issue: 401 Bad credentials' }
};

const typicalAll: FeedbackSummary[] = [
	typicalUnopened,
	typicalPiece(2, '/(app)/practices/[practiceId]/clients', 'idea_or_request'),
	typicalPiece(3, '/(app)/practices/[practiceId]/schedule', 'not_working'),
	typicalPiece(4, '/portal/(authenticated)/engagements/[engagementId]', 'something_else')
];

export const typical: RouteVariant = {
	name: 'The founder feedback list, with typical pilot data',
	typical: true,
	props: {
		data: {
			unopened: { items: [typicalUnopened], hasMore: false },
			all: { items: typicalAll, hasMore: false }
		} satisfies FeedbackListData
	}
};

export const fixture: RouteFixture = {
	name: 'The founder feedback list',
	variants: [empty, typical],
	component: Page,
	params: {},
	url: 'https://example.test/feedback',
	props: { data },
	// The first page comes from `load`, so the screen is there as soon as
	// it renders; the heading is what proves it is the screen and not an
	// error state.
	readyText: 'Feedback'
};
