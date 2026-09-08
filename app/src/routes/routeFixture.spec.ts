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
import { toRoutePath, toSweptFixtures, type RouteFixture } from './routeFixture.js';

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

/*
 * `toSweptFixtures` is the single reader both halves of the check use for
 * `variants` (#913), so what it does is asserted here rather than twice
 * over in the two halves that call it.
 */
const stub = (() => {}) as unknown as RouteFixture['component'];

function paymentsFixture(variants?: RouteFixture['variants']): RouteFixture {
	return {
		name: 'The Stripe Connect settings screen, as an Owner',
		component: stub,
		params: { practiceId: 'practice-1' },
		url: 'https://example.test/practices/practice-1/settings/payments',
		pageData: { session: { roles: ['owner'] } },
		respond: () => new Response('owner'),
		readyText: 'Getting paid',
		variants
	};
}

describe('toSweptFixtures', () => {
	it('returns a fixture that declares no other session as itself', () => {
		const fixture = paymentsFixture();

		expect(toSweptFixtures(fixture)).toEqual([fixture]);
	});

	it('puts the declared fixture first, then one complete fixture per variant', () => {
		const swept = toSweptFixtures(
			paymentsFixture([
				{ name: 'The Stripe Connect settings screen, as an Admin' },
				{ name: 'The Stripe Connect settings screen, as a Doula' }
			])
		);

		expect(swept.map((fixture) => fixture.name)).toEqual([
			'The Stripe Connect settings screen, as an Owner',
			'The Stripe Connect settings screen, as an Admin',
			'The Stripe Connect settings screen, as a Doula'
		]);
	});

	it('gives a variant everything it did not restate', () => {
		const [, admin] = toSweptFixtures(
			paymentsFixture([{ name: 'The Stripe Connect settings screen, as an Admin' }])
		);

		expect(admin).toMatchObject({
			component: stub,
			params: { practiceId: 'practice-1' },
			url: 'https://example.test/practices/practice-1/settings/payments',
			pageData: { session: { roles: ['owner'] } },
			readyText: 'Getting paid'
		});
	});

	it('lets a variant override every field that carries its session', async () => {
		const [, doula] = toSweptFixtures(
			paymentsFixture([
				{
					name: 'The Stripe Connect settings screen, as a Doula',
					params: { practiceId: 'practice-2' },
					url: 'https://example.test/practices/practice-2/settings/payments',
					props: { data: 'doula' },
					pageData: { session: { roles: ['doula'] } },
					respond: () => new Response('doula'),
					readyText: 'Getting paid, narrower'
				}
			])
		);

		expect(doula).toMatchObject({
			params: { practiceId: 'practice-2' },
			url: 'https://example.test/practices/practice-2/settings/payments',
			props: { data: 'doula' },
			pageData: { session: { roles: ['doula'] } },
			readyText: 'Getting paid, narrower'
		});
		await expect(doula.respond!('/api/anything').text()).resolves.toBe('doula');
	});

	/*
	 * Neither half of the check walks `variants` itself, and a realized
	 * fixture that still carried the field would be one a second caller
	 * could expand again -- so it is dropped rather than left to be
	 * ignored by convention.
	 */
	it('leaves no variants on the fixtures it realizes', () => {
		const swept = toSweptFixtures(
			paymentsFixture([{ name: 'The Stripe Connect settings screen, as an Admin' }])
		);

		expect(swept.map((fixture) => fixture.variants)).toEqual([undefined, undefined]);
	});
});
