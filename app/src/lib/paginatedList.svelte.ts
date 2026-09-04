/**
 * A cursor-paginated list that owns its own cursor, in-flight guard and
 * error text.
 *
 * `api.ts` owns "call the BFF" and owns it well -- base URL, cookie
 * credentials, the staff-versus-portal 401 branch, the text-or-JSON error
 * shape. What it does not own is *state*, so every list that pages
 * re-derived the same four variables (a cursor, whether more remains,
 * whether a load is in flight, and an error string) plus its own
 * append-and-advance handler. Six lists did this: the Billing ledger, the
 * Invoice book, the Clients list, the Engagement Request inbox, the Staff
 * screen's invitations, and an Engagement's visits.
 *
 * They did not all do it the same way, which is the actual argument for
 * this module. Only the Clients list guarded against a stale page landing
 * after the filter changed; the others would merge whatever came back.
 * Only some tracked `isLoadingMore` at all, so the rest never disabled
 * their own "Load more" button. Consolidating means every list gets the
 * careful version rather than each getting whichever half its author
 * needed that day.
 */

/**
 * One page of a cursor-paginated endpoint -- the envelope from
 * docs/api-design.md section 4, which every list endpoint answers with.
 */
export interface CursorPage<Item> {
	items: Item[];
	nextCursor?: string;
	hasMore: boolean;
}

/**
 * Fetches the page after `cursor`. A closure rather than a URL, because
 * what varies between lists is not only the path: the Clients list carries
 * a filter, the Invoice book takes a practice id, and both already have a
 * loader in their own module that shapes the response. Those loaders throw
 * on failure, which is what `loadMore` catches.
 *
 * `cursor` is the empty string for the first page, matching how the
 * routes already spell "no cursor yet".
 */
export type PageLoader<Item> = (cursor: string) => Promise<CursorPage<Item>>;

export interface PaginatedListOptions<Item> {
	/**
	The page the route's `load` already fetched.
	*/
	first: CursorPage<Item>;
	loadPage: PageLoader<Item>;
	/**
	 * What to show when the loader throws something that is not an Error,
	 * so the list never renders an empty error box. Written per list
	 * because "Failed to load more Clients" and "Failed to load more
	 * invoices" are the two the screens already said.
	 */
	failureMessage: string;
}

export class PaginatedList<Item> {
	#cursor = '';
	#loadPage: PageLoader<Item>;
	#failureMessage: string;
	/**
	 * Bumped by `abandon` (and so by `reset`), and compared after every
	 * await. A page that resolves against a superseded generation is
	 * dropped rather than appended -- otherwise switching a filter while a
	 * load is in flight merges the old filter's rows into the new filter's
	 * list, and the screen shows a mixture that matches no query anybody
	 * ran.
	 */
	#generation = 0;

	/**
	Every item loaded so far, first page included.
	*/
	items = $state<Item[]>([]);
	/**
	Whether the endpoint says another page exists.
	*/
	hasMore = $state(false);
	/**
	Whether a `loadMore` is in flight. Drives the button's own spinner.
	*/
	isLoadingMore = $state(false);
	/**
	The last failure, or the empty string. Cleared when a load starts.
	*/
	loadMoreError = $state('');

	constructor({ first, loadPage, failureMessage }: PaginatedListOptions<Item>) {
		this.#loadPage = loadPage;
		this.#failureMessage = failureMessage;
		this.#adopt(first);
	}

	#adopt(page: CursorPage<Item>): void {
		this.items = page.items;
		this.#cursor = page.nextCursor ?? '';
		this.hasMore = page.hasMore;
	}

	/**
	 * Replaces the list with a fresh first page and abandons any load in
	 * flight -- what a filter change does. The Clients list is the caller
	 * that needs it; every other list gets the same protection by
	 * construction.
	 */
	reset(first: CursorPage<Item>): void {
		this.abandon();
		this.#adopt(first);
	}

	/**
	 * Abandons any load in flight while leaving the current rows on screen.
	 *
	 * Separate from `reset` because the two happen at different moments.
	 * The Clients list abandons the instant the reader flips its filter --
	 * before the navigation, so a page already in flight can never land --
	 * and only replaces its rows once the new filter's first page has
	 * actually arrived. Resetting at the click instead would blank the
	 * table for the length of a request.
	 */
	abandon(): void {
		this.#generation += 1;
		this.isLoadingMore = false;
		this.loadMoreError = '';
	}

	/**
	 * Appends the next page. Does nothing when no page remains or one is
	 * already in flight, so a double click is one request rather than two
	 * that both append.
	 */
	async loadMore(): Promise<void> {
		if (!this.hasMore || this.isLoadingMore) return;

		const generation = this.#generation;
		this.loadMoreError = '';
		this.isLoadingMore = true;
		try {
			const next = await this.#loadPage(this.#cursor);
			if (generation !== this.#generation) return;
			this.items = [...this.items, ...next.items];
			this.#cursor = next.nextCursor ?? '';
			this.hasMore = next.hasMore;
		} catch (error_) {
			if (generation !== this.#generation) return;
			this.loadMoreError = error_ instanceof Error ? error_.message : this.#failureMessage;
		} finally {
			if (generation === this.#generation) this.isLoadingMore = false;
		}
	}
}
