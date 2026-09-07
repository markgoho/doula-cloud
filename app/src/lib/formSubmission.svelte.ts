/**
 * A submitted form's own state -- what it is refused for and whether it is
 * mid-flight -- in `PaginatedList`'s mold (#834, #838).
 *
 * `formErrors.ts` already owns the *words* a refusal is reported in. What
 * it does not own is the state around one: 22 routes each held their own
 * `errors = $state<FormError[]>([])` and `isSubmitting = $state(false)`,
 * and 12 of them hand-rolled an identical `errorFor(targetId)` reading the
 * first. They did not all reset the same way either -- some cleared
 * `errors` before a client-side check, some only inside the network
 * branch -- so a refused submit's state depended on which route you were
 * reading. `run` is the one shape: reset, mark busy, let `fn` do the work,
 * turn whatever it threw or returned into `FormError[]`, clear busy.
 *
 * ## What `fn` returns
 *
 * `fn` is the submit itself. `undefined` means it succeeded -- goto a next
 * screen, flip a `hasSubmitted` flag, whatever the route already did on
 * success stays inside `fn`. Anything else it returns, or throws, is *the
 * refusal*: most `fn`s already build a `FormError[]` themselves (a
 * client-side check, an awaited `refusalErrors` call) and simply return
 * it; an uncaught exception -- a dropped connection, an Identity Platform
 * SDK error -- reaches `mapRefusal` the same way. One function reads both,
 * because a route's own reasoning about "what does this refusal mean"
 * does not change depending on whether it arrived by `throw` or `return`.
 *
 * `orServiceProblem` and `orThrownMessage` below are that function for the
 * common case: pass an already-built array through unchanged, and cover
 * whatever else `fn` let through with a generic fallback. A route with its
 * own mapping (`authRefusal`, `totpCodeRefusal`, and the like) writes a
 * short one in place of these, forking the same way.
 *
 * ## What this does not own
 *
 * Focus. `ErrorSummary` already re-focuses itself on its own `$effect`
 * whenever `errors` changes to a non-empty array (#467) -- setting
 * `this.errors` here is what triggers it, so `run` does not need to reach
 * for the DOM itself. ADR-0018's rule stands: the summary is a position
 * the route's own `errorSummary` Snippet renders, never markup this module
 * builds.
 *
 * `errors` and `isSubmitting` are plain public fields, the same choice
 * `PaginatedList` makes for `items`/`hasMore`: `run` is the ordinary way
 * to write them, but a few adopters -- a synchronous client-side check
 * that never touches the network, a `ConfirmDialog`'s `onConfirm` that
 * must rethrow past this module rather than have `run` swallow it -- write
 * `errors` directly, for reasons documented where they do it.
 */

export type { FormError } from './components/molecules/ErrorSummary.svelte';

import { SERVICE_PROBLEM } from './formErrors.js';
import type { FormError } from './components/molecules/ErrorSummary.svelte';

/**
 * Turns whatever `run` caught -- a thrown exception, or a value `fn`
 * returned to mean "refused" -- into what `ErrorSummary` renders.
 */
export type RefusalMapper = (refusal: unknown) => FormError[] | Promise<FormError[]>;

/**
 * The common `mapRefusal`: `fn` already built the `FormError[]` itself, so
 * pass it through, and read anything else -- a network failure, an
 * unreadable response -- as {@link SERVICE_PROBLEM}, since neither carries
 * a message a reader could act on.
 */
export function orServiceProblem(refusal: unknown): FormError[] {
	return Array.isArray(refusal) ? refusal : [{ message: SERVICE_PROBLEM }];
}

/**
 * As {@link orServiceProblem}, but a thrown `Error`'s own message is shown
 * when it has one, for the few call sites that already relied on it
 * rather than the generic wording.
 */
export function orThrownMessage(refusal: unknown): FormError[] {
	if (Array.isArray(refusal)) return refusal;
	return [{ message: refusal instanceof Error && refusal.message ? refusal.message : SERVICE_PROBLEM }];
}

export class FormSubmission {
	/**
	 * The last refusal, or the empty array. `ErrorSummary` renders nothing
	 * for an empty array, so a clean form is never announced.
	 */
	errors = $state<FormError[]>([]);
	/**
	 * Whether a `run` is in flight. Drives a submit button's own spinner,
	 * and `run` reads it back to refuse a second, overlapping call.
	 */
	isSubmitting = $state(false);

	/**
	 * One array is the whole mechanism (#467): the summary lists it and a
	 * control reads its own entry out of the same list, so the two
	 * wordings cannot drift.
	 */
	errorFor(targetId: string): string | undefined {
		return this.errors.find((entry) => entry.targetId === targetId)?.message;
	}

	/**
	 * Runs one submit: resets `errors`, marks `isSubmitting`, awaits `fn`,
	 * and reads a thrown or returned refusal through `mapRefusal`. A
	 * second call while one is already in flight does nothing, the same
	 * double-click guard `PaginatedList.loadMore` gives a "Load more"
	 * button.
	 */
	async run(function_: () => Promise<unknown>, mapRefusal: RefusalMapper): Promise<void> {
		if (this.isSubmitting) return;

		this.errors = [];
		this.isSubmitting = true;
		try {
			const refusal = await function_();
			if (refusal !== undefined) {
				this.errors = await mapRefusal(refusal);
			}
		} catch (error_) {
			this.errors = await mapRefusal(error_);
		} finally {
			this.isSubmitting = false;
		}
	}
}