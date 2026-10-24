import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { globFiles } from './globFiles';

/*
 * The source half of #1828. `Link` and the `link` variant of `Button` read
 * a `<p>` around them as "this link is inside a sentence" and drop the
 * brief's 44px floor there (Density, the "Inline" exception of WCAG 2.2's
 * 2.5.8). So a link that stands alone must not be written inside a `<p>`:
 * `<p><Link ... /></p>` gives a screen's one action a hit target one line
 * of text tall, and the rendered page would not say why.
 *
 * It reads the markup, not a fixture's rendered page, so no fixture can
 * make it pass: every `.svelte` file under components and routes is read
 * as it is written. A paragraph that holds a link and words is a sentence
 * and passes; a paragraph that holds a link and nothing else fails.
 *
 * What it does not catch: a sentence written outside a `<p>` (in an `<li>`
 * or a `<span>`), where the link keeps its 44px box and pushes the line
 * apart. That one shows on the rendered page, and review reads for it.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const sourceFiles = globFiles('src/{lib/components,routes}/**/*.svelte', { cwd: appRoot });

// A `<p>` whose only content is one `<Link ... />` or one link-variant
// `<Button ... />`. `[^<]*?` keeps a match inside one tag, so a paragraph
// with words between two links is never read as a lone link.
const LONE_LINK = /<p(?:\s[^>]*)?>\s*<(?:Link\b|Button\b(?=[^<]*variant="link"))[^<]*?\/>\s*<\/p>/g;

function loneLinksIn(source: string): string[] {
	return source.match(LONE_LINK) ?? [];
}

describe('a link that stands alone is not written inside a <p> (#1828)', () => {
	it('finds a lone link in a paragraph, and passes one inside a sentence', () => {
		expect(loneLinksIn('<p><Link href="/a" label="Add your first Client" /></p>')).toHaveLength(1);
		expect(loneLinksIn('<p class="x">\n\t<Button variant="link" label="Sign out" onClick={go} />\n</p>')).toHaveLength(1);
		expect(loneLinksIn('<p>Read the <Link href="/a" label="Privacy Policy" />.</p>')).toEqual([]);
		expect(loneLinksIn('<p><Link href="/a" label="Terms" /> and the <Link href="/b" label="Privacy" /></p>')).toEqual([]);
		expect(loneLinksIn('<p><Button label="Save" onClick={go} /></p>')).toEqual([]);
	});

	it('reads at least one source file', () => {
		expect(sourceFiles.length).toBeGreaterThan(0);
	});

	it.each(sourceFiles)('%s', (file) => {
		const source = readFileSync(path.join(appRoot, file), 'utf8');

		expect(loneLinksIn(source)).toEqual([]);
	});
});
