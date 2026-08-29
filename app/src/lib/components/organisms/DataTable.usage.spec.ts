import { readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/*
 * The unbounded-list gate -- wayfinder #418.
 *
 * The design brief asks that "a dense list stays at 60fps while scrolling
 * under a real Practice's data, not a fixture's". That cannot be measured
 * where the gate runs: a probe on #418 found headless Chromium paces
 * requestAnimationFrame at a fixed ~120Hz no matter what it is rendering,
 * so an fps assertion there always passes. See ADR-0020.
 *
 * So the requirement is enforced on the cause instead. A list stutters
 * because something handed it every row there is; `DataTable` has taken
 * `hasMore`/`onLoadMore` since #97 and a route that ignores them is
 * promising to render a Practice's entire history in one frame. This
 * asserts no route does that.
 *
 * `DataTable.performance.svelte.spec.ts` holds the other half -- what one
 * row is allowed to cost once the count is bounded.
 */

const ROUTES_ROOT = new URL('../../../routes/', import.meta.url).pathname;

/*
 * The four callers that render every row today. Each needs its BFF
 * endpoint to grow a cursor first, which is Go work against
 * `docs/api-design.md` section 4 -- itself already naming Clients and
 * Visits as datasets that must paginate. That is #446, not this ticket.
 *
 * This list only shrinks. A new unbounded list is a new failure, and
 * clearing one here without wiring the route fails the second test below.
 */
const AWAITING_A_CURSOR = new Map([
	['practices/[practiceId]/clients/+page.svelte', 'GET /clients returns a bare array -- #446'],
	['practices/[practiceId]/billing/+page.svelte', 'GET /billing returns the whole ledger -- #446'],
	['practices/[practiceId]/staff/+page.svelte', 'GET /staff returns roster and invitations whole -- #446'],
	[
		'practices/[practiceId]/engagements/[engagementId]/+page.svelte',
		'GET /visits returns a bare array -- #446'
	]
]);

/**
Route files only: the style guide deliberately renders fixed demo data.
*/
function routeFiles(): string[] {
	const found: string[] = [];
	(function walk(directory: string) {
		for (const entry of readdirSync(directory)) {
			const full = path.join(directory, entry);
			if (statSync(full).isDirectory()) {
				if (entry !== 'style-guide') walk(full);
			} else if (entry.endsWith('.svelte')) {
				found.push(full);
			}
		}
	})(ROUTES_ROOT);
	return found.toSorted((left, right) => left.localeCompare(right));
}

/**
Every `<DataTable ... />` usage in a file, as its attribute text.
*/
function dataTableUsages(raw: string): string[] {
	return raw
		.matchAll(/<DataTable\b([\s\S]*?)\/>/g)
		.map((match) => match[1])
		.toArray();
}

function unboundedUsers(): string[] {
	const offenders: string[] = [];
	for (const filePath of routeFiles()) {
		const usages = dataTableUsages(readFileSync(filePath, 'utf8'));
		const unbounded = usages.filter((attributes) => !/\bhasMore\b/.test(attributes));
		if (unbounded.length > 0) offenders.push(path.relative(ROUTES_ROOT, filePath));
	}
	return offenders;
}

describe('a route never hands a list every row there is', () => {
	it('passes hasMore to every DataTable it renders', () => {
		const unexcused = unboundedUsers().filter((route) => !AWAITING_A_CURSOR.has(route));
		expect(unexcused).toEqual([]);
	});

	it('keeps no route on the waiting list once its endpoint paginates', () => {
		const bounded = AWAITING_A_CURSOR.keys()
			.filter((route) => !unboundedUsers().includes(route))
			.toArray();
		expect(bounded).toEqual([]);
	});
});
