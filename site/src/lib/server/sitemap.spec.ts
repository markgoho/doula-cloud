import { describe, expect, it } from 'vitest';
import type { PracticePage } from '#lib/practicePage.js';
import { practicePages } from './practicePages.js';
import { renderSitemap } from './sitemap.js';

const page: PracticePage = {
	slug: 'river-birth',
	name: 'River Birth',
	serviceDescription: 'Birth support.',
	cancellationPolicy: 'Full refund.',
	supportName: 'Ada Owner',
	supportEmail: 'ada@example.com',
	publishedAt: '2026-09-01T12:00:00Z'
};

describe('renderSitemap', () => {
	it('lists the home page and each Practice page with its publish date', () => {
		const xml = renderSitemap([page]);
		expect(xml).toContain('<url><loc>https://doula.cloud/</loc></url>');
		expect(xml).toContain(
			'<url><loc>https://doula.cloud/p/river-birth</loc><lastmod>2026-09-01T12:00:00.000Z</lastmod></url>'
		);
	});

	it('never lists the unlisted pilot terms, or a /p/ index (#444)', () => {
		const xml = renderSitemap([page]);
		expect(xml).not.toContain('pilot-terms');
		expect(xml).not.toContain('https://doula.cloud/p/</loc>');
	});

	it('is still a valid sitemap with no Practice pages at all', () => {
		expect(renderSitemap([])).toMatch(/<urlset[^>]*>\n<url><loc>https:\/\/doula.cloud\/<\/loc><\/url>\n<\/urlset>/);
	});
});

describe('practicePages', () => {
	it('is whatever the sync script wrote, in slug order', () => {
		const slugs = practicePages.map(({ slug }) => slug);
		expect(slugs).toEqual(slugs.toSorted((a, b) => a.localeCompare(b)));
	});
});
