/*
 * The Staff-side Engagement detail hub, as the continuum check sees it
 * (#595).
 *
 * `onMount` cascades through detail, Visits, Messages, both Plan
 * sections, the Contract, Invoices and (Owner/Admin only) Offers, in
 * that order (see the route's own `onMount`) -- `respond` answers every
 * one of them so the cascade completes rather than stalling partway
 * through. `detail.clientName` is the title, gated on load exactly like
 * the existing approval-screen fixture, and carries #537's hyphenated
 * double-barrelled name; the Contract's merge-field values and a Visit's
 * `staffName` carry it again, since both are exactly the free-text shape
 * `DataTable`/`ContractView` were already fixed against.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const clientName = 'Anne-Marie Ochieng-Whitfield';

const detail = {
	engagementId: 'engagement-1',
	clientId: 'client-1',
	clientName,
	status: 'active',
	createdAt: '2026-08-01T00:00:00Z',
	dueDate: '2027-03-01'
};

export const fixture: RouteFixture = {
	name: 'The Staff-side Engagement detail hub',
	component: Page,
	params: { practiceId: 'practice-1', engagementId: 'engagement-1' },
	url: 'https://example.test/practices/practice-1/engagements/engagement-1',
	// The Engagement arrives as a prop now, from +page.ts's load (#695),
	// rather than through the cascade below. The route's `respond` still
	// answers it, harmlessly, for the same reason the URL builders stayed
	// exported: nothing else has to change if it ever moves back.
	props: { data: detail },
	respond: (path) => {
		if (/\/engagements\/engagement-1$/.test(path)) return jsonResponse(detail);
		if (path.includes('/visits')) {
			return jsonResponse({
				items: [{ visitId: 'visit-1', staffId: 'staff-1', staffName: 'Anne-Marie Ochieng-Whitfield', createdAt: '2026-08-05T00:00:00Z' }],
				hasMore: false
			});
		}
		if (path.includes('/messages')) return jsonResponse({ items: [], hasMore: false });
		if (path.includes('/plans/')) return jsonResponse('not found', 404);
		if (path.endsWith('/contract/invoices')) return jsonResponse({ items: [] });
		if (path.endsWith('/contract')) return jsonResponse('not found', 404);
		if (path.endsWith('/offers')) return jsonResponse({ items: [] });
		if (path.endsWith('/staff')) return jsonResponse({ members: [], invitations: { items: [] } });
		throw new Error(`engagements/[engagementId] fixture: unmatched fetch path ${path}`);
	},
	readyText: clientName
};
