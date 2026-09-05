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
 *   Minimality: the same wide configuration is NOT acceptable a step
 *   further down -- 5% of the floor, never less than `2 * RESOLUTION`.
 *   The depth is the one compromise in this file and carries its own
 *   reasoning where it is computed below: a floor is sufficient in every
 *   environment and minimal in one, and where those two environments
 *   disagree by more than the probe depth, no literal satisfies both.
 *   Measured at 8px on OverviewHub and again on RecordDetail. Runs ONLY
 *   in the canonical environment
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
 * `QuestionPage` and `CheckAnswers`'s rail split all lost their queries to
 * `sidebar-l` (Every Layout's own Sidebar), and `CheckAnswers`'s row to
 * content-derived flexbox. Nine conditions are four, and each survivor
 * picks WHICH DOM TREE renders rather than how one tree arranges itself,
 * which is the one job no intrinsic mechanism can do: CSS rearranges a
 * tree, it does not swap one.
 *
 * `StepRail` was the exception that proved the rule, in `UNDERIVABLE`
 * below rather than here. It is not any more: #585 replaced its query
 * with a `<details>` the reader opens, so it authors no condition at all
 * and `UNDERIVABLE` is empty.
 *
 * Every surviving entry in `CRITERIA` below carries a `justification`, and
 * `has a justification for existing at all` (below) is what enforces
 * that a tenth condition cannot join without writing one down.
 */
import { describe, expect, it } from 'vitest';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { atomPages, moleculePages, organismPages, templatePages, toSlug } from './components.js';
import { isCanonicalEnvironment, mountInFrame, RESOLUTION, TOLERANCE } from './continuum.js';
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
 * The instruments, kept in full even where no condition currently uses
 * one -- the mechanism is what found every one of #564's own findings,
 * not the particular set of conditions surviving today:
 *
 *   `overflow`     does the rendered content need more room than it is
 *                  given, with emergency wrapping neutralized
 *   `measure`      does the primary column beside a rail reach its own
 *                  cap, `--form-max` or `--measure`
 *   `no-wrap`      does a short, author-controlled string -- a label, an
 *                  action's name -- wrap onto more than one line. A value
 *                  is a Practice's own data of arbitrary length and is
 *                  deliberately excluded: wrapping IT is correct
 *   `single-row`   do a row's items share one row, or has the row wrapped
 *                  onto a second. Distinct from `no-wrap`, which asks
 *                  whether ONE element's text broke: a cluster of chips
 *                  can sit on two rows with no chip wrapping inside
 *                  itself, and `lineCount`'s Range rects cannot tell the
 *                  two apart because separate chips produce separate
 *                  rects on the same line
 * There was a fourth kind, `coupled` -- this condition shares its host
 * Template's ancestor container and has no content-driven floor of its
 * own, so it must fire at exactly the width its host does. It is deleted
 * (#585). It never had a member: `StepRail`, the only component its
 * comment ever named, sat in `UNDERIVABLE` instead, so the assertion
 * `expect(condition.floorRem).toBe(host!.floorRem)` never ran on
 * anything. #585's body and #518's #729 entry both stated that the
 * coupling was asserted; it was not, which is why nothing failed when
 * `StepRail`'s literal outlived the host query it was copied from. A
 * criterion that appears to hold a component to a rule and does not is
 * worse than no criterion, so it goes rather than gaining a member.
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
	| { readonly kind: 'single-row'; readonly selector: string; readonly justification: string };

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
		kind: 'single-row',
		selector: '.contents-strip cluster-l',
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
 * **Empty, and kept empty deliberately** (#585). Its only member was ever
 * `StepRail#1`, whose query tried to read whether `sidebar-l` had wrapped
 * -- and #585 measured that it read it wrong, by 2px on two hosts and by
 * 125px on `CheckAnswers` in its wide state, because a wrap is an event
 * no selector reports and the literal had outlived the host query it was
 * copied from. That component now renders one presentation with a
 * `<details>` the reader opens, so it authors no condition and is
 * discovered by nothing here. ADR-0024 rule 1 carries the general rule.
 *
 * Kept rather than deleted, exactly as `KNOWN_BROKEN` and `UNSWEPT` are:
 * an empty named hatch makes the next exception visible in a diff, and
 * deleting it would make one invisible instead.
 */
const UNDERIVABLE: Readonly<Record<string, string>> = {};

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
 * The mount procedure is `continuum.ts`'s `mountInFrame` (#570), no longer
 * a copy of it: an unconstrained container-type frame, the base size
 * re-resolved against it (#544), and a wait for the real webfont without
 * which this check measured the fallback face on CI (#550). It used to be
 * copied here with a note saying so, because it lived inline in
 * `continuum.svelte.spec.ts`'s own `it` and no check could call it. One
 * instrument, not two -- which is what CONTEXT.md's "one artifact seen two
 * ways" asks of this file and that one.
 */

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
	criterion: Criterion,
	px: number,
	isExpected: boolean
): void {
	switch (criterion.kind) {
		case 'overflow': {
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
			break;
		}
		case 'no-wrap': {
			atWidth(frame, px);
			const measurement = measureWrap(frame, px, criterion.selectors, lineCount);
			expect(isWrapAcceptable(measurement), wrapFloorReport(key, measurement)).toBe(isExpected);
			break;
		}
		case 'single-row': {
			atWidth(frame, px);
			const row = frame.querySelector(criterion.selector);
			expect(row, `${criterion.selector} not found for ${key}`).not.toBeNull();
			const items = [...row!.children] as HTMLElement[];
			expect(items.length, `${criterion.selector} has no items for ${key}`).toBeGreaterThan(0);
			const top = items[0]!.getBoundingClientRect().top;
			const areOnOneRow = items.every(
				(item) => Math.abs(item.getBoundingClientRect().top - top) <= TOLERANCE
			);
			const rows = new Set(items.map((item) => Math.round(item.getBoundingClientRect().top))).size;
			expect(
				areOnOneRow,
				`${key}: given ${px}px, ${items.length} items across ${rows} row(s) (single-row criterion).`
			).toBe(isExpected);
			break;
		}
		default: {
			const probe = capProbe(frame, criterion.cap);
			const target = frame.querySelector(criterion.target);
			expect(target, `${criterion.target} not found for ${key}`).not.toBeNull();
			const capLabel = capCustomProperty(criterion.cap);
			atWidth(frame, px);
			const measurement = measureCap(px, capLabel, target!, probe);
			expect(isCapAcceptable(measurement), capFloorReport(key, measurement)).toBe(isExpected);
		}
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

		const floorPx = remToPx(condition.floorRem);
		/*
		 * How far below the floor minimality probes, and the only part of
		 * this check that is a compromise rather than a measurement.
		 *
		 * A floor has to be sufficient in EVERY environment, so it is the
		 * largest fixed point across them. Minimality is asserted in one,
		 * so it is judged against that one's fixed point. Where the
		 * canonical environment needs LESS room than another -- which
		 * happens, because the canonical rasterizer draws mixed text wider
		 * but its `0` narrower, so `ch`-derived caps and chip rows land
		 * lower there -- those two numbers differ by the cross-environment
		 * spread, and a probe shallower than that spread asks for a floor
		 * that cannot exist. It was measured at 8px on both OverviewHub
		 * and RecordDetail, exactly the depth a fixed 2 * RESOLUTION
		 * probed, and no literal satisfied both halves.
		 *
		 * So the depth scales with the floor and keeps a hard minimum: 5%,
		 * never less than 2 * RESOLUTION. That is far wider than any
		 * spread measured here (8px against 19px at RecordDetail's floor,
		 * 50px at OverviewHub's) and far tighter than the errors this
		 * exists to catch -- the 60rem set this ticket removed was 200px
		 * to 600px too large, which is 20% to 60%. Minimality still says
		 * "this number was measured, not chosen"; it no longer claims to
		 * pin the fixed point to the sweep's own resolution, which was
		 * never a claim any single literal could support across
		 * environments.
		 */
		const probeDepthPx = Math.max(2 * RESOLUTION, Math.round(floorPx * 0.05));
		const belowFloorPx = floorPx - probeDepthPx;

		it(`${key} is sufficient at its floor (${condition.floorRem}rem)`, async () => {
			const { run, frame } = await mountInFrame(demoForCondition(condition).component);
			try {
				forceLive(condition, scopeClasses(frame));
				assertCriterionAt(frame, key, criterion, floorPx, true);
			} finally {
				run.remove();
			}
		});

		const minimalityName = `${key} is minimal: not acceptable ${probeDepthPx}px below its floor`;
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
				const { run, frame } = await mountInFrame(demoForCondition(condition).component);
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
