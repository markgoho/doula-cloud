/*
 * The render half of #1105's gate (ADR-0039). `primitives.usage.spec.ts`
 * is the other half: it reads the source and refuses the mechanism that
 * caused this, while this file renders the defect itself and measures it.
 *
 * The defect a source gate cannot see: `stack-l` used to space its
 * children with `> * + * { margin-block-start }`, injected into `@layer
 * utilities`. `app.css` declares `utilities` before `components`, and
 * every component's scoped `<style>` is wrapped in `@layer components`, so
 * a component whose root element carried the ordinary `margin: 0` reset --
 * `Notice`, `Text`, `Heading`, `WarningText` and roughly twenty others --
 * won the cascade outright and the stack's spacing above it silently did
 * nothing. Specificity never entered into it, which is why nothing about
 * the selectors looks wrong.
 *
 * ## Why this measures geometry rather than a computed margin
 *
 * Under the fix, the margin `margin-block-start` reports on that child is
 * `0px` -- exactly what the bug report measured on `/signup`. The space is
 * the container's, not the child's, so the only honest question is where
 * the two boxes actually sit: the second child's top edge, less the
 * first's bottom edge, is the stack's `space`. That number is 0 under the
 * old mechanism and `space` under the new one, whichever layer the child's
 * own reset is written in.
 *
 * The real cascade is what makes this a test rather than a tautology, so
 * the fixture reproduces it exactly: `app.css` for the layer order and the
 * space tokens, and the child's reset written into `@layer components`,
 * where a component's own reset lives.
 */
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { registerLayoutPrimitives } from './index.js';
import '#lib/styles/app.css';

beforeAll(() => {
	if (!customElements.get('stack-l')) registerLayoutPrimitives();
});

const mounted: HTMLElement[] = [];

afterEach(() => {
	for (const element of mounted) element.remove();
	mounted.length = 0;
});

/*
 * A component's own root-element reset, in the layer a component's scoped
 * `<style>` compiles into. One stylesheet per test, removed with the
 * fixture it belongs to.
 */
function resetMarginInComponentsLayer(selector: string): void {
	const style = document.createElement('style');
	style.textContent = `@layer components {\n\t${selector} { margin: 0; }\n}`;
	document.head.append(style);
	mounted.push(style);
}

function mountStack(space?: string): { first: HTMLElement; second: HTMLElement } {
	const stack = document.createElement('stack-l');
	if (space) stack.setAttribute('space', space);

	const first = document.createElement('p');
	first.className = 'first';
	first.textContent = 'The field above';
	const second = document.createElement('p');
	second.className = 'second';
	second.textContent = 'The notice below it';

	stack.append(first, second);
	document.body.append(stack);
	mounted.push(stack);

	return { first, second };
}

function gapBetween(first: HTMLElement, second: HTMLElement): number {
	return second.getBoundingClientRect().top - first.getBoundingClientRect().bottom;
}

/*
 * The expected number is measured, never written down: every `--space-N`
 * is a `clamp()` carrying a `cqi`, so the only honest source for what the
 * space is here is a stack whose second child resets nothing. That
 * reference is also what makes this fail red on the old mechanism -- the
 * reference keeps its spacing while the reset one loses it, so the two
 * disagree exactly when the cancellation is back.
 */
function referenceGap(space?: string): number {
	const { first, second } = mountStack(space);
	return gapBetween(first, second);
}

describe('stack-l spaces a child that resets its own margin', () => {
	it('gives a non-first child the default space above it even when that child sets margin: 0', () => {
		const expected = referenceGap();
		resetMarginInComponentsLayer('.second');
		const { first, second } = mountStack();

		expect(expected).toBeGreaterThan(0);
		expect(gapBetween(first, second)).toBeCloseTo(expected, 1);
	});

	it('gives it the configured space when the stack asks for a non-default one', () => {
		const defaultGap = referenceGap();
		const expected = referenceGap('var(--space-7)');
		resetMarginInComponentsLayer('.second');
		const { first, second } = mountStack('var(--space-7)');

		expect(expected).toBeGreaterThan(defaultGap);
		expect(gapBetween(first, second)).toBeCloseTo(expected, 1);
	});

	// The first child is where the stack owes nothing, and a reset there
	// must not become spacing by accident.
	it('puts no space above the first child', () => {
		const { first } = mountStack();

		expect(first.getBoundingClientRect().top).toBeCloseTo(
			(first.parentElement as HTMLElement).getBoundingClientRect().top,
			1
		);
	});
});
