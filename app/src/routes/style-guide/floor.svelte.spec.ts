/*
 * The floor check (#564): for every `@container (min-width: …)` condition
 * under `src/lib/components`, asserts that its literal is a fixed point --
 * CONTEXT.md's Content floor entry -- rather than a value chosen with a
 * margin or copied from a neighbour.
 *
 *   Sufficiency: forced permanently live (`floor.ts`'s `forceLive`), the
 *   wide configuration is acceptable, by the condition's own criterion,
 *   at the floor width.
 *
 *   Minimality: the same wide configuration is NOT acceptable one step
 *   further down -- `2 * RESOLUTION` (8px), not `RESOLUTION` (4px). A
 *   single step landed inside a sub-pixel margin on OverviewHub (~1.02px
 *   against a 1px `TOLERANCE`), which is a flake waiting to happen rather
 *   than a genuine assertion; two steps reads several px past the true
 *   crossing on every floor measured so far. The property this proves is
 *   slightly weaker as a result -- the floor is within `2 * RESOLUTION`
 *   of the true fixed point, not within `RESOLUTION` -- but `RESOLUTION`
 *   was already a resolution and not a design (ADR-0025), so widening it
 *   loses nothing the check ever claimed to guarantee.
 *
 * Conditions are discovered from source (`floor.ts`'s `findConditions`),
 * not listed by hand, so a tenth `@container` joining the codebase with no
 * entry in `CRITERIA` below fails loudly rather than going unchecked.
 * Rendering reuses the drag surface's own demo registry -- the same
 * `pageModules` glob and `toDemos` `continuum.svelte.spec.ts` uses -- so
 * this is the second half of one artifact, not a private instrument built
 * beside it (CONTEXT.md).
 */
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { atomPages, moleculePages, organismPages, templatePages, toSlug } from './components.js';
import { ensureFontLoaded, RESOLUTION, TOLERANCE } from './continuum.js';
import { toDemos, type PageModule } from './drag-surface/dragSurface.js';
import {
	capFloorReport,
	findConditions,
	findConditionsBelowConformance,
	findMalformedConditions,
	forceLive,
	measureCap,
	measureOverflow,
	measureWrap,
	overflowFloorReport,
	remToPx,
	scopeClasses,
	wrapFloorReport,
	type CapMeasurement,
	type Condition,
	type OverflowMeasurement,
	type WrapMeasurement
} from './floor.js';

const sources = import.meta.glob('../../lib/components/**/*.svelte', {
	eager: true,
	query: '?raw',
	import: 'default'
}) as Record<string, string>;

const conditions = findConditions(sources);

/*
 * Three instruments (CONTEXT.md, this ticket's own decisions) plus one
 * documented exception to having any of them: `overflow` (does the
 * rendered content need more room than it is given, wrapping
 * neutralized), `measure` (does the primary column beside a rail reach
 * `--form-max` or `--measure`, whichever caps it), `no-wrap` (does a
 * short, author-controlled string -- a label, an action's name -- wrap
 * onto more than one line; a value is a Practice's own data of arbitrary
 * length and is deliberately excluded, since wrapping IT is correct
 * rather than a defect), and `coupled` (this condition shares its host
 * Template's ancestor container and has no content-driven floor of its
 * own -- see `StepRail.svelte`'s own comment on the rule this measures).
 *
 * `no-wrap` exists because `overflow` fails on a flexible track exactly
 * the way it fails on a rail: a wrappable string in `1fr 1fr auto` never
 * needs more room than it is given, it just wraps to one word per line,
 * so the overflow criterion reports every width as acceptable and finds
 * no floor at all -- which is indistinguishable from a genuine floor of
 * 0 until `findConditionsBelowConformance` below refuses to let a number
 * that low stand unquestioned (`CheckAnswers.svelte`'s own comment on its
 * row condition tells this story first-hand).
 *
 * A registry of CRITERIA, never of widths: every value below names an
 * instrument or a reference to another condition, and the number each
 * resolves to is read off the source, not written here.
 */
type Criterion =
	| { readonly kind: 'overflow' }
	| { readonly kind: 'measure'; readonly target: string; readonly cap: 'form-max' | 'measure' }
	| { readonly kind: 'no-wrap'; readonly selectors: readonly string[] }
	| { readonly kind: 'coupled'; readonly to: readonly string[] };

const CRITERIA: Readonly<Record<string, Criterion>> = {
	'organisms/DataTable.svelte#1': { kind: 'overflow' },
	'organisms/StaffTopBar.svelte#1': { kind: 'overflow' },
	'organisms/PortalTopBar.svelte#1': { kind: 'overflow' },
	'templates/OverviewHub.svelte#1': { kind: 'measure', target: '.body > stack-l', cap: 'measure' },
	'templates/RecordDetail.svelte#1': { kind: 'measure', target: '.sections', cap: 'measure' },
	'templates/QuestionPage.svelte#1': { kind: 'measure', target: '.column', cap: 'form-max' },
	// `dt` is every row's label; `.action a` is every row's "Change" link.
	// `.value` (the Practice's own data) is deliberately not named here.
	'templates/CheckAnswers.svelte#1': { kind: 'no-wrap', selectors: ['dt', '.action a'] },
	'templates/CheckAnswers.svelte#2': { kind: 'measure', target: '.column', cap: 'form-max' },
	// `StepRail` renders inside BOTH `QuestionPage` and `CheckAnswers`, so
	// both are named here rather than one: coupling it to only one host
	// would leave the other free to drift without this check noticing.
	'organisms/StepRail.svelte#1': {
		kind: 'coupled',
		to: ['templates/QuestionPage.svelte#1', 'templates/CheckAnswers.svelte#2']
	}
};

// `../../lib/components/organisms/DataTable.svelte` -> `organisms/DataTable.svelte#1`
function toRegistryKey(condition: Condition): string {
	return condition.file.replace('../../lib/components/', '') + '#' + condition.index;
}

const pageModules = import.meta.glob<PageModule>('./*/+page.svelte', { eager: true });
const demos = toDemos(pageModules, [
	...atomPages,
	...moleculePages,
	...organismPages,
	...templatePages
]);

function demoForCondition(condition: Condition) {
	const componentName = condition.file.split('/').at(-1)!.replace(/\.svelte$/, '');
	const slug = toSlug(componentName);
	const demo = demos.find((d) => d.slug === slug);
	if (!demo) throw new Error(`no style-guide demo for ${slug} (${condition.key})`);
	return demo;
}

if (!customElements.get('stack-l')) registerLayoutPrimitives();

/*
 * The same mount procedure `continuum.svelte.spec.ts` uses on its own
 * demos -- an unconstrained container-type frame, the base size
 * re-resolved against it (#544), and a wait for the real webfont (#550) --
 * copied rather than imported because it is inline in that file's own
 * `it`, not a function either check can call.
 */
async function mount(component: PageModule['default']) {
	const run = document.createElement('div');
	const frame = document.createElement('div');
	frame.style.containerType = 'inline-size';
	run.append(frame);
	document.body.append(run);
	await render(component, {}, { baseElement: frame });
	for (const child of frame.children) {
		(child as HTMLElement).style.fontSize = 'var(--text-body-size)';
	}
	/*
	 * `document.fonts.ready` alone let this check measure the fallback
	 * face on CI: it resolves once every REQUESTED load has settled, not
	 * once `font-display: swap` (`fonts.css`) has actually requested one,
	 * so it can resolve with nothing in flight yet. `ensureFontLoaded`
	 * requests the real face explicitly and refuses to let a measurement
	 * proceed if it did not report loaded -- shared with
	 * `continuum.svelte.spec.ts`, which hit the same bug (#550) and is
	 * meant to be one instrument with this file, not two.
	 */
	await ensureFontLoaded();
	return { run, frame };
}

/*
 * Decision #1 (#564): a floor is measured with emergency wrapping
 * neutralized, because `overflow-wrap: anywhere` changes what happens
 * after content stops fitting, not when it stops fitting -- so the
 * overflow criterion would report a break as fixed exactly when it is
 * still there, rescued.
 */
function neutralizeWrap(): HTMLStyleElement {
	const style = document.createElement('style');
	style.textContent = '* { overflow-wrap: normal !important; word-break: normal !important; }';
	document.head.append(style);
	return style;
}

function atWidth(frame: HTMLElement, px: number) {
	frame.style.inlineSize = `${px}px`;
	void frame.offsetWidth;
}

function isOverflowAcceptable(measurement: OverflowMeasurement): boolean {
	return measurement.needed - measurement.given <= TOLERANCE;
}

/*
 * The cap a measure-criterion column is judged against, resolved the same
 * way the demo's own content resolves it -- a probe inside the frame
 * (so it inherits the pairing's re-resolved base size) rather than a
 * literal copied from `tokens.css`, so a token change cannot make this
 * check quietly stale.
 */
function capProbe(frame: HTMLElement, cap: 'form-max' | 'measure'): HTMLElement {
	const probe = document.createElement('div');
	probe.style.inlineSize = cap === 'form-max' ? 'var(--form-max)' : 'var(--measure)';
	probe.style.fontSize = 'var(--text-body-size)';
	frame.append(probe);
	return probe;
}

function isCapAcceptable(measurement: CapMeasurement): boolean {
	return measurement.capPx - measurement.reachedPx <= TOLERANCE;
}

// `--form-max` / `--measure`, not `criterion.cap`'s bare `'form-max'` /
// `'measure'`: the report should name the actual custom property, not the
// registry's own internal spelling of it.
function capCustomProperty(cap: 'form-max' | 'measure'): string {
	return cap === 'form-max' ? '--form-max' : '--measure';
}

/*
 * Line count via `Range#getClientRects`, not `getBoundingClientRect`'s
 * height against a computed `line-height`: `.row`'s three cells share one
 * CSS Grid row and stretch to match whichever of them is tallest, so a
 * WRAPPING value inflates its sibling `<dt>`'s and `.action`'s own boxes
 * to the same height even when neither cell's own text has wrapped at
 * all -- measured directly on `/style-guide/check-answers`, that false
 * positive never cleared even at 1600px, because the long email value
 * always wraps somewhere. A `Range` over the element's own contents does
 * not share the grid row's stretch, so its rects reflect only the text
 * actually inside it.
 */
function lineCount(element: Element): number {
	const range = document.createRange();
	range.selectNodeContents(element);
	return range.getClientRects().length;
}

function isWrapAcceptable(measurement: WrapMeasurement): boolean {
	return measurement.wrapped.length === 0;
}

describe('the floor check (#564)', () => {
	it('never lets an @container condition escape discovery in a shape the check does not recognize', () => {
		// Decision #3: a floor stays a rem literal. A px value, a
		// min-inline-size, or a second joined condition parses as nothing
		// today rather than as a condition with no criterion -- this is
		// the test that catches THAT silent gap, not the registry test
		// above, which only catches a condition `findConditions` already
		// found.
		const malformed = findMalformedConditions(sources);
		expect(malformed.map((m) => `${m.file}:${m.line} -- ${m.raw}`)).toEqual([]);
	});

	it('never accepts a floor that resolves below the conformance commitment', () => {
		// The mistake this catches by name: CheckAnswers' row condition
		// was first measured under the overflow criterion at 284px,
		// which parses fine and looks like a plausible narrow floor, but
		// fires across the WHOLE continuum this repo verifies -- the
		// narrow branch it is meant to switch away from is unreachable,
		// which is CONTEXT.md's own failure sentence (`overflowReport`'s
		// "one configuration at every available space"). A floor this
		// low is evidence the criterion was wrong, not evidence of a
		// narrow floor, so it fails here rather than shipping quietly.
		const tooLow = findConditionsBelowConformance(sources);
		expect(tooLow.map((m) => `${m.file}:${m.line} -- ${m.raw}`)).toEqual([]);
	});

	it('has a criterion for every discovered condition', () => {
		const unregistered = conditions
			.map((condition) => toRegistryKey(condition))
			.filter((key) => !Object.hasOwn(CRITERIA, key));
		expect(unregistered, unregistered.join(', ')).toEqual([]);
	});

	it('names no criterion the source no longer has', () => {
		const discoveredKeys = new Set(conditions.map((condition) => toRegistryKey(condition)));
		const stale = Object.keys(CRITERIA).filter((key) => !discoveredKeys.has(key));
		expect(stale, stale.join(', ')).toEqual([]);
	});

	for (const condition of conditions) {
		const key = toRegistryKey(condition);
		const criterion = CRITERIA[key];
		if (!criterion) continue; // reported by the registry test above

		if (criterion.kind === 'coupled') {
			for (const to of criterion.to) {
				it(`${key} is coupled to ${to}`, () => {
					const host = conditions.find((c) => toRegistryKey(c) === to);
					expect(host, `${to} was not discovered`).toBeDefined();
					/*
					 * The whole assertion for a coupled condition: it has no
					 * content of its own to derive a floor from (StepRail's
					 * own comment records the overflow sweep that found none
					 * down to 144px), and it shares its host's ancestor
					 * container with no container of its own -- so the only
					 * way it can be correct is to fire at exactly the width
					 * its host does. Both hosts are checked, not just one:
					 * `StepRail` renders inside `QuestionPage` AND
					 * `CheckAnswers`, and a coupling to only one would leave
					 * the other free to drift without this catching it.
					 */
					expect(condition.floorRem).toBe(host!.floorRem);
				});
			}
			continue;
		}

		const floorPx = remToPx(condition.floorRem);
		// 2 * RESOLUTION (8px), not RESOLUTION: see the file-level comment
		// on minimality -- one step landed inside sub-pixel noise on
		// OverviewHub, and RESOLUTION was always a sweep resolution, not
		// a claim about how close to the true fixed point this proves.
		const belowFloorPx = floorPx - 2 * RESOLUTION;

		/*
		 * One mount and one `forceLive` call for both assertions, rather
		 * than a separate mount per width: testing sufficiency and
		 * minimality against the one forced rule they were both meant to
		 * observe is the more honest shape, since two separate mounts
		 * could each force their own copy of the rule and never prove the
		 * two widths are being read against the same fixed point.
		 */
		it(`${key} is a fixed point: acceptable at ${condition.floorRem}rem, not ${2 * RESOLUTION}px below it`, async () => {
			const { run, frame } = await mount(demoForCondition(condition).component);
			try {
				forceLive(condition, scopeClasses(frame));
				if (criterion.kind === 'overflow') {
					const wrap = neutralizeWrap();
					try {
						atWidth(frame, floorPx);
						const atFloor = measureOverflow(frame, floorPx);
						expect(isOverflowAcceptable(atFloor), overflowFloorReport(key, atFloor)).toBe(true);
						atWidth(frame, belowFloorPx);
						const belowFloor = measureOverflow(frame, belowFloorPx);
						expect(isOverflowAcceptable(belowFloor), overflowFloorReport(key, belowFloor)).toBe(
							false
						);
					} finally {
						wrap.remove();
					}
				} else if (criterion.kind === 'no-wrap') {
					atWidth(frame, floorPx);
					const atFloor = measureWrap(frame, floorPx, criterion.selectors, lineCount);
					expect(isWrapAcceptable(atFloor), wrapFloorReport(key, atFloor)).toBe(true);
					atWidth(frame, belowFloorPx);
					const belowFloor = measureWrap(frame, belowFloorPx, criterion.selectors, lineCount);
					expect(isWrapAcceptable(belowFloor), wrapFloorReport(key, belowFloor)).toBe(false);
				} else {
					const probe = capProbe(frame, criterion.cap);
					const target = frame.querySelector(criterion.target);
					expect(target, `${criterion.target} not found for ${key}`).not.toBeNull();
					const capLabel = capCustomProperty(criterion.cap);
					atWidth(frame, floorPx);
					const atFloor = measureCap(floorPx, capLabel, target!, probe);
					expect(isCapAcceptable(atFloor), capFloorReport(key, atFloor)).toBe(true);
					atWidth(frame, belowFloorPx);
					const belowFloor = measureCap(belowFloorPx, capLabel, target!, probe);
					expect(isCapAcceptable(belowFloor), capFloorReport(key, belowFloor)).toBe(false);
				}
			} finally {
				run.remove();
			}
		});
	}
});
