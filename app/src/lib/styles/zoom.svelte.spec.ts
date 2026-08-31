import { describe, expect, it } from 'vitest';
import tokensCss from './tokens.css?raw';
import './tokens.css';

/*
 * Zoom reachability -- #545.
 *
 * `tokens.spec.ts` asserts that no fluid step's ceiling is more than 2.5x
 * its floor, and that guard is arithmetic on the source. This file asserts
 * the property the guard stands in for, in a browser, against the real
 * stylesheet: every fluid step can be brought to twice its size within the
 * 500% a browser zooms to, and no amount of zoom ever makes one smaller.
 *
 * That is what WCAG 1.4.4 asks for. It wants 200% to be *reachable*, not
 * the browser's 200% setting to render it: "It is not required to achieve
 * 200% text enlargement while remaining inside a specific breakpoint (as
 * zooming may result in the variation for a new breakpoint becoming active),
 * but it should still be possible to get 200% text enlargement in some way"
 * (Understanding SC 1.4.4, Intent). A fluid step renders 1.6x to 2.0x at the
 * 200% setting -- 2.0x at the narrow end of the ramp where every step sits
 * on its `rem` floor, 1.8x for type and 1.6x for spacing at the wide end --
 * and no growth factor above 1 changes that, because the `cqi` term is an
 * absolute share of a container that zoom does not widen. Measured in
 * Chromium: 200% is reached by 2.0x zoom at 320px, 2.2x at 1440px, and 2.7x
 * in the worst case, `--space-6` at the top of the ramp.
 *
 * How page zoom is reproduced: zooming to Z scales every CSS pixel to Z
 * device pixels and narrows the CSS viewport to 1/Z of its device width. So
 * a frame given `width / Z` with the root font size left alone renders the
 * same CSS geometry, and the physical size is Z times the computed one.
 * Text-only zoom -- the root font size raised on its own -- produces the
 * identical ratios at every point, because both mechanisms multiply the
 * `rem` terms of the clamp and leave the `cqi` term where it is.
 */

/*
 * Every step on the ramp `tokens.css` declares -- read from the stylesheet
 * rather than listed here, so a step added later is covered without a second
 * list of token names to keep in step with the first. The shape is the
 * ramp's own: a `rem` floor, a `rem` intercept plus a `cqi` growth term, and
 * a `rem` ceiling, which is what `tokens.spec.ts` re-derives.
 */
const RAMP_STEP = /(--[\w-]+):\s*clamp\([\d.]+rem, [\d.]+rem \+ [\d.]+cqi, [\d.]+rem\);/g;
const ANY_FLUID = /(--[\w-]+):\s*clamp\([^;]*cqi[^;]*\);/g;

const fluidSteps = tokensCss.matchAll(RAMP_STEP).map(([, name]) => name).toArray();

/*
 * `--page-gutter` is the one fluid value that is not on the ramp:
 * `clamp(1rem, 2.5cqi, 3rem)`, a 3x spread with no `rem` intercept, given
 * its value by #531 and deliberately placed outside `tokens.spec.ts`'s 2.5x
 * guard by #533 for the reason that applies here too -- it is not text, and
 * 1.4.4 does not reach it. It behaves differently under zoom and is measured
 * rather than assumed: at 1920px it is 48px, and at 200% zoom it is 32px
 * physical, because a pure `cqi` term shrinks with the container while
 * nothing font-relative grows to offset it. That is a gutter answering less
 * room with less inset, which is what it is for; the room the content itself
 * gets is SC 1.4.10's question, not this file's.
 */
const OFF_RAMP = ['--page-gutter'];

// The ends of the ramp and one ordinary window between them.
const AVAILABLE_SPACE = [320, 1440, 1920];

const MAX_BROWSER_ZOOM = 5;
const TOLERANCE = 0.01;

function probeIn(frame: HTMLElement): HTMLElement {
	const probe = document.createElement('span');
	probe.textContent = 'x';
	frame.append(probe);
	return probe;
}

/*
 * The physical size of `step`, in the units it has at 100% zoom, when a
 * window `space` device pixels wide is zoomed to `zoom`.
 */
function physicalSize(probe: HTMLElement, step: string, space: number, zoom: number): number {
	const frame = probe.parentElement!;
	frame.style.inlineSize = `${space / zoom}px`;
	probe.style.fontSize = `var(${step})`;
	// A computed `font-size` is always a `px` length, so this needs no unit parsing.
	return zoom * Number(getComputedStyle(probe).fontSize.replace('px', ''));
}

describe('every fluid step answers browser zoom', () => {
	it('checks every fluid value except the one recorded as off the ramp', () => {
		/* A regex that matched nothing would make every assertion below
		   vacuous, and a new fluid value written in some other shape would
		   silently escape the check rather than being excused by name. */
		expect(fluidSteps.length).toBeGreaterThan(10);
		const everyFluidValue = tokensCss.matchAll(ANY_FLUID).map(([, name]) => name).toArray();
		expect(everyFluidValue.filter((name) => !fluidSteps.includes(name))).toStrictEqual(OFF_RAMP);
	});

	const frame = document.createElement('div');
	frame.style.containerType = 'inline-size';
	document.body.append(frame);
	const probe = probeIn(frame);

	for (const space of AVAILABLE_SPACE) {
		it.each(fluidSteps)(`%s reaches 200% within 500% zoom, given ${space}px`, (step) => {
			const base = physicalSize(probe, step, space, 1);
			const reached = physicalSize(probe, step, space, MAX_BROWSER_ZOOM);
			expect(reached / base).toBeGreaterThanOrEqual(2);
		});

		it.each(fluidSteps)(`%s never shrinks under zoom, given ${space}px`, (step) => {
			const base = physicalSize(probe, step, space, 1);
			for (let zoom = 1.1; zoom <= MAX_BROWSER_ZOOM + TOLERANCE; zoom += 0.1) {
				const size = physicalSize(probe, step, space, zoom);
				expect(size, `${step} at ${Math.round(zoom * 100)}% zoom`).toBeGreaterThanOrEqual(
					base - TOLERANCE
				);
			}
		});
	}
});
