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
 *   offence   colour   behaviour   centre   recognise    spelling:ignore: the
 *   one place the forbidden words are named, so the rule can be read
 *
 * #921 swept a second wave. A few of those words collide with something
 * this repo, or someone else, already owns -- an ARIA attribute name
 * (`aria-labelledby`), a CSS color keyword, this repo's own `organism`
 * design-system tier, a third party's own document title or glossary term
 * -- so a hit is a per-occurrence judgment call rather than an automatic
 * rewrite, marked `spelling:ignore` plus the reason on the line it stays.
 * None of RULES below encodes that judgment; the rule only says a spelling
 * exists, same as the first five.
 * #921 also widened the sweep from `app/src` alone to `api/`'s Go source,
 * since a spelling rule that only watched half the repo was gating nothing
 * for the other half.
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
const repoRoot = path.join(appRoot, '..');

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
	{ british: 'cent\u{72}e', american: 'center' },
	// #768 wrote "ageing" of an Invoice past its due date and the sweep had
	// nothing to say, because this list is the rule's whole reach: a
	// spelling absent from it is unenforced, not allowed. Added with the
	// word that found it rather than left for the next one.
	{ british: 'ag\u{65}ing', american: 'aging' },
	// #921's second wave. "cancelled"/"cancelling" are two rules, not one,
	// because neither is a substring of "cancellation" -- which keeps its
	// double L in American English too (Merriam-Webster), so it was never
	// part of this family and needed no rule at all.
	{ british: 'cance\u{6C}led', american: 'canceled' },
	{ british: 'cance\u{6C}ling', american: 'canceling' },
	// Matches "labelled", "unlabelled" and "relabelled" as one substring,
	// and also matches inside "aria-labelledby" -- deliberately, since that
	// ARIA attribute name needs its own spelling:ignore on every line too.
	{ british: 'label\u{6C}ed', american: 'labeled' },
	{ british: 'label\u{6C}ing', american: 'labeling' },
	{ british: 'licen\u{63}e', american: 'license' },
	{ british: 'catalog\u{75}e', american: 'catalog' },
	{ british: 'art\u{65}fact', american: 'artifact' },
	// Also catches "greys", "greyed" and "greylisting" as one substring.
	// "grey" is a real CSS color keyword too, so a hit inside a CSS value
	// stays with spelling:ignore rather than becoming "gray".
	{ british: 'gr\u{65}y', american: 'gray' },
	{ british: 'judg\u{65}ment', american: 'judgment' },
	{ british: '\u{65}nquiry', american: 'inquiry' },
	{ british: 'def\u{65}nce', american: 'defense' },
	{ british: 'customi\u{73}', american: 'customize' },
	// "organis\u{65}" also matches "organised", but not "organism" or
	// "organisation" -- neither has an "e" right after "organis". This
	// repo's own `organism`/`organisms`/`organismPages` design-system tier
	// is correct English and was never part of this family.
	{ british: 'organis\u{65}', american: 'organize' },
	{ british: 'organis\u{61}tion', american: 'organization' },
	{ british: 'normali\u{73}ed', american: 'normalized' },
	// No rule for "tyres" -> "tires": the plain substring collides with
	// ordinary camelCase ("activityResponse" lowercases to a string
	// containing "...tyres..." at the "ty"/"Res" seam), so it fails this
	// file's own no-homograph requirement. Its two real occurrences
	// (both prose, both outside app/src and api/) were fixed by hand.
	{ british: 'seriali\u{73}es', american: 'serializes' },
	{ british: 'model\u{6C}ing', american: 'modeling' },
	{ british: 'favo\u{75}rite', american: 'favorite' },
	{ british: 'emphasi\u{73}e', american: 'emphasize' },
	{ british: 'summari\u{73}es', american: 'summarizes' },
	{ british: 'reali\u{73}ed', american: 'realized' },
	// No rule for "programme" -> "program": the plain substring collides
	// with "programmer"/"programmers", an ordinary, dialect-invariant
	// English word -- the same failure mode as "tyres" above. Its one real
	// occurrence (prose, outside app/src and api/) was fixed by hand.
	{ british: 'prioriti\u{73}ing', american: 'prioritizing' },
	// American English doubles the L here, the reverse of most of this
	// list's -ed/-ing pairs: "fulfillment", not "fulfilment".
	{ british: 'fulfi\u{6C}ment', american: 'fulfillment' }
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

// #921: api/'s Go source had no gate at all before this -- every British
// spelling there was invisible to this file's RULES list no matter what
// words it held. Paths come back already prefixed "api/", since the glob
// runs from repoRoot rather than appRoot.
const apiFiles = globSync('api/**/*.go', { cwd: repoRoot });

describe('app/src and api/ spell every word the American way', () => {
	it('reads the whole app source tree', () => {
		// A glob that silently matched nothing would make the assertion
		// below pass while checking no source at all.
		expect(sourceFiles.length).toBeGreaterThan(100);
	});

	it('reads the whole api Go source tree', () => {
		expect(apiFiles.length).toBeGreaterThan(100);
	});

	it('finds no British spelling of a word the repo already spells one way', () => {
		const offenses = [
			...sourceFiles.flatMap((file) =>
				findOffensesInLines(file, readFileSync(path.join(appRoot, file), 'utf8'))
			),
			...apiFiles.flatMap((file) =>
				findOffensesInLines(file, readFileSync(path.join(repoRoot, file), 'utf8'))
			)
		];

		expect(
			offenses.map(
				(offense) =>
					`${offense.file}:${offense.line} contains "${offense.found}" -- write it as in "${offense.american}", or put ${IGNORE_MARKER} and a reason on the line`
			)
		).toEqual([]);
	});
});
