import { globSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { QUOTED, regionLines, regionLinesInFile } from './quotedCopy';

/*
 * The mechanical half of #463's writing rule (`docs/design/brief.md`,
 * "Voice"): product copy names a Client, a Staff member or anyone else by
 * the domain noun or their own first name, never by a gendered pronoun.
 *
 * ## What it reads
 *
 * Every markup line and every quoted script-string in every component and
 * route, region-classified by `quotedCopy.ts`'s `regionLines` with comments
 * already stripped -- so a pronoun named only inside a doc comment, this
 * file's own rationale included, is not an offense. That is deliberate, not
 * a gap: a comment is this repo's documented voice (CLAUDE.md), a separate
 * register from what a screen says out loud, and #463 draws that line
 * explicitly.
 *
 * A markup line is tested whole -- `<p>She joins...</p>` has no quotes at
 * all, and neither does most prose a template renders directly, so a
 * quoted-string-only scan (which is all `formErrors.usage.spec.ts` needed for
 * GOV.UK's banned words) would have missed both violations the issue named.
 * A `<style>` line is skipped outright: CSS carries no product copy. A
 * `<script>` line is scanned only inside its quoted strings, the same as
 * `formErrors.usage.spec.ts`, so a variable or type name is never a false
 * positive.
 *
 * `routes/style-guide/**` is covered too, as of #661. #463's earlier audit
 * found twelve pre-existing hits there, all in component-catalogue example
 * props rather than a real screen a Practice ever sees; #661 rewrote eleven
 * of them, settled the twelfth -- `record-detail`'s birth-plan
 * support-people value, a Client's own first-person answer -- by writing
 * it in the first person rather than building the third-party-quote
 * exemption limit 2 below describes (no real screen needs that exemption
 * yet), and fixed one more the original audit missed
 * (`contract-status:25`).
 *
 * ## What it deliberately does not catch
 *
 * Three limits, named rather than hidden:
 *
 * 1. **Singular "they/them" is not checked.** The brief's rule forbids
 *    every pronoun, not only a gendered one, but "they" is indistinguishable
 *    from the ordinary plural by a word-boundary regex -- flagging it would
 *    drown every legitimate plural in the app in false positives. A reviewer
 *    has to read for this one; nothing here can.
 * 2. **A quoted third party is not exempted.** A Client's own words --
 *    "her partner and her mother" as something a Client wrote about her own
 *    household -- would trip this the same as house copy about a Client
 *    nobody has met. No occurrence of that shape exists in the covered glob
 *    today, so the exemption is not built until a real one forces the
 *    question.
 * 3. **A `.ts` copy constant is not scanned.** The glob is `.svelte` only,
 *    matching the `formErrors.usage.spec.ts` precedent -- a string in a
 *    plain module has no comment/markup structure of its own to separate it
 *    from a doc comment without a real parser.
 */

const PRONOUNS = ['she', 'her', 'hers', 'herself', 'he', 'him', 'his', 'himself'];

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const sourceFiles = globSync('src/{lib/components,routes}/**/*.svelte', { cwd: appRoot });

interface Offense {
	file: string;
	line: number;
	found: string;
	text: string;
}

function offensesIn(text: string): Pick<Offense, 'found' | 'text'>[] {
	const found: Pick<Offense, 'found' | 'text'>[] = [];
	for (const word of PRONOUNS) {
		if (new RegExp(String.raw`\b` + word + String.raw`\b`, 'i').test(text)) {
			found.push({ found: word, text });
		}
	}
	return found;
}

function findOffensesInLines(file: string, lines: ReturnType<typeof regionLines>): Offense[] {
	const found: Offense[] = [];
	for (const { line, text, region } of lines) {
		if (region === 'style') continue;
		const candidates =
			region === 'script'
				? text
						.matchAll(QUOTED)
						.map((match) => (match[1] ?? match[2] ?? match[3])!)
						.toArray()
				: [text];
		for (const candidate of candidates) {
			for (const offense of offensesIn(candidate)) {
				found.push({ file, line, ...offense });
			}
		}
	}
	return found;
}

function findOffenses(file: string): Offense[] {
	return findOffensesInLines(file, regionLinesInFile(file, appRoot));
}

describe('product copy names a Client or Staff member, never a pronoun', () => {
	it('reads every component and route, including the style guide', () => {
		// A glob that silently matched nothing would make every assertion
		// below pass while checking no copy at all.
		expect(sourceFiles.length).toBeGreaterThan(50);
	});

	it('uses no gendered pronoun anywhere a Client or Staff member reads', () => {
		const offenses = sourceFiles.flatMap((file) => findOffenses(file));

		expect(offenses.map((offense) => `${offense.file}:${offense.line} "${offense.text}"`)).toEqual(
			[]
		);
	});
});

describe('findOffensesInLines', () => {
	it('flags a bare-text pronoun in markup, not only a quoted one', () => {
		const offenses = findOffensesInLines('fixture.svelte', regionLines('<p>She joins.</p>'));

		expect(offenses).toEqual([{ file: 'fixture.svelte', line: 1, found: 'she', text: '<p>She joins.</p>' }]);
	});

	it('ignores a pronoun inside an HTML comment', () => {
		const offenses = findOffensesInLines('fixture.svelte', regionLines('<!-- she said so --> <p>ok</p>'));

		expect(offenses).toEqual([]);
	});

	it('ignores a pronoun in a CSS comment inside <style>', () => {
		const source = ['<style>', '/* what she said */', 'p { color: red; }', '</style>'].join('\n');

		expect(findOffensesInLines('fixture.svelte', regionLines(source))).toEqual([]);
	});

	it('flags a pronoun inside a quoted script string, not the surrounding code', () => {
		const source = ['<script>', "// she waits", "const message = 'She waits';", '</script>'].join(
			'\n'
		);

		expect(findOffensesInLines('fixture.svelte', regionLines(source))).toEqual([
			{ file: 'fixture.svelte', line: 3, found: 'she', text: 'She waits' }
		]);
	});

	it('reads a real file from disk the same way', () => {
		// Exercises `regionLinesInFile`'s disk-reading wrapper directly,
		// which the glob-driven test above already covers incidentally --
		// this asserts it in isolation with a file the fix landed clean.
		const offenses = findOffenses('src/lib/components/organisms/OfferSection.svelte');

		expect(offenses).toEqual([]);
	});
});
