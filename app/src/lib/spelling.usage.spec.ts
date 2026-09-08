import { globSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * #899's fifth AC as a gate, in the mold of `roles.usage.spec.ts` (#262)
 * and `formErrors.usage.spec.ts` (#467): the repo writes American English
 * to the reader, so `app/src` must not grow a second spelling of a word it
 * already spells one way.
 *
 * The check is lexical and deliberately narrow -- only the word families
 * #899 actually found in the tree, each one a substring with no American
 * homograph, so a match is never a judgment call:
 *
 *   offence   colour   behaviour   centre   recognise
 *
 * Each pattern hides one of its letters behind a unicode escape rather than
 * spelling the word out, for two reasons: the file would otherwise fail its
 * own check, and a grep of the repo for a British form should return real
 * occurrences rather than this spec's own vocabulary.
 *
 * A deliberate exception -- an external contract, a verbatim quotation, a
 * third-party identifier -- stays by putting `spelling:ignore` plus the
 * reason on the same line, the same shape as `motion:ignore` in
 * `motion.spec.ts` and `coverage:ignore` in `api/`. The marker is the
 * comment #899's fourth AC asks every surviving exception to carry.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const IGNORE_MARKER = 'spelling:ignore';

interface Rule {
	// The British substring, one letter escaped so this file never holds it.
	readonly british: string;
	// What to write instead.
	readonly american: string;
}

const RULES: readonly Rule[] = [
	{ british: '\u{66}fence', american: 'offense' },
	{ british: 'olo\u{75}r', american: 'color' },
	{ british: 'ehavio\u{75}r', american: 'behavior' },
	{ british: 'ecogni\u{73}', american: 'recognize' },
	{ british: 'cent\u{72}e', american: 'center' }
];

interface Offense {
	readonly file: string;
	readonly line: number;
	readonly found: string;
	readonly american: string;
}

function findOffensesInLines(file: string, source: string): Offense[] {
	return source.split('\n').flatMap((line, index) => {
		if (line.includes(IGNORE_MARKER)) return [];
		const lower = line.toLowerCase();
		return RULES.filter((rule) => lower.includes(rule.british)).map((rule) => ({
			file,
			line: index + 1,
			found: rule.british,
			american: rule.american
		}));
	});
}

const sourceFiles = globSync('src/**/*.{svelte,ts,js,css,svg,html,md}', { cwd: appRoot }).filter(
	(file) => file !== 'src/lib/spelling.usage.spec.ts'
);

describe('app/src spells every word the American way', () => {
	it('reads the whole app source tree', () => {
		// A glob that silently matched nothing would make the assertion
		// below pass while checking no source at all.
		expect(sourceFiles.length).toBeGreaterThan(100);
	});

	it('finds no British spelling of a word the repo already spells one way', () => {
		const offenses = sourceFiles.flatMap((file) =>
			findOffensesInLines(file, readFileSync(path.join(appRoot, file), 'utf8'))
		);

		expect(
			offenses.map(
				(offense) =>
					`${offense.file}:${offense.line} contains "${offense.found}" -- write it as in "${offense.american}", or put ${IGNORE_MARKER} and a reason on the line`
			)
		).toEqual([]);
	});
});
