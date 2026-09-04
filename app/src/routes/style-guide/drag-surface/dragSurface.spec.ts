import type { Component } from 'svelte';
import { describe, expect, it } from 'vitest';
import type { RouteFixture } from '../../routeFixture.js';
import { respondWith, toDemos, toRouteDemos, toSlug } from './dragSurface.js';

const stub = (() => {}) as unknown as Component;

function routeFixture(overrides: Partial<RouteFixture> = {}): RouteFixture {
	return {
		name: 'The Practice-wide invoice list',
		component: stub as RouteFixture['component'],
		params: { practiceId: 'practice-1' },
		url: 'https://example.test/practices/practice-1/invoices',
		readyText: 'Invoices',
		...overrides
	};
}

describe('toSlug', () => {
	it.each([
		['../description-list/+page.svelte', 'description-list'],
		['../data-table/+page.svelte', 'data-table']
	])('reads %s as "%s"', (modulePath, expected) => {
		expect(toSlug(modulePath)).toBe(expected);
	});

	it('is empty for a path with no directory to read', () => {
		expect(toSlug('+page.svelte')).toBe('');
	});
});

describe('toDemos', () => {
	it('pairs each registered page with the module of the same slug', () => {
		const demoList = toDemos({ '../button/+page.svelte': { default: stub } }, [
			{ name: 'Button', slug: 'button' }
		]);

		expect(demoList).toEqual([{ name: 'Button', slug: 'button', component: stub }]);
	});

	it('keeps the registry order rather than the glob order', () => {
		const demoList = toDemos(
			{
				'../text/+page.svelte': { default: stub },
				'../button/+page.svelte': { default: stub }
			},
			[
				{ name: 'Button', slug: 'button' },
				{ name: 'Text', slug: 'text' }
			]
		);

		expect(demoList.map((demo) => demo.slug)).toEqual(['button', 'text']);
	});

	// A component whose page is missing is the style-guide coverage spec's
	// finding to report, not a crash here.
	it('drops a registered page that has no module', () => {
		expect(toDemos({}, [{ name: 'Button', slug: 'button' }])).toEqual([]);
	});
});

describe('toRouteDemos', () => {
	it('names each route the way its own fixture names it', () => {
		const demos = toRouteDemos({
			'../../practices/[practiceId]/invoices/page.fixture.ts': { fixture: routeFixture() }
		});

		expect(demos).toEqual([
			{
				name: 'The Practice-wide invoice list',
				slug: 'practices/[practiceId]/invoices',
				component: stub,
				fixture: expect.objectContaining({ readyText: 'Invoices' })
			}
		]);
	});

	// A glob's key order is not the reader's order, so the picker would
	// otherwise reshuffle as routes come and go.
	it('sorts by route path rather than by glob order', () => {
		const demos = toRouteDemos({
			'../../practices/[practiceId]/invoices/page.fixture.ts': { fixture: routeFixture() },
			'../../account/page.fixture.ts': { fixture: routeFixture({ name: 'The account screen' }) }
		});

		expect(demos.map((demo) => demo.slug)).toEqual(['account', 'practices/[practiceId]/invoices']);
	});
});

const echoPath = (path: string) => new Response(path);

describe('respondWith', () => {
	const baseURL = 'https://api.example.test';

	it('answers an absolute request with what the fixture says for its path', async () => {
		const answer = respondWith(echoPath, baseURL, 'A screen');

		const response = await answer(`${baseURL}/api/practices/practice-1/invoices`);

		await expect(response.text()).resolves.toBe('/api/practices/practice-1/invoices');
	});

	/*
	 * `apiBaseURL()` is '' on a real deploy, where the app and the BFF share
	 * an origin -- so the path arrives already relative and must not be
	 * mangled by stripping a prefix that is not there.
	 */
	it('answers a same-origin request, where there is no base URL to strip', async () => {
		const answer = respondWith(echoPath, '', 'A screen');

		const response = await answer('/api/staff/session');

		await expect(response.text()).resolves.toBe('/api/staff/session');
	});

	it('reads a URL and a Request the same way it reads a string', async () => {
		const answer = respondWith(echoPath, baseURL, 'A screen');

		const fromURL = await answer(new URL(`${baseURL}/api/a`));
		const fromRequest = await answer(new Request(`${baseURL}/api/b`));

		await expect(fromURL.text()).resolves.toBe('/api/a');
		await expect(fromRequest.text()).resolves.toBe('/api/b');
	});

	/*
	 * A fixture with no `respond` describes a route that fetches nothing.
	 * If such a route fetches anyway, the fixture is wrong -- and letting it
	 * through to the real BFF would render whatever a developer's own
	 * database holds, which is a screen the continuum check never swept.
	 */
	it('refuses a fetch the fixture never described, naming the screen', async () => {
		const answer = respondWith(undefined, baseURL, 'The account screen');

		await expect(answer(`${baseURL}/api/account`)).rejects.toThrow(
			/The account screen fetched \/api\/account/
		);
	});
});
