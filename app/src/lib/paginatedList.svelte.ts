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

/**
 * What a caller sees of a list nobody has asked for yet (#1149).
 *
 * The difference from the class below is `entries`, and it is the whole
 * point: a list built around a `first` page always has rows to show, so
 * `items: []` can only mean "the endpoint answered with nothing". A list
 * fetched on demand is `[]` before anything has been requested too, and
 * those two need different words on screen -- "Loading..." against
 * "Nothing recorded.".
 *
 * So this view does not offer `items` at all. The only rows it can hand
 * over are `entries`, which is `undefined` until a page has actually
 * landed, and TypeScript refuses to read a length or an index off it
 * until the caller has said what an absent page looks like. `reset` and
 * `abandon` are absent for the same reason: a deferred list's first page
 * comes from `ask`, not from a route's `load`, so there is no fresh first
 * page for a caller to hand it.
 */
export interface DeferredPaginatedList<Item> {
	/**
	 * The entries loaded so far, or `undefined` while no page has landed
	 * -- before `ask`, while the first one is in flight, and after a first
	 * one that failed.
	 */
	readonly entries: readonly Item[] | undefined;
	readonly hasMore: boolean;
	readonly isLoadingMore: boolean;
	readonly loadMoreError: string;
	ask(): Promise<void>;
	loadMore(): Promise<void>;
}

export class PaginatedList<Item> {
	/**
	 * A list that pages on demand: no first page, because nothing has
	 * asked for one. Its rows are read through `entries`, which stays
	 * `undefined` until `ask` lands a page -- the state the Staff roster's
	 * two history disclosures spent four containers each hand-rolling
	 * before this existed (#1149).
	 *
	 * `first` stays required on the ordinary constructor, so an eager
	 * caller cannot reach this state by leaving something out; a deferred
	 * list is asked for by name.
	 */
	static deferred<Item>(
		options: Omit<PaginatedListOptions<Item>, 'first'>
	): DeferredPaginatedList<Item> {
		const list = new PaginatedList<Item>({ ...options, first: { items: [], hasMore: false } });
		list.#landed = false;
		return list;
	}

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
	 * Whether a page has ever landed. True from construction for the
	 * ordinary path, because the route's `load` already fetched the first
	 * page; false for `deferred` until `ask` publishes one. It is what
	 * `entries` reads, and what stops a second `ask` -- never
	 * `loadMoreError`, which a *further* page's failure also sets, and
	 * asking again then would re-run the first-page walk from a cursor
	 * that has already moved.
	 */
	#landed = $state(true);

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
			// The one place a deferred list stops being unasked: a page
			// reached the caller, so `entries` may now say `[]` and mean it.
			this.#landed = true;
		} catch (error_) {
			if (generation !== this.#generation) return;
			this.loadMoreError = error_ instanceof Error ? error_.message : this.#failureMessage;
		}
	}

	/**
	 * The rows, or `undefined` while none has landed. See
	 * `DeferredPaginatedList`, which is the only view that exposes it.
	 */
	get entries(): readonly Item[] | undefined {
		return this.#landed ? this.items : undefined;
	}

	/**
	 * Fetches the first page, once. A second ask after one has landed does
	 * nothing, and an ask while one is in flight does nothing -- opening
	 * the same disclosure twice is one request, the guard `loadMore`
	 * already makes for every page after the first.
	 *
	 * An ask that *failed* may be asked again: nothing was published, and
	 * the cursor only ever advanced past pages that held no items, so the
	 * retry resumes where the walk stopped rather than repeating rows.
	 */
	async ask(): Promise<void> {
		if (this.#landed || this.isLoadingMore) return;

		const generation = this.#generation;
		this.loadMoreError = '';
		this.isLoadingMore = true;
		try {
			await this.#converge(generation);
		} finally {
			// Unguarded, unlike `loadMore`'s own `finally`: `abandon` is not
			// part of the deferred view, so nothing can supersede a first ask
			// and there is no later load whose flag this could clear. The
			// rows themselves are still guarded, inside `#converge`.
			this.isLoadingMore = false;
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
