/**
 * A cursor-paged ops list that reads its own first page: still loading,
 * loaded with the rows so far, failed with why, or signed out.
 *
 * `app/src/lib/paginatedList.svelte.ts`'s job, for a list no route `load`
 * hands a first page to. Four ops lists (the Operator log, the Practices,
 * Feedback and the Dormant Practices) each held `rows`, `nextCursor`,
 * `hasLoaded`, `isLoading` and `errors`, and one `load(cursor)` with no
 * catch, so a dropped connection drew nothing and said nothing. Here a
 * throw is a failure like a refusal. A later page's failure leaves the rows
 * on screen and says why beside them.
 */

import { serviceProblem, type Outcome } from './act.ts';
import type { FormError } from './authRefusal.ts';

/**
One page of a cursor-paged doula-ops list (docs/api-design.md section 4).
*/
export type CursorPage<Item> = { items: Item[]; nextCursor?: string; hasMore: boolean };

/**
The four states, one name each. `loaded` carries the rows and the cursor of the page after them, when there is one.
*/
export type ListState<Item> =
	| { status: 'loading' }
	| { status: 'loaded'; items: Item[]; nextCursor: string | undefined }
	| { status: 'failed'; errors: FormError[] }
	| { status: 'signed-out' };

export class PagedList<Item> {
	#read: (cursor: string | undefined) => Promise<Outcome<CursorPage<Item>>>;
	#onsignedout: () => unknown;

	state = $state<ListState<Item>>({ status: 'loading' });
	/**
	Whether a later page is being read. Drives the "Show more" button's own spinner.
	*/
	isLoadingMore = $state(false);
	/**
	Why the last later page failed, or the empty array; the rows stay.
	*/
	moreErrors = $state<FormError[]>([]);

	constructor(read: (cursor: string | undefined) => Promise<Outcome<CursorPage<Item>>>, onsignedout: () => unknown) {
		this.#read = read;
		this.#onsignedout = onsignedout;
	}

	async #page(cursor: string | undefined): Promise<Outcome<CursorPage<Item>>> {
		try {
			return await this.#read(cursor);
		} catch {
			return { kind: 'refused', errors: serviceProblem() };
		}
	}

	async #signedOut(): Promise<void> {
		this.state = { status: 'signed-out' };
		await this.#onsignedout();
	}

	/**
	What the list's `ErrorSummary` shows: why the first page failed, or why a later one did.
	*/
	get errors(): FormError[] {
		return this.state.status === 'failed' ? this.state.errors : this.moreErrors;
	}

	/**
	Reads the first page.
	*/
	async load(): Promise<void> {
		this.state = { status: 'loading' };
		this.moreErrors = [];
		const outcome = await this.#page(undefined);
		if (outcome.kind === 'signed-out') return this.#signedOut();
		if (outcome.kind === 'refused') {
			this.state = { status: 'failed', errors: outcome.errors };
			return;
		}
		const { items, hasMore, nextCursor } = outcome.value;
		this.state = { status: 'loaded', items, nextCursor: hasMore ? nextCursor : undefined };
	}

	/**
	 * Appends the page after the last one read. Does nothing before the list
	 * has loaded, when no page remains, or while one is already being read.
	 */
	async more(): Promise<void> {
		const before = this.state;
		if (before.status !== 'loaded' || before.nextCursor === undefined || this.isLoadingMore) return;
		this.isLoadingMore = true;
		this.moreErrors = [];
		const outcome = await this.#page(before.nextCursor);
		this.isLoadingMore = false;
		if (outcome.kind === 'signed-out') return this.#signedOut();
		if (outcome.kind === 'refused') {
			this.moreErrors = outcome.errors;
			return;
		}
		const { items, hasMore, nextCursor } = outcome.value;
		this.state = { status: 'loaded', items: [...before.items, ...items], nextCursor: hasMore ? nextCursor : undefined };
	}
}
