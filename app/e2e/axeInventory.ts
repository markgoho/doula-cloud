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
export const NOT_SCANNED: ListedRoute[] = [
	{
		route: 'style-guide/*',
		reason:
			'Sixty-odd component demo pages. They are not archetype routes, and scanning them would roughly triple the run for no additional archetype coverage. Every component on them is already scanned in the place a person meets it.'
	},
	{ route: 'demo/*', reason: "SvelteKit's own scaffolding, not a screen in the product." }
];
