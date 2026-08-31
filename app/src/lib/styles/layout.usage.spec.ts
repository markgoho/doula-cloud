import { readFileSync, globSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { styleLines } from './styleLines';

/*
 * The intrinsic-layout source gate -- ADR-0025, checking ADR-0024's
 * commitment.
 *
 * ADR-0024's commitment is that a component adapts to the space it is
 * *given*, never to a device it assumes it is on, and that every screen is
 * complete and usable from 320px up. Most of that is proved by rendering,
 * across the continuum rather than at a set of widths: the drag surface at
 * `/style-guide/drag-surface` puts a component in a frame a person resizes
 * continuously, and the continuum check asserts against it.
 *
 * This file holds the half a rendering test cannot honestly see. A render
 * shows how a component *behaves*; it can never show which mechanism the
 * source reached for, so nothing in a rendering test stops `@media
 * (min-width: ...)` from quietly accreting. That is the drift this gate
 * exists to stop, and it is why the checks here are about *mechanism*
 * rather than about outcome -- a source rule holds at every available
 * space precisely because it never measures one.
 *
 * Deliberately only two rules. Fixed pixel widths, bare `1fr` grid tracks
 * and `white-space: nowrap` all have legitimate uses, so a source check on
 * them fires on correct code until somebody suppresses it, and a gate that
 * is routinely suppressed has stopped being a gate. These two are
 * different: a width media query is wrong by ADR-0024 rule 3 every time it
 * appears, and `100vw` is wrong every time it appears.
 *
 * ## The escape hatch, and the rule that does not have one
 *
 * `layout:ignore` with a reason, scoped to the rule block it introduces,
 * exactly as `tokens:ignore` and `motion:ignore` already work in this
 * repo, excuses the `vw` rule -- a genuine case could exist.
 *
 * It does not excuse a width media query, and this is deliberate rather
 * than an oversight to correct. ADR-0024 rule 3 is that a media query is
 * for a stated user preference and never for space, so a marker on a width
 * query is a request to do the one thing the mechanism ADR forbids; there
 * is no reason that could be written in it that the ADR has not already
 * refused. If a real wall turns up, the answer is a decision recorded on
 * #518, not a marker in a file. The predecessor allowed the marker on two
 * allowlisted shell files (`SHELL_CHROME`); both now declare their own
 * containment context and query it, so the allowlist has nothing left to
 * hold and is gone with the exception it encoded.
 */

const IGNORE = 'layout:ignore';

/*
 * No marker string can match this, so `styleLines` filters nothing and
 * returns every style line in the file -- including the ones an in-force
 * `layout:ignore` would otherwise have removed. That is what makes rule 1
 * unconditional: it reads the unfiltered pass and never asks whether a
 * line claimed an exception.
 */
const NO_MARKER = String.fromCodePoint(0);

const appRoot = new URL('../../../', import.meta.url);
const cwd = fileURLToPath(appRoot);

/*
 * Every CSS-bearing file the app ships, which is the widening ADR-0025
 * asks for. The predecessor scanned `src/lib/components` and `src/routes`
 * alone, so the token layer -- one plain stylesheet holding a value every
 * page reads -- sat in the one place the gate did not look, and its static
 * `vw` was found by hand instead (#532). A gate with a blind spot the size
 * of the token layer reports a clean repo that is not clean.
 */
const svelteFiles = globSync('src/{lib/components,routes}/**/*.svelte', { cwd });
const cssFiles = globSync('src/**/*.css', { cwd });

interface Offence {
	file: string;
	line: number;
	found: string;
}

/*
 * A width media query, and only a width one. `prefers-color-scheme`,
 * `prefers-reduced-motion`, `prefers-contrast` and `print` state a user
 * preference rather than a device size, have no sizing substitute at all,
 * and ADR-0024 keeps them untouched.
 *
 * Matching `@media` rather than `@container` is also what satisfies the
 * ADR's requirement that the gate can tell the two apart: a container
 * query asks about the space a component was given and is the default
 * mechanism, so it is never an offence here however many of them there
 * are.
 */
const WIDTH_MEDIA = /@media[^{]*\bwidth\b/;

/*
 * The static viewport width unit measures the window including the strip
 * the scrollbar occupies, so an element sized with it is always wider than
 * the space actually available and the page scrolls sideways by exactly
 * the scrollbar's width. `100%`, `100dvw` and `100svw` are the values that
 * mean what the author intended.
 *
 * The number is part of the match so `100dvw` does not read as a hit: the
 * digits are followed by `d`, not by `vw`.
 */
const STATIC_VW = /\d[\d.]*vw\b/;

interface FileOffences {
	widthMedia: Offence[];
	staticVw: Offence[];
}

/*
 * Two passes over the same source. The unfiltered one is what rule 1
 * judges, so no marker can reach it. The filtered one is what an in-force
 * `layout:ignore` has already survived, and it is what rule 2 judges.
 */
function scanSource(file: string, source: string, kind: 'svelte' | 'css'): FileOffences {
	return {
		widthMedia: styleLines(source, NO_MARKER, kind)
			.filter(({ text }) => WIDTH_MEDIA.test(text))
			.map(({ line, text }) => ({ file, line, found: text.trim() })),
		staticVw: styleLines(source, IGNORE, kind)
			.filter(({ text }) => STATIC_VW.test(text))
			.map(({ line, text }) => ({ file, line, found: text.trim() }))
	};
}

function scanFile(file: string, kind: 'svelte' | 'css'): FileOffences {
	return scanSource(file, readFileSync(new URL(file, appRoot), 'utf8'), kind);
}

function scan(): FileOffences {
	const perFile = [
		...svelteFiles.map((file) => scanFile(file, 'svelte')),
		...cssFiles.map((file) => scanFile(file, 'css'))
	];
	return {
		widthMedia: perFile.flatMap(({ widthMedia }) => widthMedia),
		staticVw: perFile.flatMap(({ staticVw }) => staticVw)
	};
}

const { widthMedia, staticVw } = scan();

function report(offences: Offence[]): string[] {
	return offences.map(({ file, line, found }) => `${file}:${line} -- ${found}`);
}

describe('intrinsic layout (ADR-0024, gated per ADR-0025)', () => {
	it('reaches for a container query, and never for the window', () => {
		expect(report(widthMedia)).toEqual([]);
	});

	it('never sizes on the static viewport width unit', () => {
		expect(report(staticVw)).toEqual([]);
	});

	it('scans the token layer, not only the components', () => {
		/* The blind spot #532 fell into. A glob that silently stopped
		   matching would leave both rules passing over nothing. */
		expect(cssFiles).toContain('src/lib/styles/tokens.css');
		expect(svelteFiles.length).toBeGreaterThan(100);
	});
});

/*
 * The rules stated against sources written here rather than found in the
 * tree. The tree proves the repo is clean today; these prove the gate
 * would still catch a width query tomorrow, and that it leaves a
 * preference query alone.
 */
function wrap(css: string): string {
	return `<div class="x"></div>\n<style>\n${css}\n</style>\n`;
}

describe('what the gate judges', () => {
	it('fails a width media query', () => {
		const { widthMedia: found } = scanSource(
			'x.svelte',
			wrap('@media (min-width: 60rem) {\n.x { display: flex; }\n}'),
			'svelte'
		);
		expect(report(found)).toEqual(['x.svelte:3 -- @media (min-width: 60rem) {']);
	});

	it('fails a width media query that claims layout:ignore', () => {
		/* ADR-0024 rule 3 leaves no legitimate exception, so the marker
		   buys nothing here. A future author proposing one should read the
		   escape-hatch note at the top of this file first. */
		const { widthMedia: found } = scanSource(
			'x.svelte',
			wrap(`/* ${IGNORE} -- a reason the ADR has already refused */\n@media (max-width: 40rem) {\n.x { display: none; }\n}`),
			'svelte'
		);
		expect(report(found)).toEqual(['x.svelte:4 -- @media (max-width: 40rem) {']);
	});

	it('leaves a preference query and print alone', () => {
		const { widthMedia: found } = scanSource(
			'x.svelte',
			wrap(
				'@media (prefers-color-scheme: dark) {\n.x { color: white; }\n}\n' +
					'@media (prefers-reduced-motion: no-preference) {\n.x { transition: 1s; }\n}\n' +
					'@media (prefers-contrast: more) {\n.x { color: black; }\n}\n' +
					'@media print {\n.x { display: none; }\n}'
			),
			'svelte'
		);
		expect(report(found)).toEqual([]);
	});

	it('never counts a container query, however many there are', () => {
		const { widthMedia: found } = scanSource(
			'x.svelte',
			wrap('@container staff-top-bar (min-width: 60rem) {\n.x { display: flex; }\n}'),
			'svelte'
		);
		expect(report(found)).toEqual([]);
	});

	it('fails a static vw, and excuses one that says why', () => {
		const offending = scanSource('x.css', '.x { inline-size: 100vw; }', 'css');
		expect(report(offending.staticVw)).toEqual(['x.css:1 -- .x { inline-size: 100vw; }']);

		const excused = scanSource(
			'x.css',
			`/* ${IGNORE} -- a genuine case */\n.x {\n\tinline-size: 100vw;\n}`,
			'css'
		);
		expect(report(excused.staticVw)).toEqual([]);
	});

	it('reads a plain stylesheet, which has no style block to open', () => {
		const { widthMedia: found } = scanSource('x.css', '@media (min-width: 60rem) {\n}', 'css');
		expect(report(found)).toEqual(['x.css:1 -- @media (min-width: 60rem) {']);
	});
});
