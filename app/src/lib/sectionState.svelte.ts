/**
 * One in-page section's load/mutate state: what it holds, the last
 * failure's message, and whether an operation is in flight.
 *
 * The Engagement hub (#841) is nine sections deep and, before this,
 * re-derived the same three facts nine times over: a value, an error
 * string, and a busy flag, each behind its own hand-written
 * `catch (error_) { x = error_ instanceof Error ? error_.message : '...' }`.
 * `PaginatedList` already proved this shape for the three list sections
 * (Visits, Messages, Activity) that page for more; this is the same idea
 * for a section that loads once and is occasionally mutated in place --
 * Care Plan, Birth Plan, the Contract, Invoices, Offers, and the header's
 * portal-invite action.
 *
 * `load` and `mutate` differ in one way: `mutate` will not start a second
 * run while one is already in flight (a double click on "Save" is one
 * request, not two), the same guard `PaginatedList.loadMore` makes. `load`
 * does not guard, because a route's mount-time fetch and a push-triggered
 * refetch (Messages) are each expected to be able to re-run on their own
 * schedule rather than being silently dropped by an earlier call that
 * hasn't settled yet.
 *
 * Both report whether they succeeded, so a caller can chain a second step
 * -- reloading a list after creating a row in it -- without nesting one
 * call inside the other's own try/finally.
 */

export class SectionState<T> {
	value = $state<T>() as T;
	/**
	The last failure's message, or the empty string. Cleared when a load
	or mutate starts.
	*/
	error = $state('');
	/**
	Whether a `load` or `mutate` is in flight.
	*/
	isBusy = $state(false);

	constructor(initial: T) {
		this.value = initial;
	}

	async #run(function_: () => Promise<T>, failureMessage: string): Promise<boolean> {
		this.error = '';
		this.isBusy = true;
		try {
			this.value = await function_();
			return true;
		} catch (error_) {
			this.error = error_ instanceof Error ? error_.message : failureMessage;
			return false;
		} finally {
			this.isBusy = false;
		}
	}

	/**
	 * Runs `function_`, storing what it resolves to as `value`. On a
	 * thrown `Error`, `error` becomes that error's own message; on
	 * anything else thrown, `error` becomes `failureMessage`, so a
	 * rejected non-Error never renders an empty error box.
	 */
	async load(function_: () => Promise<T>, failureMessage: string): Promise<boolean> {
		return this.#run(function_, failureMessage);
	}

	/**
	 * Same as `load`, except a call while one is already in flight does
	 * nothing and reports failure -- a double click is one request.
	 */
	async mutate(function_: () => Promise<T>, failureMessage: string): Promise<boolean> {
		if (this.isBusy) return false;
		return this.#run(function_, failureMessage);
	}
}
