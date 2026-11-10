/**
 * A whole page's own read, as one value: still loading, loaded with what
 * it read, or failed with why.
 *
 * Before this, each route held the same fact as two flags -- an
 * `isLoaded` and a `loadError`, or a value that doubled as its own
 * "not yet" sentinel -- and hand-derived the Template's two state props
 * from them, at least seven ways. Two independent flags make four
 * combinations, and both wrong ones were reached: a thrown read that set
 * neither (a permanent Skeleton), and a `finally { isLoaded = true }`
 * that drew a failed read as a loaded, empty form. Every Template then
 * guarded the overlap with `Boolean(loading) && !loadError`. One value
 * has three states and no combinations, so neither can happen.
 *
 * Beside `SectionState`, not on it: a section always holds a value (its
 * initial one until a read lands) and shares one `error` between its
 * reads and its mutations, which is the right shape for a section that
 * keeps drawing whatever it last had. A page draws nothing until its
 * first read lands, so "loaded" is a state of its own here, and a
 * re-read after a save (`refresh`) never takes a loaded page back to
 * its failure screen. `Streamed` is the same idea for a read a route's
 * `load` hands its screen; this is for one the screen starts itself.
 *
 * Like `Streamed`, the latest read wins: a read that settles after a
 * later one has started is dropped, so a slow answer for the record
 * that was open before never overwrites the one open now.
 */

/**
The three states, one name each. `loaded` carries what was read and
`failed` the sentence a reader is shown.
*/
export type ReadState<Value> =
	| { status: 'loading' }
	| { status: 'loaded'; value: Value }
	| { status: 'failed'; message: string };

/**
What a page Template draws for its page's read (`FormPage`, `ListPage`,
`DocumentPage`, `OverviewHub`, `RecordDetail`): the Skeleton and the
label it is announced by, the Notice and its message, or the page.
Loading cannot be asked for without saying what is loading (#480).
*/
export type PageReadState =
	| { status: 'loading'; label: string }
	| { status: 'loaded' }
	| { status: 'failed'; message: string };

const LOADING: ReadState<never> = { status: 'loading' };

/**
A page with nothing to read, or whose read has landed: what a Template
draws when it is given no `read` at all.
*/
export const LOADED_PAGE: PageReadState = { status: 'loaded' };

/**
The props a page Template takes for its `<h1>` and its tab title.
*/
export interface PageTitling {
	/**
	The one `<h1>` and the tab title, in every state of the read.
	*/
	title: string;
	/**
	 * The Staff side leaves this unset and gets the product name; the
	 * Client portal passes its Practice's name (#431, #487).
	 */
	serviceName?: string;
}

/**
 * How a read's failure is worded: a sentence, used when what was thrown
 * has no message of its own (`readFailure`), or a function of what was
 * thrown, for a screen that words every failure itself.
 */
export type ReadFailure = string | ((error: unknown) => string);

/**
 * The words a failed read is reported in: a thrown `Error`'s own message
 * when it has one, `fallback` otherwise, so a failure is never drawn as
 * an empty Notice.
 */
export function readFailure(error: unknown, fallback: string): string {
	return error instanceof Error && error.message ? error.message : fallback;
}

/**
 * The one derivation from a read's state to what a Template draws.
 * `label` names what is loading for the Skeleton.
 */
export function pageReadState(state: ReadState<unknown>, label: string): PageReadState {
	if (state.status === 'loading') return { status: 'loading', label };
	if (state.status === 'failed') return { status: 'failed', message: state.message };
	return LOADED_PAGE;
}

/**
 * Two reads as one, for a page that needs both (`streamedRead`'s
 * `bothRead`, for reads a screen starts itself): the first failure wins,
 * and it is loaded only once both are.
 */
export function bothReads<First, Second>(
	first: ReadState<First>,
	second: ReadState<Second>
): ReadState<[First, Second]> {
	if (first.status === 'failed') return first;
	if (second.status === 'failed') return second;
	if (first.status === 'loading' || second.status === 'loading') return LOADING;
	return { status: 'loaded', value: [first.value, second.value] };
}

export class PageRead<Value = void> {
	#generation = 0;
	/**
	 * Deep, not raw, as each route's own `$state` was: a loaded record a
	 * route edits in place stays reactive, and a re-read that answers with
	 * the same array it answered before is still drawn afresh.
	 */
	state = $state<ReadState<Value>>(LOADING);
	/**
	The last `refresh`'s failure while the page stayed drawn, or the empty
	string. A route shows it beside what it is about; the page does not go.
	*/
	refreshError = $state('');

	async #settle(read: () => Promise<Value>, failureMessage: ReadFailure, isRefresh: boolean): Promise<boolean> {
		this.#generation += 1;
		const generation = this.#generation;
		try {
			const value = await read();
			if (generation !== this.#generation) return false;
			this.state = { status: 'loaded', value };
			return true;
		} catch (error) {
			if (generation !== this.#generation) return false;
			const message =
				typeof failureMessage === 'string' ? readFailure(error, failureMessage) : failureMessage(error);
			if (isRefresh && this.state.status === 'loaded') {
				this.refreshError = message;
			} else {
				this.state = { status: 'failed', message };
			}
			return false;
		}
	}

	/**
	What was read, once loaded; `undefined` before and after a failure.
	*/
	get value(): Value | undefined {
		return this.state.status === 'loaded' ? this.state.value : undefined;
	}

	get isLoaded(): boolean {
		return this.state.status === 'loaded';
	}

	/**
	 * A fresh read: the page goes back to loading, then to loaded or
	 * failed. Resolves `true` only when this read's value was the one
	 * written, so a caller's next step (opening a draft from the record)
	 * never runs on a read that a later one replaced.
	 */
	async load(read: () => Promise<Value>, failureMessage: ReadFailure): Promise<boolean> {
		this.state = LOADING;
		this.refreshError = '';
		return this.#settle(read, failureMessage, false);
	}

	/**
	 * A re-read in place, after a save: the page stays as it is while it
	 * runs. On failure a loaded page stays loaded, with what it last read,
	 * and `refreshError` says why; a page that never loaded fails.
	 */
	async refresh(read: () => Promise<Value>, failureMessage: ReadFailure): Promise<boolean> {
		this.refreshError = '';
		return this.#settle(read, failureMessage, true);
	}

	/**
	A save's own answer, written as what the page now holds.
	*/
	set(value: Value): void {
		this.#generation += 1;
		this.state = { status: 'loaded', value };
		this.refreshError = '';
	}

	/**
	A failure the route found itself, in a read that answered.
	*/
	fail(message: string): void {
		this.#generation += 1;
		this.state = { status: 'failed', message };
	}

	/**
	Back to loading, with any read in flight dropped.
	*/
	reset(): void {
		this.#generation += 1;
		this.state = LOADING;
		this.refreshError = '';
	}

	/**
	What the page's Template draws; `label` names what is loading.
	*/
	frame(label: string): PageReadState {
		return pageReadState(this.state, label);
	}
}
