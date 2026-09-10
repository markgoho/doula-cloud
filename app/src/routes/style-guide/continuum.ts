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

import type { Component } from 'svelte';
import { render } from 'vitest-browser-svelte';

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
 *
 * That ceiling is 414px under Vitest, and #600 settled that it stays
 * there rather than being widened -- decided, not merely inherited, and
 * recorded in ADR-0025. The window is not what holds it: a frame set to
 * 3000px inside that 414px window renders and measures correctly, so this
 * could have climbed further at any time. It does not, because space is
 * what an UNCONSTRAINED component runs out of, so it spills at 320px
 * where this already looks -- every defect these sweeps have found did --
 * and a constrained one spills nowhere. Measured: all 55 components swept
 * to 3000px break nowhere, and `DataTable` with its switch point dropped
 * to 480px, below what its content needs, still breaks nowhere, because
 * its cells carry `max-inline-size` and `overflow-wrap: anywhere`.
 * Deleting that constraint is what it took to make a widened sweep red.
 *
 * The one case a wider sweep would reach is a component rendering a
 * different DOM TREE above a content floor whose wide tree is
 * unconstrained (#542). `floor.svelte.spec.ts` owns that case: it forces
 * each discovered condition live and measures at its own floor, above
 * this ceiling.
 *
 * ## It looks inside a closed disclosure (#710)
 *
 * A closed `<details>` renders nothing but its `<summary>`: its content
 * takes no box at all, so `scrollWidth` cannot see it at any width. Until
 * #710 that made every disclosure a hole in this instrument -- #486 put
 * the Client portal's Activity ledger behind one, and the only thing that
 * measured what is actually in it was a second, hand-written overflow test
 * beside that one route. The next disclosure would have needed its own,
 * and a check a screen can be added to without joining is the shape #521
 * already proved gets walked past.
 *
 * So the sweep opens every closed disclosure under the frame before it
 * measures, and closes again after. It does that itself rather than
 * offering a fixture a hook to do it with: this repo's checks discover
 * their subjects and never wait to be opted into (`route-continuum`'s
 * `UNSWEPT`, `floor.svelte.spec.ts`'s `UNDERIVABLE`), and a hook a fixture
 * may decline is an opt-in with a longer name.
 *
 * Measuring the open state loses nothing the closed state held: a
 * `<summary>` renders in both, so an open disclosure's content is a
 * superset of a closed one's, and one sweep covers the pair. The drag
 * surface is untouched by this and needs to be -- a person standing in
 * front of it opens the disclosure by clicking it, which is the screen
 * behaving rather than the instrument reaching in.
 */
export function sweep(frame: HTMLElement, availableSpace: number): Break | undefined {
	const close = openDisclosures(frame);
	try {
		const widestSpace = Math.max(availableSpace, CONFORMANCE_COMMITMENT);
		for (let width = CONFORMANCE_COMMITMENT; width <= widestSpace; width += RESOLUTION) {
			frame.style.inlineSize = `${width}px`;
			void frame.offsetWidth;
			if (frame.scrollWidth - width > TOLERANCE) {
				return { width, needed: frame.scrollWidth };
			}
		}
		return undefined;
	} finally {
		close();
	}
}

/*
 * Opens every closed `<details>` under `frame` and hands back the undo
 * (#710). It is exported rather than kept private to `sweep` because the
 * floor check takes a measurement of its own and has the same blind spot:
 * #1124 is that work, and the query belongs in one place before a second
 * instrument writes its own copy -- which is #570's rule stated before the
 * copy exists rather than after.
 *
 * Only the disclosures that were CLOSED are touched -- a subject that
 * ships one already open (`StepRail`'s completed steps) is left as its own
 * markup declared it, since closing that would leave the frame in a state
 * the screen never has. Nothing here re-renders between the open and the
 * undo, so a Svelte-controlled `open={...}` is never reasserted mid-sweep.
 *
 * The undo is what keeps a measurement from being an action. A `toggle`
 * event is queued rather than dispatched synchronously, and repeated
 * changes coalesce, so a disclosure opened and closed again inside one
 * task never runs its `ontoggle` handler as open -- which is how the Staff
 * roster's work-state history is not fetched by the act of measuring the
 * roster. It is also why content a disclosure loads on open is measured in
 * its loading state, the gap #1126 holds. The undo serves a plainer
 * purpose too: a spec that reads the DOM after a sweep sees the screen it
 * mounted rather than the one the instrument left behind.
 *
 * What this does not handle, named because it is a limit rather than an
 * oversight: a grouped `<details name="...">`, where opening one closes
 * its siblings, would leave only the last of a group open and measured.
 * The app has no grouped disclosure today.
 */
export function openDisclosures(frame: HTMLElement): () => void {
	const closed = [...frame.querySelectorAll<HTMLDetailsElement>('details:not([open])')];
	for (const disclosure of closed) disclosure.open = true;
	void frame.offsetWidth;
	return () => {
		for (const disclosure of closed) disclosure.open = false;
	};
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

/*
 * Puts a subject in front of the sweep: an unconstrained run, a frame that
 * is a containment context, the pairing re-declared on the frame's own
 * children (#544), and a wait for the real webfont (#550).
 *
 * It lives here, beside `sweep`, because it was already written three
 * times. `continuum.svelte.spec.ts` had it inline in its own `it`;
 * `floor.svelte.spec.ts` carries a copy with a comment saying it is a copy
 * "because it is inline in that file's own `it`"; and #570's route sweep
 * would have been the third. CONTEXT.md calls the continuum check and the
 * drag surface one artifact seen two ways, and three private mount
 * procedures is how that stops being true -- #550 is what it looks like
 * when two halves of this instrument disagree, and each copy is another
 * place a fix like `ensureFontLoaded` has to be remembered.
 *
 * Importing `vitest-browser-svelte` here is safe: nothing that ships
 * imports this module. `floor.ts` does, and it is test-only; the drag
 * surface's own page imports `dragSurface.js` and the component registry,
 * never this.
 */
export interface Mounted {
	run: HTMLElement;
	frame: HTMLElement;
	remove(): void;
}

export async function mountInFrame(
	component: Component,
	properties: Record<string, unknown> = {}
): Promise<Mounted> {
	const run = document.createElement('div');
	const frame = document.createElement('div');
	frame.style.containerType = 'inline-size';
	run.append(frame);
	document.body.append(run);
	await render(component, properties, { baseElement: frame });
	/*
	 * The base size re-resolved against the frame (#544), which is the drag
	 * surface's `.frame > *` rule expressed in the DOM this builds by hand.
	 * `font-size` inherits as a computed length and a `cqi` resolves against
	 * the nearest ANCESTOR container, so without this the subject renders in
	 * letters sized for the window while the sweep reports 320px -- the
	 * instrument lying in the same direction every time.
	 */
	for (const child of frame.children) {
		(child as HTMLElement).style.fontSize = 'var(--text-body-size)';
	}
	await ensureFontLoaded();
	return { run, frame, remove: () => run.remove() };
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
