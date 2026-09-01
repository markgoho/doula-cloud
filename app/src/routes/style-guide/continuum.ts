/*
 * The instrument the continuum check sweeps with (CONTEXT.md, ADR-0025),
 * pulled out of `continuum.svelte.spec.ts` so the layout exercise can be
 * marked by the same sweep rather than by a second one. CONTEXT.md
 * defines the drag surface and the continuum check as one artifact seen
 * two ways; an exercise graded by its own private overflow test would
 * have made that two artifacts seen three ways.
 *
 * This module carries no `import.meta.glob`. That matters: #527 found
 * that an eager glob living in a module a plain `.spec.ts` imports drags
 * every component into the node-environment coverage report at nearly
 * zero execution, which dropped the repo from 100% to 94%. The globs
 * stay in the `.svelte.spec.ts` files and in `+page.svelte`, where they
 * only ever execute in a browser.
 */

// ADR-0024: 320 is a conformance commitment, not a content floor -- the
// only width this repo's verification ever names, and the low end of
// every sweep.
export const CONFORMANCE_COMMITMENT = 320;
// A sweep's step size is a resolution, not a design (ADR-0025) -- 4px is
// fine enough to catch a content floor and coarse enough to keep the
// suite fast across every component in one run.
export const RESOLUTION = 4;
// Sub-pixel rounding from the browser's own layout, not a real overflow.
export const TOLERANCE = 1;

/*
 * Whether this run is the canonical environment a content floor's
 * MINIMALITY is measured against (#564). The same font bytes rasterize
 * to different glyph widths on Linux/FreeType (CI) than on macOS/
 * CoreText (a contributor's own machine) -- confirmed directly, not
 * assumed: CI measured DataTable needing 775px where macOS measures
 * 768px, and OverviewHub's `--measure` cap at 585.0px on CI against
 * ~592px on macOS. So "the smallest space this still fits in" is not one
 * number across both, and only sufficiency (never LESS room than a floor
 * promises) is a property both environments can assert. `.github/
 * workflows/ci.yml`'s `app` job sets `VITE_FLOOR_CANONICAL` explicitly,
 * which is what this reads -- never `navigator.userAgent`, which is
 * guesswork about what a rasterizer does rather than a fact about which
 * one is running.
 */
export function isCanonicalEnvironment(): boolean {
	const environment = import.meta.env as Record<string, string | undefined>;
	return environment.VITE_FLOOR_CANONICAL === 'true';
}

export interface Break {
	width: number;
	needed: number;
}

/*
 * Sweeps a frame already holding a rendered demo, from 320px up to
 * whatever this environment naturally offers -- the same
 * `Math.max(availableSpace, CONFORMANCE_COMMITMENT)` the drag surface's
 * own page uses for its handle's far end, so the sweep invents no upper
 * width of its own either.
 */
export function sweep(frame: HTMLElement, availableSpace: number): Break | undefined {
	const widestSpace = Math.max(availableSpace, CONFORMANCE_COMMITMENT);
	for (let width = CONFORMANCE_COMMITMENT; width <= widestSpace; width += RESOLUTION) {
		frame.style.inlineSize = `${width}px`;
		void frame.offsetWidth;
		if (frame.scrollWidth - width > TOLERANCE) {
			return { width, needed: frame.scrollWidth };
		}
	}
	return undefined;
}

/*
 * Where a session meets the layout exercise (#534). The baseline capture
 * on #521 is the reason this is a failure message and not a line in
 * `CLAUDE.md`: that session read `CLAUDE.md`, wrote no CSS, shipped a
 * screen 93px past its own edge at 320px, and never once thought about
 * layout. A carrier it can walk past has already been tried here and
 * lost. A failing check cannot be walked past, so the teaching rides on
 * the one sentence the session is forced to read.
 */
export const EXERCISE_ROUTE = '/style-guide/layout-exercise';

// The webfont this repo ships (`fonts.css`), named as a plain constant
// rather than read out of that file, since this function verifies
// against `document.fonts`, not against CSS source.
const FONT_FAMILY = 'Hanken Grotesk';

/*
 * #550's own fix was `await document.fonts.ready`, and it is not enough
 * on its own: `ready` resolves once every REQUESTED font load has
 * settled, but `font-display: swap` (`fonts.css`) means nothing has
 * necessarily been requested yet the instant a render call returns --
 * `ready` can resolve with zero faces in flight, and a measurement taken
 * right after silently reads `Hanken Grotesk Fallback`, the
 * metric-compatible stand-in, instead of the real face. The fallback is
 * not uniformly narrower or wider -- it depends on the specific glyphs
 * and kerning pairs in play -- so this one bug can push one component's
 * floor too low and another's too high inside the same run, which is
 * exactly the shape four floor tests failed in CI (never locally, where
 * the face was already warm from an earlier run).
 *
 * The fix requests the face explicitly and does not trust `ready` alone
 * to prove it arrived. `FontFaceSet#check` was tried first and rejected:
 * measured directly, it reports `true` for a font family that is not
 * registered anywhere in the document at all, which makes it useless as
 * the "did the real face actually load" gate this function exists to be.
 * `document.fonts` itself is iterable and yields the `FontFace` objects
 * `fonts.css`'s `@font-face` rules registered, each with its own
 * `status`, so this reads THAT instead: at least one registered face
 * named `Hanken Grotesk` (there are two, one per `unicode-range` chunk --
 * `fonts.css` -- and only the one this repo's Latin-only content actually
 * needs is expected to load) must report `'loaded'`. `font-weight: 400
 * 600` on both is a single variable-weight range rather than one face
 * per weight this repo's tokens set (`--font-weight-normal/medium/
 * semibold`), so one request at any weight in that range triggers the
 * same resource fetch every weight would.
 */
export async function ensureFontLoaded(): Promise<void> {
	await document.fonts.load(`1rem "${FONT_FAMILY}"`);
	await document.fonts.ready;
	const isLoaded = [...document.fonts].some(
		(face) => face.family === FONT_FAMILY && face.status === 'loaded'
	);
	if (!isLoaded) {
		throw new Error(
			`"${FONT_FAMILY}" did not report a loaded face before a measurement. Measuring against the fallback face produces a confidently wrong number rather than an honest failure, so this stops instead.`
		);
	}
}

export function overflowReport(name: string, found: Break): string {
	return [
		`${name} needed ${found.needed}px inside the ${found.width}px it was given.`,
		'A component that needs more room than it is given has one configuration at every',
		'available space and no content floor to switch on (CONTEXT.md).',
		`The worked answer is the exercise at ${EXERCISE_ROUTE}: START breaks exactly this way`,
		'on purpose, FINISHED does not, and the diff between the two is what to do here.'
	].join(' ');
}
