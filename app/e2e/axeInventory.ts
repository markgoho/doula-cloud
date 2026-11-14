/*
 * The routes accessibility.e2e.ts does not scan itself, and why (#1810).
 * Every `+page.svelte` under `app/src/routes` is a row `key` in that spec,
 * or a `route` in one of these two lists, and
 * `src/lib/axeInventory.usage.spec.ts` fails the unit suite when one is
 * in none of the three.
 *
 * A route is named the way a row `key` names it: its path under
 * `app/src/routes` with each `(group)` segment removed, and `/` for the
 * root. A `route` that ends in `/*` names the route before the `/*` and
 * every route under it.
 */

export interface ListedRoute {
	route: string;
	reason: string;
}

export interface ScannedElsewhere extends ListedRoute {
	// The spec file in `app/e2e` that scans it.
	spec: string;
}

/*
 * Scanned by another spec, against the same WCAG_TAGS (wcag.ts).
 */
export const SCANNED_ELSEWHERE: ScannedElsewhere[] = [];

/*
 * Not scanned, and each is a decision rather than an oversight.
 */
const CANNOT_BE_REACHED_REASON =
	"The act's later pages (#2934) are reached only with a Doula on a birth with days on call and a time already answered, and the e2e Practice has no birth on call. Each is swept at 320px by the route continuum check on hostile content and specified in cannot-be-reached.svelte.spec.ts.";

export const NOT_SCANNED: ListedRoute[] = [
	{
		route: 'style-guide/*',
		reason:
			'Sixty-odd component demo pages. They are not archetype routes, and scanning them would roughly triple the run for no additional archetype coverage. Every component on them is already scanned in the place a person meets it.'
	},
	{ route: 'demo/*', reason: "SvelteKit's own scaffolding, not a screen in the product." },
	{ route: 'practices/[practiceId]/on-call/cannot-be-reached/when', reason: CANNOT_BE_REACHED_REASON },
	{ route: 'practices/[practiceId]/on-call/cannot-be-reached/cover', reason: CANNOT_BE_REACHED_REASON },
	{ route: 'practices/[practiceId]/on-call/cannot-be-reached/check', reason: CANNOT_BE_REACHED_REASON }
];
