import { describe, expect, it, afterEach, vi } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';

function setup(status: number, body: unknown) {
	const fetchMock = vi.fn(async () => jsonResponse(body, status));
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('clients/search/+page.ts load (#501)', () => {
	it('reads the practice session and shows the search screen to an employee Doula', async () => {
		const { load } = await import('./+page.js');
		const { fetchMock } = setup(200, { roles: ['doula'], isContractor: false });

		const result = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/practices/practice-1/session',
			expect.objectContaining({ credentials: 'include' })
		);
		expect(result).toEqual({ isContractor: false });
	});

	it('shows the explainer to a contractor Doula holding no owner or admin role', async () => {
		const { load } = await import('./+page.js');
		setup(200, { roles: ['doula'], isContractor: true });

		const result = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		expect(result).toEqual({ isContractor: true });
	});

	it("keeps the search screen for a solo Practice's owner-contractor (ADR-0017)", async () => {
		const { load } = await import('./+page.js');
		setup(200, { roles: ['owner', 'doula'], isContractor: true });

		const result = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		expect(result).toEqual({ isContractor: false });
	});

	it('keeps the search screen for an admin who also carries a contractor membership', async () => {
		const { load } = await import('./+page.js');
		setup(200, { roles: ['admin', 'doula'], isContractor: true });

		const result = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		expect(result).toEqual({ isContractor: false });
	});

	it('falls back to the search screen when the session read fails -- UX-only, SearchHandler is the real boundary', async () => {
		const { load } = await import('./+page.js');
		setup(500, 'boom');

		const result = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		expect(result).toEqual({ isContractor: false });
	});

	it('falls back to the search screen on a network failure', async () => {
		const { load } = await import('./+page.js');
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => {
				throw new Error('network down');
			})
		);

		const result = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		expect(result).toEqual({ isContractor: false });
	});
});
