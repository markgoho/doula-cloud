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
