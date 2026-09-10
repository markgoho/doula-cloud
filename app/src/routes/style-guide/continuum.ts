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
 * floor check takes a measurement of its own and had the same blind spot:
 * `floor.ts`'s `measureOverflow` is the second caller (#1124), so the query
 * lives in one place rather than in two instruments -- #570's rule stated
 * before the copy existed rather than after it.
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
 * roster. It is also why a measurement can never reach content a disclosure
 * LOADS on open: `revealDisclosures` below is where that content arrives,
 * in the preparation the measurement is taken after (#1126). The undo
 * serves a plainer purpose too: a spec that reads the DOM after a sweep
 * sees the screen it mounted rather than the one the instrument left
 * behind.
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
 * How many macrotask turns any of this instrument's waits may take before
 * it gives up (#885, #1126). A guard against a screen that polls, never a
 * budget anything is expected to spend: every response is served
 * synchronously from a fixture, so one turn drains however many `await`s a
 * section chains, and the deepest cascade this app has is eight reads long.
 */
export const SETTLE_TURNS = 50;

/*
 * The one bounded wait this instrument has (#1126), turned over to
 * whichever signal a caller can actually observe -- a route's own answering
 * going quiet, a disclosure reporting that it opened, a frame that has
 * stopped changing.
 *
 * There is one of these rather than one per signal because what matters is
 * the same in all of them and is easy to get wrong differently each time: a
 * wait that runs out must FAIL, loudly, and never fall through into a
 * measurement. A budget that expires quietly reports whatever happened to be
 * in the frame at that instant as the screen, which is the one way these
 * checks must not go wrong -- #885's part-built screens were exactly that,
 * and they were green.
 *
 * A turn is a macrotask, not a duration. Nothing here sleeps for a fixed
 * time: a sleep long enough to be safe is slow on every subject that did not
 * need it, and still wrong on the one that did.
 */
export async function awaitSettled(isSettled: () => boolean, stillArriving: string): Promise<void> {
	for (let turn = 0; turn < SETTLE_TURNS; turn += 1) {
		if (isSettled()) return;
		await new Promise((resolve) => setTimeout(resolve, 0));
	}
	throw new Error(
		`${stillArriving} after ${SETTLE_TURNS} turns. Measuring it anyway would report whatever happened to be in the frame at that moment as the screen, so this stops instead.`
	);
}

/*
 * "Nothing new happened this turn", as a signal `awaitSettled` can read
 * (#885, #1126). The caller counts whatever it can count -- responses
 * answered, DOM mutations seen -- and this reports the first turn that
 * added none.
 */
export function quiescence(sample: () => number): () => boolean {
	let previous = -1;
	return () => {
		const seen = sample();
		const isSettled = seen === previous;
		previous = seen;
		return isSettled;
	};
}

/*
 * Opens every closed disclosure under `frame` and waits for what opening
 * them brought in (#1126). It leaves them open.
 *
 * ## Why this is not `openDisclosures` with an `await` in it
 *
 * A disclosure that fetches its content on open cannot be measured on that
 * content unless the fetch happens, and #710's whole point is that MEASURING
 * must never make it happen -- an instrument that issued a request per
 * disclosure is an instrument with consequences. Awaiting the toggle inside
 * `sweep` would trade one for the other.
 *
 * So the two are separated instead of traded. This function prepares the
 * subject; `sweep` and `measureOverflow` measure it, unchanged from #710 and
 * #1124, still opening and closing inside one task and still firing no
 * handler. By the time either runs, the disclosures are already open, so
 * `openDisclosures` finds nothing closed and touches nothing at all.
 *
 * Preparation is where a screen's own loads already belong. `mountInFrame`
 * runs a route's `onMount` cascade and #885's settle wait exists precisely
 * because those loads are the check's to wait for; a disclosure opened here
 * reaches the state a person reaches by clicking one on the drag surface,
 * which is the screen behaving rather than the instrument reaching in. What
 * would be new is a REQUEST issued while measuring, and there is none:
 * `continuum.svelte.spec.ts` counts the subject's loads across a sweep and
 * gets zero.
 *
 * ## The two waits
 *
 * A `toggle` is queued as an element task, so "the handler has run" is not
 * something a `setTimeout(0)` can be assumed to come after -- task-source
 * ordering is exactly the kind of in-practice that flakes. This waits for
 * each opened disclosure to report its own toggle, and then for the frame to
 * stop changing, which is what says the content the handler asked for has
 * arrived AND been rendered. Both waits are `awaitSettled`, so both fail
 * loudly rather than falling through into a measurement.
 *
 * A subject with nothing closed pays neither wait: the common case is a
 * component demo with no disclosure at all, and it returns before the first
 * turn.
 *
 * ## Limits, named rather than left to be discovered
 *
 * Content that arrives holding a disclosure of its OWN, closed and loading,
 * is measured in ITS loading state: this opens what the frame held when it
 * was called, and does not look again. Nothing in the app nests one that
 * way. The grouped-`<details name>` limit `openDisclosures` names is this
 * function's too, since it opens through it.
 */
export async function revealDisclosures(frame: HTMLElement, subject: string): Promise<void> {
	const closed = [...frame.querySelectorAll<HTMLDetailsElement>('details:not([open])')];
	if (closed.length === 0) return;
	let toggled = 0;
	const listening = new AbortController();
	let mutations = 0;
	const observer = new MutationObserver((records) => {
		mutations += records.length;
	});
	try {
		for (const disclosure of closed) {
			disclosure.addEventListener(
				'toggle',
				() => {
					toggled += 1;
				},
				{ once: true, signal: listening.signal }
			);
		}
		observer.observe(frame, {
			subtree: true,
			childList: true,
			characterData: true,
			attributes: true
		});
		for (const disclosure of closed) disclosure.open = true;
		await awaitSettled(
			() => toggled === closed.length,
			`${subject} kept a disclosure that never reported opening`
		);
		await awaitSettled(
			quiescence(() => mutations),
			`${subject} was still filling in content a disclosure loaded on open`
		);
	} finally {
		observer.disconnect();
		listening.abort();
	}
	void frame.offsetWidth;
}

/*
 * The subject the two disclosure checks share (#710, #1124): a frame built
 * by hand around a `<details>` whose content is wider than any space either
 * instrument will offer it.
 *
 * Built by hand rather than through `mountInFrame` because what it needs is
 * a component no repo can ship -- the whole point is content that overflows
 * at every width, which every real subject is forbidden to be. It needs no
 * webfont wait either: the overflow is a declared inline size, not a
 * measured glyph.
 *
 * It lives here rather than beside either check because both take the same
 * measurement of the same blind spot -- `sweep` for the continuum check and
 * `measureOverflow` (`floor.ts`) for the floor check -- and #570's rule is
 * that one artifact is enforced by there being one function. The second
 * consumer is the bar, and #1124 is the second consumer.
 */

// Wider than the ~414px window these checks run in, so what breaks is the
// disclosure's own content rather than anything the frame could absorb.
export const OVERFLOWING = 900;

export interface HeldFrame {
	run: HTMLElement;
	frame: HTMLElement;
	remove(): void;
}

export function frameHolding(markup: string): HeldFrame {
	const run = document.createElement('div');
	const frame = document.createElement('div');
	frame.style.containerType = 'inline-size';
	frame.innerHTML = markup;
	run.append(frame);
	document.body.append(run);
	return { run, frame, remove: () => run.remove() };
}

// The Client portal's Activity ledger in miniature (#486): a summary a
// Client clicks, and behind it content that has to fit at 320px.
export function ledgerMarkup(isOpen = false): string {
	return (
		`<details${isOpen ? ' open' : ''}><summary>Show what has happened</summary>` +
		`<div style="inline-size: ${OVERFLOWING}px">Everything that has happened</div></details>`
	);
}

/*
 * The Staff roster's work-state history in miniature (#1126): a closed
 * disclosure holding nothing but the word `Loading...`, which asks for its
 * own content the first time it is opened and lays out something far wider
 * than the frame once that content lands.
 *
 * Both instruments need this one and neither can build it out of
 * `ledgerMarkup`, so it lives here with its sibling for the reason #1124
 * moved that one here: two consumers is the bar, and the pair of them are
 * the same blind spot measured twice.
 *
 * The load is a real `ontoggle` -> `await` -> insert path rather than a
 * flag a test flips. The content arrives a whole macrotask turn AFTER the
 * toggle, which is what a fetch answered by a fixture actually does, and it
 * is the reason a settle signal is needed at all: anything that opened the
 * disclosure and measured in the same turn would read `Loading...` and
 * report a screen that fits.
 *
 * `loads` counts the requests the subject issued. It is what pins the
 * property this ticket had to keep -- a measurement is not an action -- to
 * a number rather than to a doc comment: a sweep of this subject leaves it
 * at zero.
 */
export interface LoadingLedger extends HeldFrame {
	loads(): number;
}

export function frameHoldingLoadingLedger(): LoadingLedger {
	const held = frameHolding(
		'<details><summary>Show what has happened</summary><p>Loading...</p></details>'
	);
	const disclosure = held.frame.querySelector('details')!;
	const waiting = disclosure.querySelector('p')!;
	let loads = 0;
	disclosure.addEventListener('toggle', () => {
		if (!disclosure.open || loads > 0) return;
		loads += 1;
		setTimeout(() => {
			const loaded = document.createElement('div');
			loaded.style.inlineSize = `${OVERFLOWING}px`;
			loaded.textContent = 'Everything that has happened';
			waiting.replaceWith(loaded);
		}, 0);
	});
	return { ...held, loads: () => loads };
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
 * children (#544), a wait for the real webfont (#550), and the subject's
 * own disclosures opened and filled in (#1126).
 *
 * That last step is here rather than in either measurement because it is
 * preparation: it lets a disclosure that fetches on open do so, which a
 * measurement must never do. Every instrument mounts through this one
 * function -- the component sweep, the route sweep and the floor check
 * alike -- so all three measure the same revealed screen. A route's own
 * disclosures usually arrive later than this, behind its `onMount` cascade;
 * `route-continuum.svelte.spec.ts` reveals again once that cascade has gone
 * quiet, which is the same function called at the moment the subject
 * actually has them.
 *
 * A revealed disclosure is left open, and nothing re-closes it: the floor
 * check's `forceLive` re-renders nothing (it writes classes and inline
 * styles onto elements already mounted), and no component in this repo
 * binds `open`, so a subject that was opened here stays open for the
 * measurement that follows.
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
	await revealDisclosures(frame, 'A mounted subject');
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
