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

export function overflowReport(name: string, found: Break): string {
	return [
		`${name} needed ${found.needed}px inside the ${found.width}px it was given.`,
		'A component that needs more room than it is given has one configuration at every',
		'available space and no content floor to switch on (CONTEXT.md).',
		`The worked answer is the exercise at ${EXERCISE_ROUTE}: START breaks exactly this way`,
		'on purpose, FINISHED does not, and the diff between the two is what to do here.'
	].join(' ');
}
