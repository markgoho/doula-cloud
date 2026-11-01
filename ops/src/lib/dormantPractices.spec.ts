/* eslint-disable unicorn/no-null -- doula-ops answers null for a field with nothing in it */
import { describe, expect, it, vi } from 'vitest';
import { dormantFacts, readDormantPractices, type DormantPractice } from './dormantPractices.ts';

function answering(status: number, body?: unknown) {
	return vi.fn<typeof fetch>(
		async () => new Response(body === undefined ? undefined : JSON.stringify(body), { status })
	);
}

const quiet: DormantPractice = {
	id: 'p-1',
	name: 'Quiet Hills Doulas',
	signedUpAt: '2023-05-01T14:00:00Z',
	balance: 4,
	lastContactAt: '2024-10-01T16:00:00Z'
};

describe('dormantFacts', () => {
	it('states when the Practice signed up, the Credits it holds and its last contact', () => {
		expect(dormantFacts(quiet)).toEqual([
			{ label: 'Signed up', value: 'May 1, 2023' },
			{ label: 'Credits held', value: '4 Credits' },
			{ label: 'Last contact', value: 'October 1, 2024' }
		]);
	});

	it('says nobody has ever been seen at a Practice with no contact on record', () => {
		expect(dormantFacts({ ...quiet, balance: 1, lastContactAt: null })).toEqual([
			{ label: 'Signed up', value: 'May 1, 2023' },
			{ label: 'Credits held', value: '1 Credit' },
			{ label: 'Last contact', value: 'Never' }
		]);
	});
});

describe('readDormantPractices', () => {
	it('reads the first page', async () => {
		const page = { items: [quiet], hasMore: true, nextCursor: 'c-2' };
		const fetcher = answering(200, page);

		expect(await readDormantPractices(undefined, fetcher)).toEqual({ kind: 'ok', value: page });
		expect(fetcher).toHaveBeenCalledWith('/api/practices/dormant', { credentials: 'include' });
	});

	it('reads a later page', async () => {
		const fetcher = answering(200, { items: [], hasMore: false });

		await readDormantPractices('c 2', fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/dormant?cursor=c%202', { credentials: 'include' });
	});

	it('names an ended session', async () => {
		expect(await readDormantPractices(undefined, answering(401, { message: 'ended' }))).toEqual({ kind: 'signed-out' });
	});
});
