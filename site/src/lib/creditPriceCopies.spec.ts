import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { TERMS, creditPrice } from './legal.js';

/*
 * The documents that state the price of a Credit by hand (#1841). Each
 * version of the Terms states the price, and every page prints it from
 * there (legal.ts's creditPrice); a Markdown document cannot import a
 * value, so this spec reads each one and fails when the price it states
 * is not the price of the newest version of the Terms.
 *
 * The newest version, not the one in force: a rise lands with its text
 * 60 days or more before it starts, and these documents change in that
 * same commit (docs/environment.md, "A rise in the price of a Credit"),
 * so during the notice they already state the coming price. Comparing
 * them with the version in force would fail for the whole notice.
 *
 * The site job runs on every PR, a docs-only one included, so a document
 * edited alone is held to this too.
 */
const repoRoot = new URL('../../../', import.meta.url);

function read(path: string): string {
	return readFileSync(fileURLToPath(new URL(path, repoRoot)), 'utf8');
}

// Each document, and the sentences in it that state the price. A pattern
// captures the price as written.
const COPIES: readonly { path: string; sentences: readonly RegExp[] }[] = [
	{
		path: 'docs/copy/pilot-terms-page.md',
		sentences: [/A Credit costs \*\*(\$[\d,.]+)\*\*/g, /A Credit still costs (\$[\d,.]+)\./g]
	},
	{ path: 'docs/copy/terms-of-service-page.md', sentences: [/A Credit costs \*\*(\$[\d,.]+)\*\*/g] }
];

const newestVersion = TERMS.versions.at(-1);
if (!newestVersion) throw new Error('the Terms of Service have no version');
const newest = creditPrice(newestVersion);

describe('the documents that state the price of a Credit by hand (#1841)', () => {
	it.each(COPIES)('$path states the price of the newest version of the Terms', ({ path, sentences }) => {
		const text = read(path);
		for (const sentence of sentences) {
			const prices = text
				.matchAll(sentence)
				.map(([, price]) => price)
				.toArray();
			expect(prices, `${path} has no sentence like ${sentence}`).not.toEqual([]);
			expect(prices).toEqual(prices.map(() => newest));
		}
	});

	// So a rise finds every copy from one list, and a copy added here is
	// added to the procedure too.
	it('the procedure for a rise names every one of them', () => {
		const procedure = /^### A rise in the price of a Credit\n([\s\S]*?)^### /m.exec(read('docs/environment.md'));
		expect(procedure?.[1], 'docs/environment.md has no "A rise in the price of a Credit" section').toBeDefined();
		for (const { path } of COPIES) expect(procedure?.[1]).toContain(`\`${path}\``);
	});
});
