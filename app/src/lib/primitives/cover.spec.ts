/*
 * The render half of #1220's answer for `cover-l` (ADR-0052), in the
 * shape `stack.spec.ts` set for Stack. `primitives.usage.spec.ts` is the
 * source half: it refuses a child margin in Cover's rules, which this file
 * cannot see; this file measures the geometry, which that one cannot.
 *
 * Cover used to space its children with `cover-l > * { margin-block }`
 * in `@layer utilities`. A child that reset its own margin in `@layer
 * components` -- the ordinary `margin: 0` on a component's root element --
 * won the cascade outright and lost its half of that spacing. Cover now
 * spaces with `gap` and keeps only the centered child's `margin-block:
 * auto`, which is alignment rather than spacing.
 *
 * `space` is measured, never written down: every `--space-N` is a
 * `clamp()` carrying a `cqi`, so the cover's own computed padding -- the
 * same `space`, resolved against the same container -- is the honest
 * source for what it is at each width.
 *
 * The three widths are #1220's acceptance criteria, where #1105 measured
 * Stack's defect. They are not breakpoints: `space` changes with width,
 * and the claim is that the geometry holds wherever the clamp lands. The
 * continuum check (ADR-0025) owns overflow; this file owns spacing.
 */
import { page } from 'vitest/browser';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { registerLayoutPrimitives } from './index.js';
import { mountForTest, removeMounted, resetMarginInComponentsLayer } from './layerFixture.js';
import '#lib/styles/app.css';

beforeAll(() => {
	if (!customElements.get('cover-l')) registerLayoutPrimitives();
});

afterEach(removeMounted);

function paragraph(className: string): HTMLElement {
	const element = document.createElement('p');
	element.className = className;
	element.textContent = className;
	return element;
}

// Two adjacent non-centered children above the centered `h1`, one below.
function mountCover() {
	const cover = document.createElement('cover-l');
	const first = paragraph('first');
	const second = paragraph('second');
	const heading = document.createElement('h1');
	heading.textContent = 'Centered';
	const last = paragraph('last');

	cover.append(first, second, heading, last);
	document.body.append(mountForTest(cover));

	return { cover, first, second, heading, last };
}

function top(element: Element): number {
	return element.getBoundingClientRect().top;
}

function bottom(element: Element): number {
	return element.getBoundingClientRect().bottom;
}

function space(cover: HTMLElement): number {
	return Number(getComputedStyle(cover).paddingBlockStart.replace('px', ''));
}

describe.each([1440, 480, 320])('cover-l at %ipx', (width) => {
	beforeAll(async () => {
		await page.viewport(width, 800);
	});

	it('puts one space between two adjacent non-centered children', () => {
		const { cover, first, second } = mountCover();

		expect(space(cover)).toBeGreaterThan(0);
		expect(top(second) - bottom(first)).toBeCloseTo(space(cover), 1);
	});

	// Measured against an otherwise identical cover whose child resets
	// nothing, so this fails whenever a reset changes the step at all --
	// under the old margins it halved it, which happened to equal `space`.
	it('gives a non-first child that sets margin: 0 the same space above it', () => {
		const reference = mountCover();
		const expected = top(reference.second) - bottom(reference.first);
		reference.cover.remove();

		resetMarginInComponentsLayer('cover-l > .second');
		const { first, second } = mountCover();

		expect(expected).toBeGreaterThan(0);
		expect(top(second) - bottom(first)).toBeCloseTo(expected, 1);
	});

	it('centers the centered child between its neighbors', () => {
		const { cover, second, heading, last } = mountCover();
		const above = top(heading) - bottom(second);
		const below = top(last) - bottom(heading);

		expect(above).toBeGreaterThan(space(cover));
		expect(above).toBeCloseTo(below, 1);
	});

	it('puts the first and last children against the padding, not a margin', () => {
		const { cover, first, last } = mountCover();

		expect(top(first) - top(cover)).toBeCloseTo(space(cover), 1);
		expect(bottom(cover) - bottom(last)).toBeCloseTo(space(cover), 1);
	});
});
