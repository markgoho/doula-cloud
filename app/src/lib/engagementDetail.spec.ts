import { describe, expect, it, vi } from 'vitest';

import { jsonResponse } from './testResponse.js';
import {
	loadAttachmentPreviews,
	loadEngagement,
	loadMessagesPage,
	loadOffersSection,
	loadVisitsPage,
	messagesURL,
	portalInviteURL,
	visitsURL,
	type EngagementReference,
	type Visit
} from './engagementDetail.js';

vi.mock('./api.js', () => ({
	apiFetchWithSession: vi.fn(),
	apiErrorMessage: (response: Response) => response.text()
}));

const reference: EngagementReference = { practiceId: 'practice-1', engagementId: 'engagement-1' };
const base = '/api/practices/practice-1/engagements/engagement-1';

describe('loadEngagement', () => {
	it('reads the Engagement', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ clientName: 'Tasha Bell' }));

		const detail = await loadEngagement(fetcher, reference);

		expect(fetcher).toHaveBeenCalledWith(base);
		expect(detail.clientName).toBe('Tasha Bell');
	});

	it('throws a refusal, so load turns it into the route error', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not permitted to read this', 403));

		await expect(loadEngagement(fetcher, reference)).rejects.toThrow('not permitted to read this');
	});
});

describe('loadVisitsPage', () => {
	it('reads the first page without a cursor parameter', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadVisitsPage(fetcher, reference, '');

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits`);
	});

	it('encodes a cursor onto the next page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadVisitsPage(fetcher, reference, 'a b/c');

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits?cursor=a%20b%2Fc`);
	});

	it('throws a refusal, so PaginatedList can catch it', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(loadVisitsPage(fetcher, reference, '')).rejects.toThrow('nope');
	});

	it('keeps the endpoint order, newest first', async () => {
		const visits: Visit[] = [
			{ visitId: 'v2', staffId: 's1', staffName: 'Maya', createdAt: '2027-02-01' },
			{ visitId: 'v1', staffId: 's1', staffName: 'Maya', createdAt: '2027-01-01' }
		];
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: visits, hasMore: false }));

		const page = await loadVisitsPage(fetcher, reference, '');

		expect(page.items.map((v) => v.visitId)).toEqual(['v2', 'v1']);
	});
});

describe('loadMessagesPage', () => {
	// The one list on this page whose order is flipped: the BFF answers
	// newest-first like every other cursor list, but a thread reads
	// oldest-at-the-top. This used to be a bare .toReversed() inline in the
	// route, where nothing said why.
	it('reverses the page, because a thread reads oldest first', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse({
				items: [{ messageId: 'newest' }, { messageId: 'oldest' }],
				nextCursor: 'c1',
				hasMore: true
			})
		);

		const page = await loadMessagesPage<{ messageId: string }>(fetcher, reference, '');

		expect(page.items.map((m) => m.messageId)).toEqual(['oldest', 'newest']);
		expect(page.nextCursor).toBe('c1');
		expect(page.hasMore).toBe(true);
	});

	// Older messages page backwards from a cursor, which is the prepend
	// this list does instead of PaginatedList's append.
	it('encodes a cursor onto the older page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadMessagesPage(fetcher, reference, 'a b/c');

		expect(fetcher).toHaveBeenCalledWith(`${base}/messages?cursor=a%20b%2Fc`);
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(loadMessagesPage(fetcher, reference, '')).rejects.toThrow('nope');
	});
});

function blobResponse(): Response {
	return new Response(new Blob(['x'], { type: 'image/png' }), { status: 200 });
}

describe('loadAttachmentPreviews', () => {
	it('fetches only the image attachments not already loaded', async () => {
		const fetcher = vi.fn().mockResolvedValue(blobResponse());

		const loaded = await loadAttachmentPreviews(
			fetcher,
			reference,
			[
				{ messageId: 'has-image', attachmentContentType: 'image/png' },
				{ messageId: 'has-pdf', attachmentContentType: 'application/pdf' },
				{ messageId: 'no-attachment' },
				{ messageId: 'already-here', attachmentContentType: 'image/jpeg' }
			],
			{ 'already-here': 'blob:existing' }
		);

		expect(fetcher).toHaveBeenCalledTimes(1);
		expect(fetcher).toHaveBeenCalledWith(`${messagesURL(reference)}/has-image/attachment`);
		expect(Object.keys(loaded)).toEqual(['has-image']);
	});

	// A thumbnail that will not load is a missing image, not a broken
	// thread, so the refusal is skipped rather than thrown.
	it('skips an attachment the caller may not read', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		const loaded = await loadAttachmentPreviews(
			fetcher,
			reference,
			[{ messageId: 'refused', attachmentContentType: 'image/png' }],
			{}
		);

		expect(loaded).toEqual({});
	});

	it('returns the new URLs rather than mutating what it was given', async () => {
		const fetcher = vi.fn().mockResolvedValue(blobResponse());
		const existing = { 'already-here': 'blob:existing' };

		const loaded = await loadAttachmentPreviews(
			fetcher,
			reference,
			[{ messageId: 'fresh', attachmentContentType: 'image/png' }],
			existing
		);

		expect(existing).toEqual({ 'already-here': 'blob:existing' });
		expect(Object.keys(loaded)).toEqual(['fresh']);
	});
});

describe('loadOffersSection', () => {
	const roster = {
		members: [
			{ staffId: 's1', name: 'Maya', employmentType: 'contractor', roles: ['doula'] },
			{ staffId: 's2', name: 'Ada', employmentType: 'employee', roles: ['owner'] },
			{ staffId: 's3', name: 'Bess', employmentType: 'employee', roles: ['doula', 'admin'] }
		]
	};

	it('returns the offers and only the roster members holding the Doula role', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(roster));
		const loadOffers = vi.fn().mockResolvedValue([{ offerId: 'o1' }]);

		const section = await loadOffersSection(fetcher, reference, loadOffers);

		expect(section?.offers).toEqual([{ offerId: 'o1' }]);
		expect(section?.doulas).toEqual([
			{ staffId: 's1', name: 'Maya', employmentType: 'contractor' },
			{ staffId: 's3', name: 'Bess', employmentType: 'employee' }
		]);
	});

	// Both reads are Owner/Admin. Either refusing is what tells the page
	// the caller is a Doula, and a Doula may not read who else was offered
	// her work -- so the section is left out rather than shown broken.
	it('answers undefined when the Offers read refuses', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(roster));
		const loadOffers = vi.fn().mockRejectedValue(new Error('not permitted to read this'));

		expect(await loadOffersSection(fetcher, reference, loadOffers)).toBeUndefined();
	});

	it('answers undefined when the roster read refuses', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));
		const loadOffers = vi.fn().mockResolvedValue([]);

		expect(await loadOffersSection(fetcher, reference, loadOffers)).toBeUndefined();
	});
});

describe('URL builders', () => {
	it('build every path off the one Engagement reference', () => {
		expect(visitsURL(reference)).toBe(`${base}/visits`);
		expect(messagesURL(reference)).toBe(`${base}/messages`);
		expect(portalInviteURL(reference)).toBe(`${base}/portal-invite`);
	});
});
