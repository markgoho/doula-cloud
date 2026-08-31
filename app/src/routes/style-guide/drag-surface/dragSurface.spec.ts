import type { Component } from 'svelte';
import { describe, expect, it } from 'vitest';
import { toDemos, toSlug } from './dragSurface.js';

const stub = (() => {}) as unknown as Component;

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
