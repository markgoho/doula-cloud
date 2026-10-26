import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { globFiles } from './globFiles';
import { copyCandidates, NOT_COPY } from './quotedCopy';

/*
 * #2023: two hyphens are not a dash on a screen. Every other line a person
 * reads uses an em dash (—) or two sentences, and a ` -- ` left in rendered
 * copy reads as a typing slip. This repo's comments and documents use ` -- `
 * freely (that is the documented voice, `CLAUDE.md`), so the check reads
 * rendered copy only: markup text and quoted strings in a component or
 * route, and quoted strings in a `.ts` module that a screen imports, with
 * every comment stripped first by `quotedCopy.ts`.
 *
 * A string that has to keep two hyphens -- a developer-facing error that is
 * never rendered, a CLI flag -- carries `voice:ignore` and the reason on the
 * same line, the marker the other two copy specs read.
 *
 * ## What it deliberately does not catch
 *
 * 1. **A lone hyphen used as a dash** ("Expired - invite again") is not
 *    checked: a hyphen between spaces is also how a range or a minus sign
 *    is typed, and telling them apart needs a reader, not a regex.
 * 2. **A string built from parts across lines** is read line by line, so
 *    `'Expired -' + '- invite'` would pass. Nothing in the app builds copy
 *    that way today.
 *
 * The Staff ledger's half of #2023 ("no action code reaches a screen") is
 * not here: `staffActivityPhrases.usage.spec.ts` (#2006) already fails the
 * build on an action with no Staff phrase, and `staffActivityPhrase` throws
 * rather than print one.
 */

const MARKER = 'voice:ignore';

// A marker only counts with a reason after it.
const MARKER_WITH_REASON = new RegExp(String.raw`${MARKER}\W*\w`);

// Two hyphens with a space (or the line's edge) either side: a dash typed
// as hyphens, including one that opens or closes a wrapped line of prose.
// `--token` (a CSS custom property) and `a--b` (a BEM modifier) touch a
// word on one side, so neither is a dash.
const DOUBLE_HYPHEN = /(?:^|\s)--(?:\s|$)/;

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const GLOBS = ['src/{lib/components,routes}/**/*.svelte', 'src/{lib,routes}/**/*.ts'];

interface Offense {
	file: string;
	line: number;
	text: string;
}

function findOffensesInSource(file: string, source: string): Offense[] {
	const raw = source.split('\n');
	return copyCandidates(file, source)
		.filter(({ line, text }) => DOUBLE_HYPHEN.test(text) && !MARKER_WITH_REASON.test(raw[line - 1]!))
		.map(({ line, text }) => ({ file, line, text }));
}

// Read and scanned once, at module scope, so the cost of the whole-tree
// read is paid on import rather than charged against one `it`'s timeout
// (#1211).
const files = globFiles(GLOBS, { cwd: appRoot }).filter((file) => !NOT_COPY.test(file));

const offenses = files.flatMap((file) =>
	findOffensesInSource(file, readFileSync(new URL(file, `file://${appRoot}`), 'utf8'))
);

describe('rendered copy uses a dash, never two hyphens', () => {
	it('reads every component, route and copy module', () => {
		// A glob that silently matched nothing would make the assertion
		// below pass while checking no copy at all.
		expect(files.filter((file) => file.endsWith('.svelte')).length).toBeGreaterThan(50);
		expect(files.filter((file) => file.endsWith('.ts')).length).toBeGreaterThan(50);
	});

	it('finds no " -- " anywhere a person reads', () => {
		expect(
			offenses.map(
				(offense) =>
					`${offense.file}:${offense.line} "${offense.text.trim()}": write an em dash (—) or two sentences. ` +
					`If the hyphens have to stay, put ${MARKER} and the reason on the line.`
			)
		).toEqual([]);
	});
});

describe('findOffensesInSource', () => {
	it('flags two hyphens in markup text', () => {
		expect(findOffensesInSource('fixture.svelte', '<p>Expired -- invite again</p>')).toEqual([
			{ file: 'fixture.svelte', line: 1, text: '<p>Expired -- invite again</p>' }
		]);
	});

	it('flags two hyphens that open a wrapped line of prose', () => {
		const source = ['<p>', '\t-- a URL somebody pasted', '</p>'].join('\n');

		expect(findOffensesInSource('fixture.svelte', source).map((offense) => offense.line)).toEqual([2]);
	});

	it('flags a quoted string in a <script> block, not the code around it', () => {
		const source = ['<script>', 'let i = 0; i --;', "const label = 'Archived -- gone';", '</script>'].join('\n');

		expect(findOffensesInSource('fixture.svelte', source).map((offense) => offense.text)).toEqual([
			'Archived -- gone'
		]);
	});

	it('flags a quoted string in a .ts module', () => {
		const source = ['const n = 1;', 'export const A = `Not chosen yet -- ask later`;'].join('\n');

		expect(findOffensesInSource('fixture.ts', source).map((offense) => offense.line)).toEqual([2]);
	});

	it('ignores two hyphens in a comment, a <style> line and a custom property', () => {
		const source = [
			'<!-- a comment -- in markup -->',
			'<p style="--gap: 1rem">ok</p>',
			'<script>',
			'// a comment -- in script',
			'</script>',
			'<style>',
			'p { margin: var(--space-1); } .a -- b {}',
			'</style>'
		].join('\n');

		expect(findOffensesInSource('fixture.svelte', source)).toEqual([]);
	});

	it('honors the marker with a reason on the same line', () => {
		const line = `<p>Run it with -- after the name</p> <!-- ${MARKER}: a CLI separator -->`;

		expect(findOffensesInSource('fixture.svelte', line)).toEqual([]);
	});

	it('does not honor a marker with no reason', () => {
		const line = `<p>Expired -- invite again</p> <!-- ${MARKER} -->`;

		expect(findOffensesInSource('fixture.svelte', line)).toHaveLength(1);
	});
});
