import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const authenticatedRoot = fileURLToPath(new URL('(authenticated)', import.meta.url));

function pagesUnder(directory: string): string[] {
	return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
		const entryPath = path.join(directory, entry.name);
		if (entry.isDirectory()) return pagesUnder(entryPath);
		return entry.name === '+page.svelte' ? [entryPath] : [];
	});
}

const pages = pagesUnder(authenticatedRoot).map((file) => ({
	route: file.slice(authenticatedRoot.length),
	source: readFileSync(file, 'utf8')
}));

/*
 * #2700: Your care and Messages sat at the left of the portal while Birth
 * plan and Contract sat centered, because each Template chose its own width
 * and the portal shell chose none. The shell's `<main>` now owns the column's
 * width and inline position, so a signed-in portal page's only job is to
 * render a Template and leave the frame alone.
 */
describe('signed-in portal pages leave the page frame to the shell and a Template', () => {
	it('finds the signed-in pages', () => {
		expect(pages.length).toBeGreaterThan(0);
	});

	it.each(pages)('$route renders a Template', ({ source }) => {
		expect(source).toMatch(/from '#lib\/components\/templates\/[A-Za-z]+\.svelte'/);
	});

	it.each(pages)('$route hand-writes no page frame or width of its own', ({ source }) => {
		expect(source).not.toMatch(/<(center-l|container-l)\b/);
		expect(source).not.toMatch(/max-inline-size\s*:\s*var\(--(measure|form-max)\)/);
	});
});
