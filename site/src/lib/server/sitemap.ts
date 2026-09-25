import type { PracticePage } from '#lib/practicePage.js';
import { SITE_ORIGIN } from '#lib/product.js';

/**
 * sitemap.xml for the site: the home page and every Practice page.
 *
 * Listed explicitly rather than discovered, so what a crawler is pointed
 * at is a decision in one place. /pilot-terms is left out on purpose
 * (#444): it is noindexed, and listing a noindexed page publishes the URL
 * the tag exists to withhold. There is no /p/ index to list either.
 *
 * A Practice page's `lastmod` is when she last published it, which is the
 * honest date for a page that changes only when she republishes.
 */
export function renderSitemap(pages: PracticePage[]): string {
	const urls = [
		`<url><loc>${SITE_ORIGIN}/</loc></url>`,
		...pages.map(
			(page) =>
				`<url><loc>${SITE_ORIGIN}/p/${encodeURIComponent(page.slug)}</loc><lastmod>${new Date(page.publishedAt).toISOString()}</lastmod></url>`
		)
	];
	return [
		'<?xml version="1.0" encoding="utf-8" standalone="yes"?>',
		'<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
		...urls,
		'</urlset>',
		''
	].join('\n');
}
