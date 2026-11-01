import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { globFiles } from './globFiles';
import { copyCandidates, NOT_COPY } from './quotedCopy';

/*
 * The mechanical half of #463's writing rule (`docs/design/brief.md`,
 * "Voice"): product copy names a Client, a Staff member or anyone else by
 * the domain noun or their own first name, never by a gendered pronoun.
 *
 * ## What it reads
 *
 * Every markup line and every quoted script-string in every component and
 * route, and every quoted string in a copy module ("Copy modules" below),
 * region-classified by `quotedCopy.ts`'s `regionLines` with comments
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
 * found twelve pre-existing hits there, all in component-catalog example
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
 * Two limits, named rather than hidden:
 *
 * 1. **Singular "they/them" is checked only in one shape.** The brief's
 *    rule forbids every pronoun, not only a gendered one, but "they" is
 *    indistinguishable from the ordinary plural by a word-boundary regex --
 *    a trial run in #2198 flagged 37 lines, nearly all plurals of things
 *    ("reorder them", "pay their invoices"). What is checked is "them" as
 *    the person something is sent or told to (`SINGULAR_THEM`):
 *    "An email has been sent to them" was the line #2198 found, and in
 *    that shape "them" is a person every time. Every other "they", "them"
 *    and "their" is still the reviewer's to read for.
 * 2. **A quoted third party is not exempted.** A Client's own words --
 *    "her partner and her mother" as something a Client wrote about her own
 *    household -- would trip this the same as house copy about a Client
 *    nobody has met. No occurrence of that shape exists in the covered glob
 *    today, so the exemption is not built until a real one forces the
 *    question.
 *
 * ## Copy modules (#2092)
 *
 * A quoted string in a `.ts` module under `src/lib` or `src/routes` is read
 * too, the way `copy.dash.usage.spec.ts` (#2023) reads it: comments
 * stripped by `quotedCopy.ts`, every string literal tested, specs and
 * fixtures left out because their strings are test names and hostile test
 * data, not a screen. #2092 found "Send her an Offer" built in `staff.ts`,
 * which the `.svelte`-only glob could not see.
 *
 * A string whose word is not a pronoun for a person -- a quoted title, a
 * token -- carries `voice:ignore` and the reason on the same line, the
 * marker the other two copy specs read.
 */

const PRONOUNS = ['she', 'her', 'hers', 'herself', 'he', 'him', 'his', 'himself'];

// "them" as the one a message or a reminder goes to: a person, never a
// plural of things (limit 1 above, #2198). Not "ask": "asks them" is
// how a Plan Template says it asks its questions.
const SINGULAR_THEM =
	/\b(?:sends?|sent|emails?|emailed|tells?|told|invites?|invited|reminds?|reminded|messages?|messaged|notify|notifies|notified|lets?)\s+(?:to\s+)?them\b/i;

const MARKER = 'voice:ignore';

// A marker only counts with a reason after it.
const MARKER_WITH_REASON = new RegExp(String.raw`${MARKER}\W*\w`);

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const GLOBS = ['src/{lib/components,routes}/**/*.svelte', 'src/{lib,routes}/**/*.ts'];


// #2469: ops/ (ADR-0062) is held to the same voice. Its paths are made
// relative to app/, so the read below resolves them as it does app/'s own.
const opsCopyFiles = globFiles(['ops/src/**/*.svelte', 'ops/src/**/*.ts'], {
	cwd: fileURLToPath(new URL('../../../', import.meta.url))
})
	.filter((file) => !NOT_COPY.test(file))
	.map((file) => `../${file}`);

const sourceFiles = [
	...globFiles(GLOBS, { cwd: appRoot }).filter((file) => !NOT_COPY.test(file)),
	...opsCopyFiles
];

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
	if (SINGULAR_THEM.test(text)) found.push({ found: 'them', text });
	return found;
}

function findOffensesInSource(file: string, source: string): Offense[] {
	const raw = source.split('\n');
	return copyCandidates(file, source)
		.filter(({ line }) => !MARKER_WITH_REASON.test(raw[line - 1]!))
		.flatMap(({ line, text }) => offensesIn(text).map((offense) => ({ file, line, ...offense })));
}

function findOffenses(file: string): Offense[] {
	return findOffensesInSource(file, readFileSync(new URL(file, `file://${appRoot}`), 'utf8'));
}

// Read and scanned once, at module scope, so the cost of the whole-tree
// read is paid on import rather than charged against one `it`'s 5-second
// `testTimeout` (#1211).
const offenses = sourceFiles.flatMap((file) => findOffenses(file));

describe('product copy names a Client or Staff member, never a pronoun', () => {
	it('reads every component, route and copy module, including the style guide', () => {
		// A glob that silently matched nothing would make every assertion
		// below pass while checking no copy at all.
		expect(sourceFiles.filter((file) => file.endsWith('.svelte')).length).toBeGreaterThan(50);
		expect(sourceFiles.filter((file) => file.endsWith('.ts')).length).toBeGreaterThan(50);
		expect(opsCopyFiles.length).toBeGreaterThan(3);
	});

	it('uses no gendered pronoun anywhere a Client or Staff member reads', () => {
		expect(offenses.map((offense) => `${offense.file}:${offense.line} "${offense.text}"`)).toEqual(
			[]
		);
	});
});

describe('findOffensesInSource', () => {
	it('flags a bare-text pronoun in markup, not only a quoted one', () => {
		const offenses = findOffensesInSource('fixture.svelte', '<p>She joins.</p>');

		expect(offenses).toEqual([{ file: 'fixture.svelte', line: 1, found: 'she', text: '<p>She joins.</p>' }]);
	});

	it('ignores a pronoun inside an HTML comment', () => {
		const offenses = findOffensesInSource('fixture.svelte', '<!-- she said so --> <p>ok</p>');

		expect(offenses).toEqual([]);
	});

	it('ignores a pronoun in a CSS comment inside <style>', () => {
		const source = ['<style>', '/* what she said */', 'p { color: red; }', '</style>'].join('\n');

		expect(findOffensesInSource('fixture.svelte', source)).toEqual([]);
	});

	it('flags a pronoun inside a quoted script string, not the surrounding code', () => {
		const source = ['<script>', '// she waits', "const message = 'She waits';", '</script>'].join(
			'\n'
		);

		expect(findOffensesInSource('fixture.svelte', source)).toEqual([
			{ file: 'fixture.svelte', line: 3, found: 'she', text: 'She waits' }
		]);
	});

	it('flags a quoted string in a .ts module, not its comments or code', () => {
		const source = [
			'/** she is named once */',
			'const her = 1;',
			'export const HINT = `Send her an Offer`;'
		].join('\n');

		expect(findOffensesInSource('fixture.ts', source)).toEqual([
			{ file: 'fixture.ts', line: 3, found: 'her', text: 'Send her an Offer' }
		]);
	});

	it('flags "them" as the person a message goes to (#2198)', () => {
		const line = '<Text text="Invited. An email has been sent to them." />';

		expect(findOffensesInSource('fixture.svelte', line)).toEqual([
			{ file: 'fixture.svelte', line: 1, found: 'them', text: line }
		]);
		expect(findOffensesInSource('fixture.ts', "export const HINT = 'Remind them to sign';")).toHaveLength(1);
	});

	it('leaves "them" alone where it is a plural of things', () => {
		const source = [
			"export const A = 'Add, remove and reorder them.';",
			"export const B = 'They are shown once, when you ask for them.';",
			"export const C = 'This device is not receiving them.';"
		].join('\n');

		expect(findOffensesInSource('fixture.ts', source)).toEqual([]);
	});

	it('honors the marker with a reason on the same line', () => {
		const line = `export const TITLE = 'Her Birth, Her Way'; // ${MARKER}: a book's title`;

		expect(findOffensesInSource('fixture.ts', line)).toEqual([]);
	});

	it('does not honor a marker with no reason', () => {
		const line = `export const HINT = 'Send her an Offer'; // ${MARKER}`;

		expect(findOffensesInSource('fixture.ts', line)).toHaveLength(1);
	});

	it('reads a real file from disk the same way', () => {
		// Exercises the disk-reading wrapper directly, which the glob-driven
		// test above already covers incidentally -- this asserts it in
		// isolation with files the fixes landed clean.
		expect(findOffenses('src/lib/components/organisms/OfferSection.svelte')).toEqual([]);
		expect(findOffenses('src/lib/staff.ts')).toEqual([]);
	});
});
