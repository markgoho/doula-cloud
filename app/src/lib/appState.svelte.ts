/*
 * Where a route reads its own params, URL and layout data (#597).
 *
 * This is `$app/state`'s `page` with one seam in it, and the seam exists
 * for exactly one caller: the drag surface. `CONTEXT.md` defines the
 * continuum check and the drag surface as one artifact seen two ways, and
 * #570 had to narrow that -- a route joined the automated half and could
 * not join the human half, because a route reads `page.params` out of
 * `$app/state` and `vi.mock` is a test-runner mechanism that does not
 * exist in a running dev page. #550 is what happened the first day the two
 * halves had something to disagree about, so the narrowing was recorded as
 * a defect rather than accepted.
 *
 * A dev page cannot mock a module, but it can read a different one. So
 * every route reads `page` from here, and the drag surface hands it a
 * fixture's own `params`/`url`/`pageData` before mounting that route.
 * Nothing else in the app ever calls `overridePage`.
 *
 * ## Why the seam is here rather than nowhere
 *
 * The cheaper-looking option was to leave routes on `$app/state` and let a
 * dragged route read the drag surface's own `page` -- empty `params`,
 * empty `data`. That does not merely lose fidelity, it stops the screen
 * rendering at all: eight fixtures answer `respond(path)` by inspecting
 * the path, and a route builds that path out of `page.params`, so a
 * dragged route would fetch `/api/practices/undefined/...` and draw its
 * error state. The human view would then be measuring a different screen
 * than the check, which is the narrowing this ticket exists to undo.
 *
 * The other option was a shim only for the four routes that render
 * `page.data` as visible text, leaving the rest on `$app/state` under a
 * rule ("use the shim if you render it"). #521 and #551 are this repo's
 * own evidence against that: a carrier a session can walk past gets walked
 * past. There is no rule here -- a route reads `page` from `#lib`, and
 * that is the only place it is exported from.
 *
 * ## Why `vi.mock('$app/state')` still works
 *
 * Every spec that mocks `$app/state` keeps working unchanged, because
 * this module *reads* that module rather than replacing it: with the
 * source mocked, `realPage` below is the mock object, and no override is
 * ever set in a test. The check's own mock (`route-continuum.svelte.
 * spec.ts`) is untouched by this file existing.
 */
import { page as realPage } from '$app/state';

/**
What a fixture can say about a route's environment. Deliberately looser
than SvelteKit's own `page`: a fixture describes plain data -- a
`Record` of params, a real `URL`, whatever an ancestor `+layout.ts`
merged in -- and knows nothing about this app's generated route unions
or `App.PageData`.
*/
export interface PageOverride {
	readonly params: Record<string, string>;
	readonly url: URL;
	readonly data: Record<string, unknown>;
}

let override = $state<PageOverride | undefined>();

/**
Hands every route reading `page` a fixture's environment instead of the
real one, until `clearPageOverride`. The drag surface is the only
caller; a `$effect.pre` there clears it when the reader picks something
else or leaves the page.
*/
export function overridePage(next: PageOverride): void {
	override = next;
}

export function clearPageOverride(): void {
	override = undefined;
}

/*
 * A getter per property rather than `override ?? realPage`, so that
 * reading `page.params` inside a `$derived` tracks BOTH the override rune
 * and the real page's own state. Returning one object or the other from a
 * plain expression would read the override once, at module scope, and a
 * route mounted afterwards would never see it change.
 *
 * The three casts are the whole seam, and they are here rather than at
 * each of the 30 call sites on purpose. A route keeps SvelteKit's own
 * types exactly -- `page.params.practiceId` stays a string and
 * `page.data.practiceName` stays what `App.PageData` says it is -- so
 * reading through this module costs a route no type safety. What the cast
 * asserts is the fixture's own claim: that the plain data it supplies
 * describes this route. Nothing checks that claim here, for the same
 * reason nothing here checks `fixture.props` -- a route's own `+page.ts`
 * is what says its data is well-formed.
 */
type RealPage = typeof realPage;

export const page = {
	get params(): RealPage['params'] {
		return (override?.params ?? realPage.params) as RealPage['params'];
	},
	get url(): RealPage['url'] {
		return (override?.url ?? realPage.url) as RealPage['url'];
	},
	get data(): RealPage['data'] {
		return (override?.data ?? realPage.data) as RealPage['data'];
	}
};
