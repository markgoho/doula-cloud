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
 * #1154 was a third wave -- "double-barrelled" -- and the reason there was
 * a third is the shape of RULES rather than any word missing from it. RULES
 * is a denylist of exact spellings, so, as the `ageing` note below puts it,
 * "a spelling absent from it is unenforced, not allowed": it can only ever
 * hold words somebody already found by eye, which is how #921 followed #899
 * and #1154 followed both. `model\u{6C}ing` is the proof -- #921 added the
 * -ing form it happened to meet and left the -ed form of the same word to
 * be found three waves later.
 *
 * FAMILY below is the other shape, and it is an allowlist. British English
 * doubles a final `l` before `-ed`/`-ing` whatever the stress; American
 * English doubles it only when the last syllable is stressed ("compelled")
 * or when the base word already ends in `ll` ("spelled"). So every
 * `-alled`/`-elled`/`-alling`/`-elling` word is an offense unless it is
 * named in KEEPS_ITS_DOUBLE_L, and the fix is computed by dropping one `l`
 * rather than written down per word. A fourth wave of this family cannot
 * arrive quietly: an unfamiliar word turns the gate red and is either
 * corrected or added to the allowlist with a reason.
 *
 * The other vowels are deliberately not in FAMILY. `-illed`, `-olled` and
 * `-ulled` are almost entirely base words that already end in `ll`
 * ("filled", "rolled", "pulled") or are stressed ("controlled"), so their
 * allowlist would be open-ended while the tree holds no British form of
 * them at all; they stay in reach of RULES, one word at a time, if one ever
 * shows up.
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

// One word of the `-alled`/`-elled`/`-alling`/`-elling` family, matched
// whole rather than as a substring: "modelled" and "compelling" differ by
// name, not by any substring one of them holds and the other does not.
const FAMILY = /\b[a-z]+[ae]l\u{6C}(?:ed|ing)\b/gu;

// Every word that reaches this family the American way. Two groups: a base
// word that already ends in `ll` ("spell", "sell", "install", "eyeball"),
// and a base word ending in one `l` whose last syllable is stressed
// ("compel", "excel", "corral"). Anything else with two `l`s here is
// British, so an addition to this list is a claim about one of those two
// groups and belongs beside a word that is really in one of them.
const KEEPS_ITS_DOUBLE_L: ReadonlySet<string> = new Set([
	// Base word ends in `ll`.
	'belled',
	'belling',
	'dwelled',
	'dwelling',
	'felled',
	'felling',
	'gelled',
	'gelling',
	'jelled',
	'jelling',
	'misspelled',
	'misspelling',
	'quelled',
	'quelling',
	'shelled',
	'shelling',
	'smelled',
	'smelling',
	'spelled',
	'spelling',
	'swelled',
	'swelling',
	'welled',
	'welling',
	'yelled',
	'yelling',
	'bestselling',
	'foretelling',
	'outselling',
	'reselling',
	'retelling',
	'selling',
	'storytelling',
	'telling',
	'underselling',
	'upselling',
	'appalled',
	'appalling',
	'balled',
	'balling',
	'befalling',
	'called',
	'calling',
	'enthralled',
	'enthralling',
	'eyeballed',
	'eyeballing',
	'falling',
	'forestalled',
	'forestalling',
	'galled',
	'galling',
	'installed',
	'installing',
	'miscalled',
	'preinstalled',
	'recalled',
	'recalling',
	'reinstalled',
	'reinstalling',
	'snowballed',
	'snowballing',
	'stalled',
	'stalling',
	'stonewalled',
	'stonewalling',
	'uninstalled',
	'uninstalling',
	'walled',
	'walling',
	// Base word ends in one `l` on a stressed syllable, which American
	// English doubles as well.
	'compelled',
	'compelling',
	'corralled',
	'corralling',
	'dispelled',
	'dispelling',
	'excelled',
	'excelling',
	'expelled',
	'expelling',
	'impelled',
	'impelling',
	'propelled',
	'propelling',
	'rebelled',
	'rebelling',
	'repelled',
	'repelling'
]);

// A word of this family hides inside an identifier as often as it sits in
// prose, and `\b` does not fall between `is` and `Cancelled` or either side
// of the `_` in a Go tag. Split those seams before the match so
// `isCancelledFlag` and `total_cancelled` read as words.
function asWords(line: string): string {
	return line
		.replace(/([a-z0-9])([A-Z])/gu, '$1 $2')
		.replaceAll('_', ' ')
		.toLowerCase();
}

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
		const named = RULES.filter((rule) => lower.includes(rule.british)).map((rule) => ({
			file,
			line: index + 1,
			found: rule.british,
			american: rule.american
		}));
		const family = [...asWords(line).matchAll(FAMILY)]
			.map(([word]) => word)
			.filter((word) => !KEEPS_ITS_DOUBLE_L.has(word))
			.map((word) => ({
				file,
				line: index + 1,
				found: word,
				american: word.replace(/l(l(?:ed|ing))$/u, '$1')
			}));
		return [...named, ...family];
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

	// The family rule is the one rule here that can be wrong in both
	// directions -- it can miss a British word, and, unlike the substring
	// RULES, it can also fire on an American one. Both directions are
	// asserted against fixture lines rather than against the tree, so the
	// day the tree happens to hold none of these words the guarantee is
	// still checked.
	it('reads a doubled L as British only when American English keeps one', () => {
		const american = 'const spelling = compelling ? storytelling : dwelling; // installed, recalled';
		expect(findOffensesInLines('fixture.ts', american)).toEqual([]);

		const british = [
			`a double-barrel\u{6C}ed name`,
			`mode\u{6C}led on the row above`,
			`he trave\u{6C}led`,
			`const isCance\u{6C}ledFlag = true;`,
			`total_signa\u{6C}led`
		].join('\n');
		expect(findOffensesInLines('fixture.ts', british).map((offense) => offense.american)).toEqual([
			'barreled',
			'modeled',
			'traveled',
			// The named `cance\u{6C}led` rule and the family both see this
			// one; the family is the half that reads it inside camelCase.
			'canceled',
			'canceled',
			'signaled'
		]);

		expect(findOffensesInLines('fixture.ts', `a mode\u{6C}led row ${IGNORE_MARKER}: fixture`)).toEqual(
			[]
		);
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
