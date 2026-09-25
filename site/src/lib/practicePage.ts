/**
 * A Practice's published page at doula.cloud/p/<slug> (#441).
 *
 * This file is the contract between the two halves of the page:
 * scripts/sync-practice-pages.ts reads the rows out of Postgres and writes
 * one JSON file per page into site/practice-pages/, and the route at
 * src/routes/p/[slug]/ prerenders each file into HTML. The script imports
 * the type from here, so the two cannot drift.
 *
 * No Svelte, Vite or Bun APIs are used in this file, because the root
 * `scripts/` tree imports it and runs under `bun test`.
 */

/*
 * One published page, as the sync script reads it out of Postgres.
 *
 * - slug: the path segment of doula.cloud/p/<slug>, assigned once (00046).
 * - name: the Practice's name, and the business name Stripe asks for.
 * - serviceDescription: what she offers, in her own words, up to 500
 *   characters.
 * - cancellationPolicy: her cancellation or refund position, up to 500
 *   characters.
 * - supportName: the Owner who published, shown as the contact.
 * - supportEmail: that Owner's address -- how a Client reaches the
 *   Practice.
 * - publishedAt: when the page was last published, RFC 3339.
 */
export interface PracticePage {
	slug: string;
	name: string;
	serviceDescription: string;
	cancellationPolicy: string;
	supportName: string;
	supportEmail: string;
	publishedAt: string;
}

/**
 * Every page, in slug order, from the modules a glob over the generated
 * directory returned.
 *
 * Sorted so the sitemap and the prerender entries come out in the same
 * order on every build, whatever order the file system listed them in.
 */
export function collectPages(modules: Record<string, PracticePage>): PracticePage[] {
	return Object.values(modules).toSorted((a, b) => a.slug.localeCompare(b.slug));
}

/**
 * The longest a meta description runs before a search engine cuts it
 * itself. The same limit the Hugo layout used.
 */
const META_DESCRIPTION_MAX = 160;

/**
 * The page's meta description, from the service description she typed.
 *
 * Whitespace is collapsed first, because her line breaks are prose
 * structure on the page and noise in a search result. A description that
 * is too long is cut at the last word that fits and ends with an
 * ellipsis, so a result never shows half a word.
 */
export function metaDescription(text: string): string {
	const collapsed = text.replaceAll(/\s+/g, ' ').trim();
	if (collapsed.length <= META_DESCRIPTION_MAX) return collapsed;
	const cut = collapsed.slice(0, META_DESCRIPTION_MAX);
	const lastSpace = cut.lastIndexOf(' ');
	return `${(lastSpace > 0 ? cut.slice(0, lastSpace) : cut).trimEnd()}…`;
}

const longDate = new Intl.DateTimeFormat('en-US', {
	day: 'numeric',
	month: 'long',
	year: 'numeric',
	timeZone: 'UTC'
});

/**
 * A "Last updated" date: the calendar date of an instant, in UTC.
 *
 * UTC because the site is built once for everyone who reads it and has no
 * reader's zone to use, and because it is the zone the Hugo site printed
 * these dates in.
 */
export function formatUpdated(instant: string): { datetime: string; text: string } {
	const date = new Date(instant);
	return { datetime: date.toISOString().slice(0, 10), text: longDate.format(date) };
}
