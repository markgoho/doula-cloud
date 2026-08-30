import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { globSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { styleLines } from './styleLines';

/*
 * The other half of the token contract.
 *
 * `tokens.spec.ts` proves the values in `tokens.css` are sound -- the
 * contrast floors, the OKLCH-only rule, the two dark blocks staying in
 * sync. It says nothing about whether anything *uses* them, so a component
 * could carry `34px` and `17px` lifted straight off a canvas and the whole
 * suite stayed green. That is exactly what happened while building the
 * shell (#452): five hard-coded lengths in four new components, caught by
 * a person reading the diff rather than by anything here.
 *
 * So this walks every component and route and fails on a raw value where
 * a token is the answer. It runs in the unit suite, which
 * `scripts/hooks/pre-commit` already runs in full, so it blocks a commit
 * rather than warning about one -- the same shape as #418's motion gate,
 * and for the same reason: a commitment nobody measures decays.
 *
 * ## The escape hatch
 *
 * Some raw lengths are not design values at all -- the WCAG clip
 * technique's `1px`, a drawn ring's own stroke. Those are marked with a
 * `tokens:ignore` comment carrying a reason, the way `motion:ignore` and
 * `coverage:ignore` already work in this repo.
 *
 * The marker covers the rule block it introduces, and nothing beyond it.
 * Not the whole file, because a file-level opt-out is how a gate stops
 * meaning anything; and not a single line, because the exceptions that
 * exist in practice are whole idioms -- three declarations of the clip
 * technique, not one -- and a reason repeated three times is a reason
 * nobody reads.
 */

const IGNORE = 'tokens:ignore';

const componentFiles = globSync('src/{lib/components,routes}/**/*.svelte', {
	cwd: fileURLToPath(new URL('../../../', import.meta.url))
});

const appRoot = new URL('../../../', import.meta.url);

interface Offence {
	file: string;
	line: number;
	text: string;
	found: string;
}

/*
 * Declaration-level rather than pattern-level, because a negative
 * lookahead after `\s*` backtracks onto the whitespace and matches
 * everything -- which is how the first draft of this file reported every
 * correct `font-size: var(--text-body-size)` in the app as an offence.
 */
function scanDeclarations(properties: string[], isAllowed: (value: string) => boolean): Offence[] {
	const declaration = new RegExp(String.raw`(${properties.join('|')})\s*:\s*([^;]+)`, 'g');
	const offences: Offence[] = [];
	for (const file of componentFiles) {
		const source = readFileSync(new URL(file, appRoot), 'utf8');
		for (const { line, text } of styleLines(source, IGNORE)) {
			const breaches = text
				.matchAll(declaration)
				.filter((match) => !isAllowed(match[2]!.trim()))
				.toArray();
			for (const match of breaches) {
				offences.push({ file, line, text: text.trim(), found: match[0].trim() });
			}
		}
	}
	return offences;
}

// `styleLines` has already dropped everything a `tokens:ignore` covers, so
// neither scanner has to think about the marker.
function scan(pattern: RegExp): Offence[] {
	const offences: Offence[] = [];
	for (const file of componentFiles) {
		const source = readFileSync(new URL(file, appRoot), 'utf8');
		for (const { line, text } of styleLines(source, IGNORE)) {
			const match = pattern.exec(text);
			pattern.lastIndex = 0;
			if (match !== null) offences.push({ file, line, text: text.trim(), found: match[0] });
		}
	}
	return offences;
}

function report(offences: Offence[]): string[] {
	return offences.map(
		(offence) => `${offence.file}:${offence.line}  ${offence.found}  --  ${offence.text}`
	);
}

describe('components spend tokens, not raw values', () => {
	it('finds components to check at all, so a broken glob cannot pass silently', () => {
		expect(componentFiles.length).toBeGreaterThan(40);
	});

	/*
	 * Every length in the design system has a token: spacing on the 4px
	 * scale, the shell's own dimensions, the page frame, the two border
	 * weights, the focus ring. A raw px is therefore either a token that
	 * has not been declared yet or a canvas value that was never a
	 * decision -- both worth stopping at.
	 */
	it('uses no raw px length', () => {
		const offences = scan(/(?<![\w-])-?\d*\.?\d+px/g);

		expect(report(offences)).toEqual([]);
	});

	/*
	 * Colour is the one axis where a raw value is never defensible: the
	 * whole point of authoring in OKLCH in one file is that the dark theme
	 * is a derivation rather than a second hand-tuned palette, and a hex
	 * in a component opts that component out of the dark theme entirely.
	 */
	it('names no colour of its own', () => {
		const offences = scan(/#[0-9a-fA-F]{3,8}(?![\w-])|\brgba?\(|\bhsla?\(|\boklch\(/g);

		expect(report(offences)).toEqual([]);
	});

	/*
	 * The type scale is closed by design (#417): a route names a purpose
	 * and never reaches for a size. A literal font-size is how a ninth step
	 * gets into an eight-step scale without anybody deciding to add one --
	 * which is exactly how the shell's wordmark arrived at 17px.
	 */
	it('sets no font-size, font-weight or letter-spacing outside the scale', () => {
		const offences = scanDeclarations(
			['font-size', 'font-weight', 'letter-spacing'],
			(value) => value.startsWith('var(--') || value === 'inherit' || value === 'normal'
		);

		expect(report(offences)).toEqual([]);
	});
});

/*
 * The favicon is the one asset that legitimately carries colour literals:
 * it is rendered outside the document, so it can reach no custom property.
 * That makes it the one place a value can drift away from the palette
 * without anything noticing -- which is why it is still authored in OKLCH,
 * and why every value in it has to be one that tokens.css actually
 * declares.
 */
describe('the favicon holds no colour tokens.css does not', () => {
	const favicon = readFileSync(new URL('src/lib/assets/favicon.svg', appRoot), 'utf8');
	const tokens = readFileSync(new URL('src/lib/styles/tokens.css', appRoot), 'utf8');
	const colors = favicon
		.matchAll(/oklch\([^)]+\)/g)
		.map((match) => match[0])
		.toArray();

	it('draws the mark in both themes', () => {
		// Three arcs, light and dark.
		expect(colors).toHaveLength(6);
	});

	it.each(colors)('spends %s, which tokens.css declares', (color) => {
		expect(tokens).toContain(color);
	});

	it('names no colour outside OKLCH', () => {
		// Comments stripped first: an issue reference like #452 is three hex
		// digits followed by a non-word character, which is a colour as far
		// as a regex is concerned.
		const markup = favicon.replaceAll(/<!--.*?-->/gs, '');

		expect(markup).not.toMatch(/#[0-9a-fA-F]{3,8}(?![\w-])|\brgba?\(|\bhsla?\(/);
	});
});
