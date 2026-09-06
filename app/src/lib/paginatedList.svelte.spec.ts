import { describe, expect, it, vi } from 'vitest';

import { PaginatedList, type CursorPage } from './paginatedList.svelte.js';

function page(items: string[], nextCursor?: string): CursorPage<string> {
	return { items, nextCursor, hasMore: nextCursor !== undefined };
}

function listOf(
	first: CursorPage<string>,
	loadPage: (cursor: string) => Promise<CursorPage<string>>
): PaginatedList<string> {
	return new PaginatedList({ first, loadPage, failureMessage: 'Failed to load more' });
}

describe('PaginatedList', () => {
	it('adopts the first page the route already loaded', () => {
		const list = listOf(page(['a', 'b'], 'cursor-1'), vi.fn());

		expect(list.items).toEqual(['a', 'b']);
		expect(list.hasMore).toBe(true);
		expect(list.isLoadingMore).toBe(false);
		expect(list.loadMoreError).toBe('');
	});

	it('reports no more pages when the first page is the last', () => {
		const list = listOf(page(['a']), vi.fn());

		expect(list.hasMore).toBe(false);
	});

	it('appends the next page and advances the cursor', async () => {
		const loadPage = vi.fn().mockResolvedValue(page(['c'], 'cursor-2'));
		const list = listOf(page(['a', 'b'], 'cursor-1'), loadPage);

		await list.loadMore();

		expect(loadPage).toHaveBeenCalledWith('cursor-1');
		expect(list.items).toEqual(['a', 'b', 'c']);
		expect(list.hasMore).toBe(true);

		await list.loadMore();

		expect(loadPage).toHaveBeenLastCalledWith('cursor-2');
		expect(list.items).toEqual(['a', 'b', 'c', 'c']);
	});

	it('stops offering more once a page comes back without a cursor', async () => {
		const list = listOf(page(['a'], 'cursor-1'), vi.fn().mockResolvedValue(page(['b'])));

		await list.loadMore();

		expect(list.hasMore).toBe(false);
	});

	it('does nothing when no page remains', async () => {
		const loadPage = vi.fn();
		const list = listOf(page(['a']), loadPage);

		await list.loadMore();

		expect(loadPage).not.toHaveBeenCalled();
	});

	// A double click on "Load more" is one request, not two that both
	// append the same page.
	it('ignores a second call while one is in flight', async () => {
		const held = Promise.withResolvers<CursorPage<string>>();
		const loadPage = vi.fn().mockReturnValue(held.promise);
		const list = listOf(page(['a'], 'cursor-1'), loadPage);

		const first = list.loadMore();
		await list.loadMore();

		expect(loadPage).toHaveBeenCalledTimes(1);
		expect(list.isLoadingMore).toBe(true);

		held.resolve(page(['b']));
		await first;

		expect(list.items).toEqual(['a', 'b']);
		expect(list.isLoadingMore).toBe(false);
	});

	it("surfaces a thrown Error's own message", async () => {
		const list = listOf(
			page(['a'], 'cursor-1'),
			vi.fn().mockRejectedValue(new Error('The practice was not found'))
		);

		await list.loadMore();

		expect(list.loadMoreError).toBe('The practice was not found');
		expect(list.items).toEqual(['a']);
		expect(list.isLoadingMore).toBe(false);
	});

	// So a rejected non-Error never renders an empty error box.
	it('falls back to the list its own failure message when something else is thrown', async () => {
		const list = listOf(page(['a'], 'cursor-1'), vi.fn().mockRejectedValue('offline'));

		await list.loadMore();

		expect(list.loadMoreError).toBe('Failed to load more');
	});

	it('clears a previous error when the next load starts', async () => {
		const loadPage = vi
			.fn()
			.mockRejectedValueOnce(new Error('temporary'))
			.mockResolvedValueOnce(page(['b']));
		const list = listOf(page(['a'], 'cursor-1'), loadPage);

		await list.loadMore();
		expect(list.loadMoreError).toBe('temporary');

		await list.loadMore();
		expect(list.loadMoreError).toBe('');
		expect(list.items).toEqual(['a', 'b']);
	});

	describe('abandon', () => {
		// The Clients list abandons at the click, before its navigation, so
		// the rows have to stay up until the new filter's page arrives.
		it('leaves the current rows on screen', () => {
			const list = listOf(page(['a', 'b'], 'cursor-1'), vi.fn());

			list.abandon();

			expect(list.items).toEqual(['a', 'b']);
			expect(list.hasMore).toBe(true);
		});

		it('drops a page that resolves after it', async () => {
			const held = Promise.withResolvers<CursorPage<string>>();
			const list = listOf(
				page(['a'], 'cursor-1'),
				vi.fn().mockReturnValue(
					held.promise
				)
			);

			const inFlight = list.loadMore();
			list.abandon();

			held.resolve(page(['stale']));
			await inFlight;

			expect(list.items).toEqual(['a']);
			expect(list.isLoadingMore).toBe(false);
		});

		// So the abandoned filter's failure never sits on the new one.
		it('drops a failure that arrives after it', async () => {
			const held = Promise.withResolvers<CursorPage<string>>();
			const list = listOf(
				page(['a'], 'cursor-1'),
				vi.fn().mockReturnValue(
					held.promise
				)
			);

			const inFlight = list.loadMore();
			list.abandon();

			held.reject(new Error('stale failure'));
			await inFlight;

			expect(list.loadMoreError).toBe('');
		});

		// Abandoning clears the in-flight flag, so the list is immediately
		// able to page again rather than being stuck behind a request whose
		// answer it is no longer going to use.
		it('lets the list load again straight away', async () => {
			const held = Promise.withResolvers<CursorPage<string>>();
			const loadPage = vi
				.fn()
				.mockReturnValueOnce(held.promise)
				.mockResolvedValueOnce(page(['b']));
			const list = listOf(page(['a'], 'cursor-1'), loadPage);

			const inFlight = list.loadMore();
			list.abandon();
			await list.loadMore();

			expect(list.items).toEqual(['a', 'b']);

			held.resolve(page(['stale']));
			await inFlight;

			expect(list.items).toEqual(['a', 'b']);
		});
	});

	describe('reset', () => {
		it('replaces the list with a new first page', () => {
			const list = listOf(page(['a', 'b'], 'cursor-1'), vi.fn());

			list.reset(page(['x']));

			expect(list.items).toEqual(['x']);
			expect(list.hasMore).toBe(false);
		});

		it('clears a standing error', async () => {
			const list = listOf(page(['a'], 'cursor-1'), vi.fn().mockRejectedValue(new Error('nope')));
			await list.loadMore();

			list.reset(page(['x']));

			expect(list.loadMoreError).toBe('');
		});

		// The bug only the Clients list guarded against: switching the
		// filter while a page is in flight must not merge the old filter's
		// rows into the new filter's list.
		it('drops a page that resolves after it', async () => {
			const held = Promise.withResolvers<CursorPage<string>>();
			const list = listOf(
				page(['a'], 'cursor-1'),
				vi.fn().mockReturnValue(
					held.promise
				)
			);

			const inFlight = list.loadMore();
			list.reset(page(['x'], 'cursor-9'));

			held.resolve(page(['stale']));
			await inFlight;

			expect(list.items).toEqual(['x']);
			expect(list.isLoadingMore).toBe(false);
		});

		// The same guard on the failure path: a rejection belonging to the
		// abandoned filter must not put its error on the new list.
		it('drops a failure that arrives after it', async () => {
			const held = Promise.withResolvers<CursorPage<string>>();
			const list = listOf(
				page(['a'], 'cursor-1'),
				vi.fn().mockReturnValue(
					held.promise
				)
			);

			const inFlight = list.loadMore();
			list.reset(page(['x'], 'cursor-9'));

			held.reject(new Error('stale failure'));
			await inFlight;

			expect(list.loadMoreError).toBe('');
			expect(list.items).toEqual(['x']);
		});

		it('lets the new list page on from its own cursor', async () => {
			const loadPage = vi.fn().mockResolvedValue(page(['y']));
			const list = listOf(page(['a'], 'cursor-1'), loadPage);

			list.reset(page(['x'], 'cursor-9'));
			await list.loadMore();

			expect(loadPage).toHaveBeenCalledWith('cursor-9');
			expect(list.items).toEqual(['x', 'y']);
		});
	});

	// #709: a per-viewer read gate can filter a whole page's rows out,
	// so an endpoint may legitimately answer zero items with `hasMore:
	// true`. Neither entry point -- the first page a route obtains, nor a
	// subsequent page from `loadMore` -- may ever leave that shape sitting
	// in `items`/`hasMore` for `DataTable` to render.
	describe('converging past a zero-item page with more to come (#709)', () => {
		describe('the first page', () => {
			it('pages past an empty response and lands on the first non-empty one', async () => {
				const loadPage = vi
					.fn()
					.mockResolvedValueOnce(page([], 'cursor-2'))
					.mockResolvedValueOnce(page(['a'], 'cursor-3'));
				const list = listOf(page([], 'cursor-1'), loadPage);

				// Never exposed in between: the empty first page never sits
				// alongside `hasMore: true`.
				expect(list.items).toEqual([]);
				expect(list.hasMore).toBe(false);

				await vi.waitFor(() => expect(list.items).toEqual(['a']));

				expect(list.hasMore).toBe(true);
				expect(loadPage).toHaveBeenNthCalledWith(1, 'cursor-1');
				expect(loadPage).toHaveBeenNthCalledWith(2, 'cursor-2');
			});

			it('lands on the genuinely-empty state once every page comes back empty', async () => {
				const loadPage = vi
					.fn()
					.mockResolvedValueOnce(page([], 'cursor-2'))
					.mockResolvedValueOnce(page([]));
				const list = listOf(page([], 'cursor-1'), loadPage);

				await vi.waitFor(() => expect(loadPage).toHaveBeenCalledTimes(2));

				expect(list.items).toEqual([]);
				expect(list.hasMore).toBe(false);
			});

			it('converges the same way when a route hands reset a fresh first page directly', async () => {
				const loadPage = vi
					.fn()
					.mockResolvedValueOnce(page([], 'cursor-2'))
					.mockResolvedValueOnce(page(['x'], 'cursor-3'));
				const list = listOf(page(['seed']), loadPage);

				list.reset(page([], 'cursor-1'));

				await vi.waitFor(() => expect(list.items).toEqual(['x']));
				expect(list.hasMore).toBe(true);
			});

			it("surfaces a thrown Error's own message on the failure text, not just loadMore's", async () => {
				const loadPage = vi.fn().mockRejectedValue(new Error('The practice was not found'));
				const list = listOf(page([], 'cursor-1'), loadPage);

				await vi.waitFor(() => expect(list.loadMoreError).toBe('The practice was not found'));
			});

			it('drops a failure that arrives after the list moved on', async () => {
				const held = Promise.withResolvers<CursorPage<string>>();
				const loadPage = vi.fn().mockReturnValue(held.promise);
				const list = listOf(page([], 'cursor-1'), loadPage);

				list.abandon();
				held.reject(new Error('stale failure'));
				await vi.waitFor(() => expect(loadPage).toHaveBeenCalledTimes(1));

				expect(list.loadMoreError).toBe('');
			});

			// So a rejected non-Error never renders an empty error box, the
			// same guarantee loadMore's own catch makes.
			it("falls back to the list's own failure message when something else is thrown", async () => {
				const loadPage = vi.fn().mockRejectedValue('offline');
				const list = listOf(page([], 'cursor-1'), loadPage);

				await vi.waitFor(() => expect(list.loadMoreError).toBe('Failed to load more'));
			});

			it('drops a still-empty page that resolves after the list moved on', async () => {
				const held = Promise.withResolvers<CursorPage<string>>();
				const loadPage = vi.fn().mockReturnValue(held.promise);
				const list = listOf(page([], 'cursor-1'), loadPage);

				list.abandon();
				held.resolve(page([], 'cursor-2'));
				await vi.waitFor(() => expect(loadPage).toHaveBeenCalledTimes(1));

				expect(list.items).toEqual([]);
				expect(list.hasMore).toBe(false);
			});
		});

		describe('loadMore', () => {
			it('pages past an empty response and appends the first non-empty one', async () => {
				const loadPage = vi
					.fn()
					.mockResolvedValueOnce(page([], 'cursor-2'))
					.mockResolvedValueOnce(page(['b'], 'cursor-3'));
				const list = listOf(page(['a'], 'cursor-1'), loadPage);

				await list.loadMore();

				expect(list.items).toEqual(['a', 'b']);
				expect(list.hasMore).toBe(true);
				expect(loadPage).toHaveBeenNthCalledWith(1, 'cursor-1');
				expect(loadPage).toHaveBeenNthCalledWith(2, 'cursor-2');
			});

			it('exhausts a run of empty pages and stops offering more, without dropping existing rows', async () => {
				const loadPage = vi
					.fn()
					.mockResolvedValueOnce(page([], 'cursor-2'))
					.mockResolvedValueOnce(page([]));
				const list = listOf(page(['a'], 'cursor-1'), loadPage);

				await list.loadMore();

				expect(list.items).toEqual(['a']);
				expect(list.hasMore).toBe(false);
				expect(loadPage).toHaveBeenCalledTimes(2);
			});
		});
	});
});
