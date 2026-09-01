/*
 * The floor check (#564): for every `@container (min-width: …)` condition
 * under `src/lib/components`, asserts that its literal is a fixed point --
 * CONTEXT.md's Content floor entry -- rather than a value chosen with a
 * margin or copied from a neighbour.
 *
 *   Sufficiency: forced permanently live (`floor.ts`'s `forceLive`), the
 *   wide configuration is acceptable, by the condition's own criterion,
 *   at the floor width. Runs in EVERY environment -- this is the safety
 *   property: a floor that hands real content less room than it promised
 *   is a broken layout wherever it renders, and nothing about which
 *   rasterizer is running changes that.
 *
 *   Minimality: the same wide configuration is NOT acceptable one step
 *   further down -- `2 * RESOLUTION` (8px), not `RESOLUTION` (4px); a
 *   single step landed inside a sub-pixel margin on OverviewHub (~1.02px
 *   against a 1px `TOLERANCE`). Runs ONLY in the canonical environment
 *   (`continuum.ts`'s `isCanonicalEnvironment`), and says so in its own
 *   test name everywhere else, as `it.skip` rather than a silent pass.
 *   "The smallest space this still fits in" turned out not to be
 *   portable (#564): the same font bytes rasterize to different glyph
 *   widths on CI's Linux/FreeType than on a contributor's own macOS/
 *   CoreText, so a floor minimal on one machine can read as short --
 *   still `overflowReport`'s "one configuration at every available
 *   space" -- on the other, and no single number satisfies minimality on
 *   both. Sufficiency has no such problem, because "enough room
 *   everywhere" only ever gets harder to satisfy as more environments are
 *   added, never contradictory between two of them.
 *
 * Conditions are discovered from source (`floor.ts`'s `findConditions`),
 * not listed by hand, so a tenth `@container` joining the codebase with no
 * entry in `CRITERIA` below fails loudly rather than going unchecked.
 * Rendering reuses the drag surface's own demo registry -- the same
 * `pageModules` glob and `toDemos` `continuum.svelte.spec.ts` uses -- so
 * this is the second half of one artifact, not a private instrument built
 * beside it (CONTEXT.md).
 *
 * The burden of proof inverted (#564, after #520's own reading of Every
 * Layout on container queries as "circuit breakers... I'd sooner not have
 * them anywhere I know they're not needed"): a `@container` condition is
 * an authored threshold, the opposite of the intrinsic mechanisms this
 * repo otherwise reaches for, so existing at all now needs its own
 * justification, not just a criterion. `OverviewHub`, `RecordDetail`,
 * `QuestionPage`, `CheckAnswers`'s rail split, and `StepRail` all lost
 * their queries to `sidebar-l` (Every Layout's own Sidebar) or, for
 * `CheckAnswers`'s row, a content-derived CSS Grid; `RecordDetail`'s
 * chip/rail choice moved to `(pointer: coarse)`, a stated preference
 * ADR-0024 rule 3 already permits. Nine conditions are three. Every
 * surviving entry in `CRITERIA` below carries a `justification`, and
 * `has a justification for existing at all` (below) is what enforces
 * that a tenth condition cannot join without writing one down.
 */
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { atomPages, moleculePages, organismPages, templatePages, toSlug } from './components.js';
import { ensureFontLoaded, isCanonicalEnvironment, RESOLUTION, TOLERANCE } from './continuum.js';
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
 * documented exception to having any of them, kept even though only
 * `overflow` currently has a condition registered against it -- the
 * mechanism is what found every one of #564's own findings, not the
 * particular set of conditions surviving today: `overflow` (does the
 * rendered content need more room than it is given, wrapping
 * neutralized), `measure` (does the primary column beside a rail reach
 * `--form-max` or `--measure`, whichever caps it), `no-wrap` (does a
 * short, author-controlled string -- a label, an action's name -- wrap
 * onto more than one line; a value is a Practice's own data of arbitrary
 * length and is deliberately excluded, since wrapping IT is correct
 * rather than a defect), and `coupled` (this condition shares its host
 * Template's ancestor container and has no content-driven floor of its
 * own).
 *
 * `justification` is the new field (#564): every entry states, in
 * writing, why no intrinsic mechanism can do this job -- the burden the
 * check now puts on a query existing at all, not only on its number.
 *
 * A registry of CRITERIA, never of widths: every value below names an
 * instrument, a justification, or a reference to another condition, and
 * the number each resolves to is read off the source, not written here.
 */
type Criterion =
	| { readonly kind: 'overflow'; readonly justification: string }
	| {
			readonly kind: 'measure';
			readonly target: string;
			readonly cap: 'form-max' | 'measure';
			readonly justification: string;
	  }
	| { readonly kind: 'no-wrap'; readonly selectors: readonly string[]; readonly justification: string }
	| { readonly kind: 'coupled'; readonly to: readonly string[]; readonly justification: string };

const CRITERIA: Readonly<Record<string, Criterion>> = {
	'organisms/DataTable.svelte#1': {
		kind: 'overflow',
		justification:
			'A <table> versus one <dl> per record is a different DOM tree, not the same content laid out ' +
			'differently -- no intrinsic CSS mechanism swaps markup (ADR-0024), so a query is the only way ' +
			'to pick between the two trees a route renders.'
	},
	'organisms/StaffTopBar.svelte#1': {
		kind: 'overflow',
		justification:
			'A wide nav row versus a menu-button sheet is a different DOM tree -- two landmarks, one always ' +
			'display:none -- for the same reason DataTable keeps its own query: no intrinsic mechanism ' +
			'swaps markup, only a query can pick which tree renders.'
	},
	'organisms/PortalTopBar.svelte#1': {
		kind: 'overflow',
		justification:
			'The same shape as StaffTopBar: a wide nav row versus a narrow stacked row is a different DOM ' +
			'tree, and no intrinsic mechanism swaps markup.'
	},
	'templates/RecordDetail.svelte#1': {
		kind: 'no-wrap',
		selectors: ['.contents-links'],
		justification:
			'A chip row versus a vertical list of links is a different DOM tree, one always display:none so ' +
			'nothing is announced twice -- no intrinsic mechanism swaps markup. The vertical list is correct ' +
			'at any width, so it is the chip row that earns its place, and it earns it by fitting on one ' +
			'line. Answered entirely from this block own containment context, so it never has to know ' +
			'whether sidebar-l put it beside the record or above it.'
	}
};

/*
 * A condition that has no content question to derive a floor from, and is
 * therefore not justified but is also not silently accepted -- the same
 * shape as `continuum.svelte.spec.ts`'s `KNOWN_BROKEN`, and for the same
 * reason: an exception that names its ticket is visible, and one that
 * does not is a suppression.
 *
 * `StepRail`'s two presentations are both correct at every width -- a
 * vertical list of steps reads fine anywhere, and its strip is a summary
 * line, a track and a link stacked vertically, so nothing in either fits
 * or fails to fit. What separates them is vertical cost in a layout
 * context the component cannot observe: `sidebar-l` decides whether the
 * journey sits beside the record or above it, the browser decides that,
 * and CSS exposes no selector reporting it. Detecting it from the rail's
 * own width was tried and disproved on #564 -- paired it is exactly its
 * 20rem basis and wrapped it is the container's width, and at 320px those
 * are the same number, so any floor separating them would sit on the
 * conformance commitment, which CONTEXT.md forbids.
 */
const UNDERIVABLE: Readonly<Record<string, string>> = {
	'organisms/StepRail.svelte#1': '#585'
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

/*
 * One criterion-branch, shared by the sufficiency test (every
 * environment, at the floor) and the minimality test (canonical
 * environment only, below it) -- each now its own `it()` with its own
 * mount, so this is what keeps the three criteria' measuring logic
 * written once rather than twice.
 */
function assertCriterionAt(
	frame: HTMLElement,
	key: string,
	criterion: Exclude<Criterion, { readonly kind: 'coupled' }>,
	px: number,
	isExpected: boolean
): void {
	if (criterion.kind === 'overflow') {
		const wrap = neutralizeWrap();
		try {
			atWidth(frame, px);
			const measurement = measureOverflow(frame, px);
			expect(isOverflowAcceptable(measurement), overflowFloorReport(key, measurement)).toBe(
				isExpected
			);
		} finally {
			wrap.remove();
		}
	} else if (criterion.kind === 'no-wrap') {
		atWidth(frame, px);
		const measurement = measureWrap(frame, px, criterion.selectors, lineCount);
		expect(isWrapAcceptable(measurement), wrapFloorReport(key, measurement)).toBe(isExpected);
	} else {
		const probe = capProbe(frame, criterion.cap);
		const target = frame.querySelector(criterion.target);
		expect(target, `${criterion.target} not found for ${key}`).not.toBeNull();
		const capLabel = capCustomProperty(criterion.cap);
		atWidth(frame, px);
		const measurement = measureCap(px, capLabel, target!, probe);
		expect(isCapAcceptable(measurement), capFloorReport(key, measurement)).toBe(isExpected);
	}
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
			.filter((key) => !Object.hasOwn(CRITERIA, key))
			.filter((key) => !Object.hasOwn(UNDERIVABLE, key));
		expect(unregistered, unregistered.join(', ')).toEqual([]);
	});

	/*
	 * An exception has to stay pointed at its ticket. If the condition is
	 * gone, or has grown a real criterion, the exception is stale and says
	 * so here rather than sitting in the file forever.
	 */
	it('names no exception the source no longer has', () => {
		const discoveredKeys = new Set(conditions.map((condition) => toRegistryKey(condition)));
		const stale = Object.keys(UNDERIVABLE).filter(
			(key) => !discoveredKeys.has(key) || Object.hasOwn(CRITERIA, key)
		);
		expect(stale, stale.join(', ')).toEqual([]);
	});

	it('names no criterion the source no longer has', () => {
		const discoveredKeys = new Set(conditions.map((condition) => toRegistryKey(condition)));
		const stale = Object.keys(CRITERIA).filter((key) => !discoveredKeys.has(key));
		expect(stale, stale.join(', ')).toEqual([]);
	});

	/*
	 * The burden of proof (#564): a query is an authored threshold, the
	 * opposite of the intrinsic mechanisms this repo otherwise reaches
	 * for, so existing at all needs its own written reason -- not just a
	 * criterion to be judged by once it exists. A minimum length rather
	 * than mere presence, so a placeholder string cannot stand in for an
	 * actual argument.
	 */
	const MINIMUM_JUSTIFICATION_LENGTH = 40;

	it('has a justification for existing at all, for every criterion', () => {
		const unjustified = Object.entries(CRITERIA)
			.filter(([, criterion]) => criterion.justification.trim().length < MINIMUM_JUSTIFICATION_LENGTH)
			.map(([key]) => key);
		expect(unjustified, unjustified.join(', ')).toEqual([]);
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
		// Stays tight rather than widening to absorb cross-platform spread:
		// minimality no longer crosses platforms (below), so there is no
		// spread left for a wider probe to buy anything against.
		const belowFloorPx = floorPx - 2 * RESOLUTION;

		it(`${key} is sufficient at its floor (${condition.floorRem}rem)`, async () => {
			const { run, frame } = await mount(demoForCondition(condition).component);
			try {
				forceLive(condition, scopeClasses(frame));
				assertCriterionAt(frame, key, criterion, floorPx, true);
			} finally {
				run.remove();
			}
		});

		const minimalityName = `${key} is minimal: not acceptable ${2 * RESOLUTION}px below its floor`;
		/*
		 * Minimality only in the canonical environment (`continuum.ts`'s
		 * `isCanonicalEnvironment`) -- see the file-level comment for why.
		 * `it.skip` rather than omitting the test entirely, so `bun run
		 * test:unit` on a contributor's own machine shows this as skipped
		 * in the output rather than reading as a property nobody thought
		 * to check; the "(skipped outside...)" suffix says why without
		 * needing the source open.
		 */
		if (isCanonicalEnvironment()) {
			it(minimalityName, async () => {
				const { run, frame } = await mount(demoForCondition(condition).component);
				try {
					forceLive(condition, scopeClasses(frame));
					assertCriterionAt(frame, key, criterion, belowFloorPx, false);
				} finally {
					run.remove();
				}
			});
		} else {
			it.skip(`${minimalityName} (skipped outside the canonical environment)`, () => {});
		}
	}
});
