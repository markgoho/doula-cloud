/*
 * `toRoutePath` is read by both halves of the continuum check (CONTEXT.md):
 * `route-continuum.svelte.spec.ts` globs `./**` and sees `+page.svelte` and
 * `page.fixture.ts` alike, while the drag surface globs `../../**` and sees
 * only fixtures. Both forms are asserted here, together, because the reason
 * this function exists at all is that the two callers had already written
 * it twice and the copies had drifted -- one required a separator the other
 * made optional, which silently loses the root route.
 */
import { describe, expect, it } from 'vitest';
import { toRoutePath } from './routeFixture.js';

describe('toRoutePath', () => {
	it.each([
		// The check's own form: globbed from `src/routes` itself.
		['./practices/[practiceId]/invoices/+page.svelte', 'practices/[practiceId]/invoices'],
		['./practices/[practiceId]/invoices/page.fixture.ts', 'practices/[practiceId]/invoices'],
		// The drag surface's form: globbed from two directories down.
		['../../practices/[practiceId]/invoices/page.fixture.ts', 'practices/[practiceId]/invoices'],
		['../../account/page.fixture.ts', 'account'],
		// A route group is part of the path, never stripped: it is how the
		// check and the surface name two same-named screens apart.
		['./portal/(signed-out)/login/+page.svelte', 'portal/(signed-out)/login']
	])('reads %s as "%s"', (modulePath, expected) => {
		expect(toRoutePath(modulePath)).toBe(expected);
	});

	/*
	 * The root route. Its fixture is `page.fixture.ts` with no directory in
	 * front of it, so a rule requiring a separator drops `/` -- a screen this
	 * app ships -- out of whichever half used it.
	 */
	it.each([
		['./+page.svelte', ''],
		['../../page.fixture.ts', '']
	])('reads the root route %s as the empty path', (modulePath, expected) => {
		expect(toRoutePath(modulePath)).toBe(expected);
	});
});
