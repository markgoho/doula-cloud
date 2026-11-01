/*
 * The Client-portal Engagement hub, as the continuum check sees it
 * (#595).
 *
 * `practiceName` reaches this screen two ways at once: through the
 * ancestor `+layout.ts`'s `page.data` (`RecordDetail`'s `serviceName`)
 * and through this route's own `onMount` fetch (the summary's facts).
 * Both carry #530's own URL, since a Practice's registered name is
 * exactly the value that broke a grid track there.
 *
 * #2007 paints the Practice's name again, under the `<h1>`, as the
 * drawing has it -- so this fixture's hostile URL is measured on this
 * screen once more, and the line it sits in wraps anywhere.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const practiceName =
	'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake';

export const createdAt = '2026-03-12T20:00:00Z';

export const detail = {
	engagementId: 'engagement-1',
	practiceName,
	clientName: 'Anne-Marie Ochieng-Whitfield',
	dueDate: '2027-03-01',
	offersBirthPlan: true,
	isOneOfSeveral: false,
	careEnded: false,
	totalToPayCents: 335_000,
	/*
	 * #2007's three cards, each at its longest: a signed Contract, a
	 * Birth Plan she has confirmed, and a Message whose body is one long
	 * unbreakable run beside a double-barreled sender's name. Their
	 * other states are the spec's departures from this record.
	 */
	contract: { status: 'signed', signedAt: '2026-08-14T15:00:00Z' },
	birthPlan: { acknowledgedAt: '2026-08-20T12:00:00Z' },
	lastMessage: {
		messageId: 'message-1',
		senderType: 'staff',
		senderName: 'Marguerite Ashworth-Delacroix-Whitfield',
		body: 'https://maps.example.org/directions/to/the/birth-center/entrance?from=parking-garage-level-3',
		createdAt: '2026-08-28T15:00:00Z'
	}
};

/*
 * #478's own two rows, and they are the row-set split ADR-0025 asks for
 * rather than two examples of the same thing: one scheduled Visit with
 * every field at its busiest -- a hyphenated double-barreled Doula name
 * a real Practice employs -- and one past Visit, which is the
 * `hasHappened` flag's other state and the other of the two `When` formats. A fixture
 * with only the upcoming row would sweep one of the two strings this
 * section can render.
 */
export const visits = [
	{
		visitId: 'visit-upcoming',
		scheduledAt: '2027-02-18T19:00:00Z',
		doulaName: 'Marguerite Ashworth-Delacroix-Whitfield',
		hasHappened: false
	},
	{ visitId: 'visit-past', scheduledAt: '2026-08-18T14:30:00Z', doulaName: 'Priya Raman', hasHappened: true }
];

/*
 * #708's own row-set split, the same shape the Visits above take. The
 * ledger used to answer this route's `/activity` read with an empty page,
 * and once the What column started carrying a whole sentence rather than
 * an action string, an empty ledger stopped standing in for the real one.
 *
 * These rows are what checks the ledger's layout, and since #710 they do
 * it through the shared sweep rather than beside it. This comment used to
 * say the opposite -- `route-continuum`'s sweep never opened the
 * disclosure, a closed `<details>` renders nothing at all to measure, and
 * the open-state sweep lived in `engagement-hub.svelte.spec.ts` on a row
 * of its own. The sweep now opens every closed disclosure under the frame
 * before it measures, so these rows are the ones it lays out at 320px and
 * the private copy is gone.
 *
 * Three rows, because three things about this column can be worst-case:
 * the longest phrase the Client register holds (`invoice_payment_failed`,
 * pinned in `activityPhrases.usage.spec.ts`), the longest Who a Client
 * reads ("Your practice", on `payment_reversed`), and the longest single
 * unbreakable word among them, which is what a 320px track actually
 * cannot split -- "notifications", on a row whose actor is her own name
 * rather than "Your practice", so the Who column is swept under a real
 * Client's name too.
 */
export const activity = [
	{
		subjectKind: 'engagement',
		subjectId: detail.engagementId,
		action: 'invoice_payment_failed',
		actorKind: 'client',
		actorName: detail.clientName,
		createdAt: '2026-08-31T09:00:00Z'
	},
	{
		subjectKind: 'engagement',
		subjectId: detail.engagementId,
		action: 'payment_reversed',
		actorKind: 'staff',
		actorName: 'Your practice',
		createdAt: '2026-08-30T15:00:00Z'
	},
	{
		subjectKind: 'engagement',
		subjectId: detail.engagementId,
		action: 'push_notifications_enabled',
		actorKind: 'client',
		actorName: detail.clientName,
		createdAt: '2026-08-29T11:30:00Z'
	}
];

export const fixture: RouteFixture = {
	name: 'The Client-portal Engagement hub',
	component: Page,
	params: { engagementId: 'engagement-1' },
	url: 'https://example.test/portal/engagements/engagement-1',
	// #310: the ancestor `+layout.ts` load now carries `createdAt` too --
	// `+page.svelte` builds `RecordDetail`'s `serviceName` from it via
	// `engagementLabel`, the same way the real load chain does.
	pageData: { practiceName, createdAt },
	// #486: the Activity ledger's own read shares this route's one mocked
	// fetcher, and needs the cursor-list envelope docs/api-design.md
	// section 4 asks for -- the bare `detail` shape above has no `items`,
	// which is exactly what crashed DataTable when this fixture answered
	// every path with it alike. #708 gave it real rows; see `activity`.
	respond: (path) => {
		if (path.includes('/activity')) return jsonResponse({ items: activity, hasMore: false });
		if (path.includes('/visits')) return jsonResponse({ items: visits, hasMore: false });
		return jsonResponse(detail);
	},
	readyText: 'Your care'
};
