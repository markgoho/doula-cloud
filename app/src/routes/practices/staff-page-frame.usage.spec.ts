import { existsSync, readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const practicesRoot = fileURLToPath(new URL('.', import.meta.url));

function svelteFilesUnder(directory: string): string[] {
	return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
		const entryPath = path.join(directory, entry.name);
		if (entry.isDirectory()) return svelteFilesUnder(entryPath);
		return entry.name.endsWith('.svelte') ? [entryPath] : [];
	});
}

const route = (file: string) => file.slice(practicesRoot.length);


const TEMPLATE_IMPORT = /from '#lib\/components\/templates\/[A-Za-z]+\.svelte'/;
const LOCAL_COMPONENT_IMPORT = /from '(\.{1,2}\/[^']+\.svelte)'/g;
const FRAME = /<(center-l|container-l)\b/;

function isTemplateRendered(file: string, seen = new Set<string>()): boolean {
	if (seen.has(file)) return false;
	seen.add(file);
	const source = readFileSync(file, 'utf8');
	if (TEMPLATE_IMPORT.test(source)) return true;
	return source
		.matchAll(LOCAL_COMPONENT_IMPORT)
		.map(([, specifier]) => path.resolve(path.dirname(file), specifier))
		.some(
			(imported) => existsSync(imported) && isTemplateRendered(imported, seen)
		);
}

const files = svelteFilesUnder(practicesRoot).map((file) => ({
	file,
	route: route(file),
	source: readFileSync(file, 'utf8'),
}));
const pages = files.filter(
	({ file }) => path.basename(file) === '+page.svelte'
);

/*
 * #2701: a Staff page's inline start edge is its Template's (ADR-0018's
 * 2026-11-07 amendment). ListPage, RecordDetail and OverviewHub start at
 * the page gutter, the capped Templates center their column, and nothing
 * under practices/ draws a frame of its own, so a new Staff page takes
 * its edge by choosing a Template and has no other way to choose one.
 */
describe('Staff pages leave the page frame to a Template', () => {
	it('finds the Staff pages', () => {
		expect(pages.length).toBeGreaterThan(0);
	});

	it.each(pages)(
		'$route renders a Template, itself or through a local component',
		({ file }) => {
			expect(isTemplateRendered(file)).toBe(true);
		}
	);

	it.each(files)('$route hand-writes no page frame', ({ source }) => {
		expect(source).not.toMatch(FRAME);
	});
});
