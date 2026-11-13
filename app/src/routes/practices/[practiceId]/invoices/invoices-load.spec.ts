import { describe, expect, it, vi, afterEach } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { loadedValue, settledRead } from '#lib/testStreamedRead.js';
import { load } from './+page.js';

function setup(status: number, body: unknown) {
	const fetchMock = vi.fn(async () => jsonResponse(body, status));
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

afterEach(() => {
	vi.unstubAllGlobals();
});

const emptyBook = {
	items: [],
	hasMore: false,
	outstandingCents: 0,
	outstandingCount: 0,
	paidCents: 0,
	overdueCents: 0,
	overdueCount: 0
};

describe('invoices/+page.ts load', () => {
	it('fetches the Practice-wide invoices path and hands over the page', async () => {
		const { fetchMock } = setup(200, emptyBook);

		const data = await load({ depends: () => {}, params: { practiceId: 'practice-1' }, url: new URL('https://example.test/practices/practice-1/invoices') } as Parameters<typeof load>[0]);

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/practices/practice-1/invoices',
			expect.objectContaining({ credentials: 'include' })
		);
		await expect(loadedValue(data)).resolves.toEqual({ ...emptyBook, view: 'all' });
	});

	// #768: the narrowing is in the URL, so the load reads it there and
	// asks the BFF for the same slice the link named.
	it('carries an overdue narrowing from the URL through to the BFF', async () => {
		const { fetchMock } = setup(200, emptyBook);

		const data = await load({
			depends: () => {},
			params: { practiceId: 'practice-1' },
			url: new URL('https://example.test/practices/practice-1/invoices?show=overdue')
		} as Parameters<typeof load>[0]);

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/practices/practice-1/invoices?overdue=true',
			expect.objectContaining({ credentials: 'include' })
		);
		await expect(loadedValue(data)).resolves.toEqual({ ...emptyBook, view: 'overdue' });
	});

	// #2249: the screen has changed by the time a refusal lands, so the
	// screen follows it; the read carries the response through unread.
	it.each([
		['show=unpaid', 'unpaid'],
		['show=overdue', 'overdue']
	] as const)('reads %s as the %s view', async (query, view) => {
		const { fetchMock } = setup(200, emptyBook);

		const data = await load({
			depends: () => {},
			params: { practiceId: 'practice-1' },
			url: new URL(`https://example.test/practices/practice-1/invoices?${query}`)
		} as Parameters<typeof load>[0]);

		expect(fetchMock).toHaveBeenCalledWith(
			`/api/practices/practice-1/invoices?${view}=true`,
			expect.objectContaining({ credentials: 'include' })
		);
		await expect(loadedValue(data)).resolves.toEqual({ ...emptyBook, view });
	});

	it('hands a refusal to the screen rather than throwing mid-load', async () => {
		setup(403, { code: 'FORBIDDEN', message: 'not permitted to read this' });

		const data = await load({ depends: () => {}, params: { practiceId: 'practice-1' }, url: new URL('https://example.test/practices/practice-1/invoices') } as Parameters<typeof load>[0]);

		await expect(settledRead(data)).resolves.toMatchObject({ isLoaded: false, response: { status: 403 } });
	});
});
