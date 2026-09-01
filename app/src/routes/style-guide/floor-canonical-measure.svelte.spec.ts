/*
 * TEMPORARY diagnostic for #564 -- deleted before this branch lands.
 * Sweeps every discovered @container condition in THIS run's own
 * environment and prints its fixed point per criterion, so the
 * canonical-environment floors can be read off CI's log rather than
 * guessed by rounding up the macOS numbers.
 *
 * Gated behind `isCanonicalEnvironment()`, exactly like the real check's
 * own minimality half: locally this whole file is skipped, so `bun run
 * test:unit:coverage` -- what the pre-commit hook runs -- stays green and
 * this can be committed and pushed normally. It only runs, and only then
 * deliberately fails so its payload reaches the log (this reporter does
 * not print `console.log` for a passing test, confirmed directly), once
 * CI sets `VITE_FLOOR_CANONICAL`.
 */
import { describe, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { atomPages, moleculePages, organismPages, templatePages, toSlug } from './components.js';
import {
	CONFORMANCE_COMMITMENT,
	ensureFontLoaded,
	isCanonicalEnvironment,
	RESOLUTION,
	TOLERANCE
} from './continuum.js';
import { toDemos, type PageModule } from './drag-surface/dragSurface.js';
import { findConditions, forceLive, scopeClasses, type Condition } from './floor.js';

const sources = import.meta.glob('../../lib/components/**/*.svelte', {
	eager: true,
	query: '?raw',
	import: 'default'
}) as Record<string, string>;
const conditions = findConditions(sources);

type Criterion =
	| { readonly kind: 'overflow' }
	| { readonly kind: 'measure'; readonly target: string; readonly cap: 'form-max' | 'measure' }
	| { readonly kind: 'no-wrap'; readonly selectors: readonly string[] };

// Same registry as floor.svelte.spec.ts's CRITERIA, minus the coupled
// StepRail entry -- its value is read off QuestionPage's own result
// instead of swept, since it has no independent floor to sweep for.
const CRITERIA: Readonly<Record<string, Criterion>> = {
	'organisms/DataTable.svelte#1': { kind: 'overflow' },
	'organisms/StaffTopBar.svelte#1': { kind: 'overflow' },
	'organisms/PortalTopBar.svelte#1': { kind: 'overflow' },
	'templates/OverviewHub.svelte#1': { kind: 'measure', target: '.body > stack-l', cap: 'measure' },
	'templates/RecordDetail.svelte#1': { kind: 'measure', target: '.sections', cap: 'measure' },
	'templates/QuestionPage.svelte#1': { kind: 'measure', target: '.column', cap: 'form-max' },
	'templates/CheckAnswers.svelte#1': { kind: 'no-wrap', selectors: ['dt', '.action a'] },
	'templates/CheckAnswers.svelte#2': { kind: 'measure', target: '.column', cap: 'form-max' }
};

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

const WIDE = 1700;

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
	await ensureFontLoaded();
	return { run, frame };
}

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

function capProbe(frame: HTMLElement, cap: 'form-max' | 'measure'): HTMLElement {
	const probe = document.createElement('div');
	probe.style.inlineSize = cap === 'form-max' ? 'var(--form-max)' : 'var(--measure)';
	probe.style.fontSize = 'var(--text-body-size)';
	frame.append(probe);
	return probe;
}

function lineCount(element: Element): number {
	const range = document.createRange();
	range.selectNodeContents(element);
	return range.getClientRects().length;
}

async function sweepFixedPoint(condition: Condition, criterion: Criterion): Promise<number> {
	const { frame } = await mount(demoForCondition(condition).component);
	forceLive(condition, scopeClasses(frame));
	const wrap = criterion.kind === 'overflow' ? neutralizeWrap() : undefined;
	const probe = criterion.kind === 'measure' ? capProbe(frame, criterion.cap) : undefined;
	const target =
		criterion.kind === 'measure' ? frame.querySelector(criterion.target) : undefined;
	try {
		for (let width = CONFORMANCE_COMMITMENT; width <= WIDE; width += RESOLUTION) {
			atWidth(frame, width);
			if (criterion.kind === 'overflow') {
				if (frame.scrollWidth - width <= TOLERANCE) return width;
			} else if (criterion.kind === 'no-wrap') {
				const wrapped = criterion.selectors.some((selector) =>
					[...frame.querySelectorAll(selector)].some((element) => lineCount(element) > 1)
				);
				if (!wrapped) return width;
			} else if (probe && target) {
				const capPx = probe.getBoundingClientRect().width;
				const reachedPx = target.getBoundingClientRect().width;
				if (capPx - reachedPx <= TOLERANCE) return width;
			}
		}
		return -1;
	} finally {
		wrap?.remove();
	}
}

describe('canonical fixed-point sweep (temporary, #564)', () => {
	for (const condition of conditions) {
		const key = toRegistryKey(condition);
		const criterion = CRITERIA[key];
		if (!criterion) continue; // StepRail (coupled) and anything unregistered

		if (!isCanonicalEnvironment()) {
			it.skip(`sweeps ${key} (skipped outside the canonical environment)`, () => {});
			continue;
		}

		it(`sweeps ${key}`, async () => {
			const px = await sweepFixedPoint(condition, criterion);
			const rem = px / 16;
			// Deliberately failing: this repo's reporter only surfaces
			// console output for a failing test, and this line IS the
			// payload -- read it straight out of the CI log.
			throw new Error(`CANONICAL_FIXED_POINT ${key} ${px}px ${rem}rem`);
		});
	}
});
