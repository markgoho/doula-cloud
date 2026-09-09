import { describe, expect, it, vi } from 'vitest';

import { jsonResponse } from './testResponse.js';
import {
	inReadingOrder,
	loadPortalVisitsPage,
	noVisitsMessage,
	portalVisitColumns,
	type PortalVisit
} from './portalVisits.js';

const visit = (overrides: Partial<PortalVisit> = {}): PortalVisit => ({
	visitId: 'visit-1',
	scheduledAt: '2027-02-18T19:00:00Z',
	doulaName: 'Priya Raman',
	hasHappened: false,
	...overrides
});

describe('loadPortalVisitsPage', () => {
	it('reads the portal path', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadPortalVisitsPage(fetcher, 'engagement-1', '');

		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/engagement-1/visits');
	});

	it('encodes a cursor onto the next page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadPortalVisitsPage(fetcher, 'engagement-1', 'a b/c');

		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/engagement-1/visits?cursor=a%20b%2Fc');
	});

	it('throws a refusal, so PaginatedList can catch it', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(loadPortalVisitsPage(fetcher, 'engagement-1', '')).rejects.toThrow('nope');
	});
});

describe('portalVisitColumns', () => {
	// The design source's own two columns, in its own order: when, then
	// who. Nothing else is offered to a Client at all.
	it('offers when and who, and nothing else', () => {
		expect(portalVisitColumns().map((column) => column.label)).toEqual(['When', 'Who']);
	});

	it('carries the raw instant underneath the display string', () => {
		const when = portalVisitColumns()[0]!;

		expect(when.datetimeAccessor!(visit())).toBe('2027-02-18T19:00:00Z');
		expect(when.accessor(visit())).not.toBe('2027-02-18T19:00:00Z');
	});

	// The Activity ledger replaces a Staff actor's name with "Your
	// practice" (CONTEXT.md: she never reads who inside the Practice did
	// what). That rule is about the Practice's roster acts; who is coming
	// to her home is a fact about her care, and this column says so.
	it('names the Doula who is coming, not the Practice', () => {
		const who = portalVisitColumns()[1]!;

		expect(who.accessor(visit({ doulaName: 'Maya Okonkwo' }))).toBe('Maya Okonkwo');
	});
});

// The wire's order is furthest-future first, and the page's is the next
// Visit first -- the question both journeys ask.
describe('inReadingOrder', () => {
	const later = visit({ visitId: 'later', scheduledAt: '2027-04-20T15:00:00Z' });
	const soon = visit({ visitId: 'soon', scheduledAt: '2027-02-18T19:00:00Z' });
	const recent = visit({ visitId: 'recent', scheduledAt: '2026-08-18T14:30:00Z', hasHappened: true });
	const older = visit({ visitId: 'older', scheduledAt: '2026-08-04T14:30:00Z', hasHappened: true });

	it('reads scheduled soonest-first, then what has happened newest-first', () => {
		expect(inReadingOrder([later, soon, recent, older]).map((v) => v.visitId)).toEqual([
			'soon',
			'later',
			'recent',
			'older'
		]);
	});

	it('leaves the caller its own array', () => {
		const page = [later, soon];

		inReadingOrder(page);

		expect(page.map((v) => v.visitId)).toEqual(['later', 'soon']);
	});

	it.each([
		['nothing at all', []],
		['only scheduled Visits', [later, soon]],
		['only Visits that have happened', [recent, older]]
	])('holds for a list of %s', (_name, items) => {
		expect(inReadingOrder(items)).toHaveLength(items.length);
	});
});

// CB-G5: an empty list must not promise a visit that is not scheduled --
// true for a postpartum-only Engagement and for one with nothing booked.
describe('noVisitsMessage', () => {
	it('reports the state of the list and promises nothing', () => {
		expect(noVisitsMessage).toBe('Nothing is booked yet.');
		expect(noVisitsMessage).not.toMatch(/will|soon|shortly|coming/i);
	});
});
