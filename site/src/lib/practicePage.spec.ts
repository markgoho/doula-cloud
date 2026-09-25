import { describe, expect, it } from 'vitest';
import { collectPages, formatUpdated, metaDescription, type PracticePage } from './practicePage.js';

function page(slug: string): PracticePage {
	return {
		slug,
		name: slug,
		serviceDescription: 'Birth support.',
		cancellationPolicy: 'Full refund.',
		supportName: 'Ada Owner',
		supportEmail: 'ada@example.com',
		publishedAt: '2026-09-01T12:00:00.000Z'
	};
}

describe('collectPages', () => {
	it('returns every page in slug order, whatever order the files were listed in', () => {
		const pages = collectPages({
			'/practice-pages/zeta.json': page('zeta'),
			'/practice-pages/alpha.json': page('alpha')
		});
		expect(pages.map(({ slug }) => slug)).toEqual(['alpha', 'zeta']);
	});

	it('is empty when nothing was synced', () => {
		expect(collectPages({})).toEqual([]);
	});
});

describe('metaDescription', () => {
	it('collapses her line breaks and runs of spaces into single spaces', () => {
		expect(metaDescription('  Birth support.\n\nPostpartum   care. ')).toBe(
			'Birth support. Postpartum care.'
		);
	});

	it('cuts a long description at the last whole word and marks the cut', () => {
		const long = `${'word '.repeat(40)}end`;
		const result = metaDescription(long);
		expect(result.endsWith('word…')).toBe(true);
		expect(result.length).toBeLessThanOrEqual(161);
	});

	it('cuts a single unbroken run at the limit when there is no word boundary', () => {
		expect(metaDescription('x'.repeat(200))).toBe(`${'x'.repeat(160)}…`);
	});
});

describe('formatUpdated', () => {
	it('gives the machine date and the words for the same UTC calendar day', () => {
		expect(formatUpdated('2026-09-01T23:30:00.000Z')).toEqual({
			datetime: '2026-09-01',
			text: 'September 1, 2026'
		});
	});

	it('reads a bare date as that day', () => {
		expect(formatUpdated('2026-08-29').text).toBe('August 29, 2026');
	});
});
