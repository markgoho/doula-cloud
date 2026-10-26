import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { globFiles } from './globFiles';

/*
 * #1958's gate. `dates.ts` holds the one rule every screen that prints a
 * date has to get right -- an instant and a calendar day are different
 * kinds of value -- and prints both month-day, as ADR-0022's 2026-10-24
 * amendment (#1940) records. Eight places went around it with a bare
 * `toLocaleDateString()`: each printed "10/24/2026", and the one that
 * read a calendar day as UTC midnight showed a Payment a day early.
 *
 * This spec fails the build on a `toLocaleDateString`,
 * `toLocaleTimeString` or `toLocaleString` call anywhere in `app/src`
 * outside `dates.ts`. `toLocaleString` is in the net because on a Date it
 * prints a date too; on a number it does not, and a line that formats a
 * number that way takes the marker. `Intl.DateTimeFormat` is not: the
 * places that use it compute a value (the reader's zone, a Practice's
 * today as `YYYY-MM-DD`) rather than print one.
 *
 * Specs are out of the sweep: a spec builds the text it expects, it does
 * not print a date a person reads, and several build it the same way the
 * formatter does on purpose. A deliberate exception elsewhere stays by
 * putting `dates:ignore` and the reason on the line, the same shape as
 * `spelling:ignore`.
 */

/*
 * #2008's rule, beside it. A date of birth went around `dates.ts` with no
 * `toLocale*` call at all: the record and the search results printed the
 * stored value as it came, "1991-03-14", while intake printed "Mar 14,
 * 1991". A raw ISO day reaching the screen is a value, not a call, so no
 * source scan can find every one; what this one can find reliably is the
 * shape that happened. In a `.svelte` file, every read of `.dateOfBirth`
 * is one of three things: handed to `formatCalendarDay` (or `splitDate`,
 * which fills the three date boxes), tested for presence (`!x.dateOfBirth`,
 * `x.dateOfBirth ? … :`, `x.dateOfBirth && …`), or a print of the raw
 * value -- which this fails. It does not catch a calendar day under
 * another name, and it does not read a `.ts` file, where a draft carries
 * the stored value on to the BFF; the formatter the record and intake
 * share, `formatCalendarDay`, is the answer for both.
 */
const CHAIN = /[\w$]+(?:!?\??\.[\w$]+)*/g;
const READ = /!?\??\.dateOfBirth$/;
const FORMATTERS = ['formatCalendarDay(', 'splitDate('];
const NEGATED_BEFORE = /!\s*$/;
const TESTED_AFTER = /^\s*(?:\?(?![?.])|&&)/;

// Inside a call to one of FORMATTERS that has not closed yet, however deep
// -- `splitDate(untrack(() => draft.dateOfBirth))` is handed to it too.
function isInsideFormatter(before: string): boolean {
	const opened = Math.max(...FORMATTERS.map((call) => before.lastIndexOf(call)));
	if (opened === -1) return false;
	const call = [...before.slice(before.indexOf('(', opened))];
	let depth = 0;
	for (const character of call) {
		if (character === '(') depth += 1;
		else if (character === ')') depth -= 1;
		// The formatter's own call closed before the read.
		if (depth === 0) return false;
	}
	return true;
}

function findRawDateOfBirth(file: string, source: string): Offense[] {
	return source.split('\n').flatMap((text, index) => {
		if (text.includes(IGNORE_MARKER)) return [];
		const isRaw = text.matchAll(CHAIN).some((match) => {
			if (!READ.test(match[0])) return false;
			const before = text.slice(0, match.index);
			const after = text.slice(match.index + match[0].length);
			return !isInsideFormatter(before) && !NEGATED_BEFORE.test(before) && !TESTED_AFTER.test(after);
		});
		return isRaw ? [{ file, line: index + 1 }] : [];
	});
}

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const IGNORE_MARKER = 'dates:ignore';
const OWNER = 'src/lib/dates.ts';

const PRINTS_A_DATE = /\.toLocale(?:Date|Time)?String\s*\(/;

interface Offense {
	file: string;
	line: number;
}

function findDatePrinting(file: string, source: string): Offense[] {
	return source
		.split('\n')
		.flatMap((text, index) =>
			PRINTS_A_DATE.test(text) && !text.includes(IGNORE_MARKER) ? [{ file, line: index + 1 }] : []
		);
}

const sourceFiles = globFiles('src/**/*.{svelte,ts,js}', { cwd: appRoot }).filter(
	(file) => file !== OWNER && !/\.(?:test|spec)\.[jt]s$/.test(file)
);

// Read and scanned once, at module scope, for the reason
// spelling.usage.spec.ts gives (#1211).
const offenses = sourceFiles.flatMap((file) =>
	findDatePrinting(file, readFileSync(path.join(appRoot, file), 'utf8'))
);

const svelteFiles = sourceFiles.filter((file) => file.endsWith('.svelte'));
const rawDatesOfBirth = svelteFiles.flatMap((file) =>
	findRawDateOfBirth(file, readFileSync(path.join(appRoot, file), 'utf8'))
);

describe('a date of birth reaches the screen through dates.ts (#2008)', () => {
	it('reads the screens that print one', () => {
		expect(svelteFiles).toContain('src/routes/practices/[practiceId]/clients/[clientId]/+page.svelte');
		expect(svelteFiles).toContain('src/routes/practices/[practiceId]/clients/search/+page.svelte');
	});

	it('finds the raw value printed, in a row, in markup and behind a fallback', () => {
		expect(findRawDateOfBirth('a.svelte', "{ label: 'Date of birth', value: match.dateOfBirth || '—' },")).toEqual([
			{ file: 'a.svelte', line: 1 }
		]);
		expect(findRawDateOfBirth('a.svelte', '\n<dd>{detail!.dateOfBirth}</dd>')).toEqual([{ file: 'a.svelte', line: 2 }]);
		expect(findRawDateOfBirth('a.svelte', "x.dateOfBirth ? x.dateOfBirth : '—'")).toEqual([
			{ file: 'a.svelte', line: 1 }
		]);
		expect(findRawDateOfBirth('a.svelte', 'x.dateOfBirth ?? y')).toEqual([{ file: 'a.svelte', line: 1 }]);
	});

	it('leaves a formatted value, a filled date box, a presence test and a line that carries the marker', () => {
		expect(findRawDateOfBirth('a.svelte', "x.dateOfBirth ? formatCalendarDay(x.dateOfBirth) : '—'")).toEqual([]);
		expect(findRawDateOfBirth('a.svelte', 'value: d.answers.dateOfBirth && formatCalendarDay(d.answers.dateOfBirth)')).toEqual([]);
		expect(findRawDateOfBirth('a.svelte', 'parts = splitDate(typed.dateOfBirth);')).toEqual([]);
		expect(findRawDateOfBirth('a.svelte', 'splitDate(untrack(() => journey.draft.answers.dateOfBirth))')).toEqual([]);
		expect(findRawDateOfBirth('a.svelte', "formatCalendarDay(x.y) + (z.dateOfBirth || '')")).toEqual([
			{ file: 'a.svelte', line: 1 }
		]);
		expect(findRawDateOfBirth('a.svelte', 'if (!fields.name && !fields.dateOfBirth) {')).toEqual([]);
		expect(findRawDateOfBirth('a.svelte', '{x.dateOfBirth} <!-- ' + IGNORE_MARKER + ': a demo -->')).toEqual([]);
		expect(findRawDateOfBirth('a.svelte', 'let dateOfBirth = $derived(composed.value);')).toEqual([]);
	});

	it('finds no date of birth printed raw', () => {
		expect(
			rawDatesOfBirth.map(
				(offense) =>
					`${offense.file}:${offense.line} prints a date of birth as it is stored -- pass it through formatCalendarDay, or put ${IGNORE_MARKER} and a reason on the line`
			)
		).toEqual([]);
	});
});

describe('every date a screen prints goes through dates.ts (#1958)', () => {
	it('reads the whole app/src tree, and no spec', () => {
		// A glob that silently matched nothing would make the last
		// assertion pass while checking no source at all.
		expect(sourceFiles.some((file) => file.endsWith('.svelte'))).toBe(true);
		expect(sourceFiles).toContain('src/lib/invoice.ts');
		expect(sourceFiles).not.toContain(OWNER);
		expect(sourceFiles.some((file) => file.endsWith('.spec.ts'))).toBe(false);
	});

	it('finds each of the three calls, on a Date or not', () => {
		expect(findDatePrinting('a.ts', 'new Date(x).toLocaleDateString()')).toEqual([{ file: 'a.ts', line: 1 }]);
		expect(findDatePrinting('a.ts', '\n\tdate.toLocaleTimeString (undefined)')).toEqual([
			{ file: 'a.ts', line: 2 }
		]);
		expect(findDatePrinting('a.svelte', '{new Date(at).toLocaleString()}')).toEqual([
			{ file: 'a.svelte', line: 1 }
		]);
	});

	it('leaves a mention in prose and a line that carries the marker', () => {
		expect(findDatePrinting('a.ts', '// toLocaleDateString always returns a string')).toEqual([]);
		expect(findDatePrinting('a.ts', `n.toLocaleString(); // ${IGNORE_MARKER}: a number`)).toEqual([]);
	});

	it('finds no date printed around dates.ts', () => {
		expect(
			offenses.map(
				(offense) =>
					`${offense.file}:${offense.line} prints a date around dates.ts -- use formatInstant, formatCalendarDay or another dates.ts formatter, or put ${IGNORE_MARKER} and a reason on the line`
			)
		).toEqual([]);
	});
});
