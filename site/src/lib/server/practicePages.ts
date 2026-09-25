import { collectPages, type PracticePage } from '#lib/practicePage.js';

/**
 * Every published Practice page, read at build time from the files
 * scripts/sync-practice-pages.ts wrote into site/practice-pages/.
 *
 * A glob rather than a file-system read, so Vite resolves the set once
 * while it builds and the prerenderer never touches the disk itself. An
 * empty or absent directory is an empty list: a local build and a PR
 * preview, neither of which has a database credential, build the site
 * with no Practice pages in it (docs/environment.md).
 */
export const practicePages: PracticePage[] = collectPages(
	import.meta.glob<PracticePage>('/practice-pages/*.json', { eager: true, import: 'default' })
);
