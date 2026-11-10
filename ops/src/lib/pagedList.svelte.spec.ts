import { describe, expect, it, vi } from 'vitest';
import type { Outcome } from './act.ts';
import { PagedList, type CursorPage } from './pagedList.svelte.ts';
import { SERVICE_PROBLEM } from './session.ts';

type Answer = Outcome<CursorPage<string>>;

function reading(...answers: (Answer | Error)[]) {
	const read = vi.fn<(cursor: string | undefined) => Promise<Answer>>();
	for (const answer of answers) {
		if (answer instanceof Error) read.mockRejectedValueOnce(answer);
		else read.mockResolvedValueOnce(answer);
	}
	return read;
}

const page = (items: string[], nextCursor?: string): Answer => ({
	kind: 'ok',
	value: { items, hasMore: nextCursor !== undefined, nextCursor }
});

describe('PagedList', () => {
	it('is loading until the first page answers, then holds its rows and the next cursor', async () => {
		const list = new PagedList(reading(page(['a'], 'c-2')), vi.fn());

		const loading = list.load();
		expect(list.state).toEqual({ status: 'loading' });
		await loading;

		expect(list.state).toEqual({ status: 'loaded', items: ['a'], nextCursor: 'c-2' });
		expect(list.errors).toEqual([]);
	});

	it('ignores a cursor on a last page', async () => {
		const list = new PagedList(reading({ kind: 'ok', value: { items: ['a'], hasMore: false, nextCursor: 'stale' } }), vi.fn());

		await list.load();

		expect(list.state).toEqual({ status: 'loaded', items: ['a'], nextCursor: undefined });
	});

	it('appends the next page, and the one after it', async () => {
		const read = reading(page(['a'], 'c-2'), page(['b'], 'c-3'), page(['c']));
		const list = new PagedList(read, vi.fn());
		await list.load();

		const more = list.more();
		expect(list.isLoadingMore).toBe(true);
		await more;
		expect(read).toHaveBeenLastCalledWith('c-2');
		expect(list.state).toEqual({ status: 'loaded', items: ['a', 'b'], nextCursor: 'c-3' });

		await list.more();
		expect(read).toHaveBeenLastCalledWith('c-3');
		expect(list.state).toEqual({ status: 'loaded', items: ['a', 'b', 'c'], nextCursor: undefined });
		expect(list.isLoadingMore).toBe(false);
	});

	it('reads no more before it has loaded, after the last page, or while a page is being read', async () => {
		const read = reading(page(['a'], 'c-2'), page(['b']));
		const list = new PagedList(read, vi.fn());

		await list.more();
		await list.load();
		const more = list.more();
		await list.more();
		await more;
		await list.more();

		expect(read).toHaveBeenCalledTimes(2);
	});

	it('fails with a refused first page', async () => {
		const list = new PagedList(reading({ kind: 'refused', errors: [{ message: 'Start from the first page.' }] }), vi.fn());

		await list.load();

		expect(list.state).toEqual({ status: 'failed', errors: [{ message: 'Start from the first page.' }] });
		expect(list.errors).toEqual([{ message: 'Start from the first page.' }]);
	});

	// A dropped connection was silent before: no rows, no reason.
	it('fails with a problem with the service when the first page throws', async () => {
		const list = new PagedList(reading(new TypeError('Failed to fetch')), vi.fn());

		await list.load();

		expect(list.errors).toEqual([{ message: SERVICE_PROBLEM }]);
	});

	it('keeps the rows and says why when a later page fails, and reads it again on the next press', async () => {
		const read = reading(page(['a'], 'c-2'), new TypeError('Failed to fetch'), page(['b']));
		const list = new PagedList(read, vi.fn());
		await list.load();

		await list.more();
		expect(list.state).toEqual({ status: 'loaded', items: ['a'], nextCursor: 'c-2' });
		expect(list.errors).toEqual([{ message: SERVICE_PROBLEM }]);

		await list.more();
		expect(list.state).toEqual({ status: 'loaded', items: ['a', 'b'], nextCursor: undefined });
		expect(list.errors).toEqual([]);
	});

	it('hands back once the session has ended, on the first page or a later one', async () => {
		const onsignedout = vi.fn();
		const list = new PagedList(reading({ kind: 'signed-out' }), onsignedout);
		await list.load();
		expect(list.state).toEqual({ status: 'signed-out' });

		const later = new PagedList(reading(page(['a'], 'c-2'), { kind: 'signed-out' }), onsignedout);
		await later.load();
		await later.more();
		expect(later.state).toEqual({ status: 'signed-out' });
		expect(onsignedout).toHaveBeenCalledTimes(2);
	});
});
