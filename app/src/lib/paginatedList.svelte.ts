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

	/**
	 * Adopts a page, but never exposes the shape `DataTable` cannot render
	 * sanely: zero items with `hasMore: true` (#709). A per-viewer read
	 * gate can filter a whole batch of rows out before any of them reach
	 * the caller, so an endpoint is allowed to answer that way; this class
	 * is what hides it, by holding `hasMore` at `false` until convergence
	 * finds a page actually worth offering.
	 */
	#adopt(page: CursorPage<Item>): void {
		this.#cursor = page.nextCursor ?? '';
		if (page.items.length > 0 || !page.hasMore) {
			this.items = page.items;
			this.hasMore = page.hasMore;
			return;
		}
		this.items = [];
		this.hasMore = false;
		void this.#converge(this.#generation);
	}

	/**
	 * Silently pages past a run of zero-item/`hasMore: true` responses,
	 * until one yields an item or the endpoint genuinely runs out, then
	 * publishes that page. `loadMore` below has its own copy of the same
	 * loop rather than calling this: it must keep the rows already on
	 * screen while it runs, instead of blanking them to `[]` first.
	 */
	async #converge(generation: number): Promise<void> {
		try {
			let page: CursorPage<Item>;
			do {
				page = await this.#loadPage(this.#cursor);
				if (generation !== this.#generation) return;
				this.#cursor = page.nextCursor ?? '';
			} while (page.items.length === 0 && page.hasMore);
			this.items = page.items;
			this.hasMore = page.hasMore;
		} catch (error_) {
			if (generation !== this.#generation) return;
			this.loadMoreError = error_ instanceof Error ? error_.message : this.#failureMessage;
		}
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
	 *
	 * Loops silently past a zero-item/`hasMore: true` response (#709)
	 * rather than appending nothing and leaving `hasMore` true -- that
	 * combination is exactly the empty-message-plus-Load-more
	 * contradiction `DataTable` cannot render sanely.
	 */
	async loadMore(): Promise<void> {
		if (!this.hasMore || this.isLoadingMore) return;

		const generation = this.#generation;
		this.loadMoreError = '';
		this.isLoadingMore = true;
		try {
			let next: CursorPage<Item>;
			do {
				next = await this.#loadPage(this.#cursor);
				if (generation !== this.#generation) return;
				this.#cursor = next.nextCursor ?? '';
				this.hasMore = next.hasMore;
			} while (next.items.length === 0 && this.hasMore);
			this.items = [...this.items, ...next.items];
		} catch (error_) {
			if (generation !== this.#generation) return;
			this.loadMoreError = error_ instanceof Error ? error_.message : this.#failureMessage;
		} finally {
			if (generation === this.#generation) this.isLoadingMore = false;
		}
	}
}
