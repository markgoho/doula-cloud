import '#lib/styles/app.css';
import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Badge from './Badge.svelte';

type SetupOptions = Partial<ComponentProps<typeof Badge>>;

async function setup({ label = 'Active', variant = 'success', ...rest }: SetupOptions = {}) {
	const { container } = await render(Badge, { label, variant, ...rest });
	return { container };
}

const variants = ['info', 'success', 'warning', 'error', 'neutral'] as const;

/*
 * #434 -- Badge tints each variant's background with color-mix(), so the
 * regression (every variant converging on the same hue) only shows up in
 * the actual rendered pixel, never in the color-mix(...) source string. A
 * canvas samples the real, browser-resolved sRGB for whatever color syntax
 * getComputedStyle hands back (oklch, oklab, color-mix's own oklab(...)
 * serialization -- all of them), so these checks don't care which color
 * space produced the value, only what a reader would actually see.
 */
function pixelOf(cssColor: string): [number, number, number] {
	const canvas = document.createElement('canvas');
	canvas.width = 1;
	canvas.height = 1;
	const context = canvas.getContext('2d')!;
	context.fillStyle = cssColor;
	context.fillRect(0, 0, 1, 1);
	const [r, g, b] = context.getImageData(0, 0, 1, 1).data;
	return [r, g, b];
}

// Hue angle in degrees for an sRGB triplet. A grey pixel (no chroma) has no
// hue at all, which is exactly the failure mode #434 describes -- so this
// returns NaN rather than an arbitrary 0, and the test below fails loudly on it.
function hueOf([r, g, b]: [number, number, number]): number {
	const [rn, gn, bn] = [r, g, b].map((channel) => channel / 255);
	const max = Math.max(rn, gn, bn);
	const min = Math.min(rn, gn, bn);
	const delta = max - min;
	if (delta === 0) return NaN;

	let raw: number;
	if (max === rn) raw = ((gn - bn) / delta) % 6;
	else if (max === gn) raw = (bn - rn) / delta + 2;
	else raw = (rn - gn) / delta + 4;

	const degrees = raw * 60;
	return degrees < 0 ? degrees + 360 : degrees;
}

function circularHueDistance(a: number, b: number): number {
	const diff = Math.abs(a - b) % 360;
	return Math.min(diff, 360 - diff);
}

// Every unordered pair of variants, with the hue gap between them -- pulled
// out of the test body so the pairwise loop has nowhere to `continue` from.
function pairwiseHueGaps(hues: Map<string, number>) {
	const entries = [...hues];
	const gaps: { a: string; b: string; gap: number }[] = [];
	for (let index = 0; index < entries.length; index++) {
		for (let other = index + 1; other < entries.length; other++) {
			const [a, hueA] = entries[index];
			const [b, hueB] = entries[other];
			gaps.push({ a, b, gap: circularHueDistance(hueA, hueB) });
		}
	}
	return gaps;
}

// WCAG 2.x relative luminance and contrast ratio, defined on gamma-encoded sRGB.
function luminance([r, g, b]: [number, number, number]): number {
	const [rl, gl, bl] = [r, g, b]
		.map((channel) => channel / 255)
		.map((channel) => (channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4));
	return 0.2126 * rl + 0.7152 * gl + 0.0722 * bl;
}

function contrast(a: [number, number, number], b: [number, number, number]): number {
	const [high, low] = [luminance(a), luminance(b)].toSorted((x, y) => y - x);
	return (high + 0.05) / (low + 0.05);
}

async function withTheme<T>(theme: 'light' | 'dark', run: () => Promise<T>): Promise<T> {
	if (theme === 'dark') document.documentElement.dataset.theme = 'dark';
	try {
		return await run();
	} finally {
		delete document.documentElement.dataset.theme;
	}
}

describe('Badge.svelte', () => {
	it('renders the label as visible text', async () => {
		await setup({ label: 'Active' });

		await expect.element(page.getByText('Active')).toBeVisible();
	});

	it('applies a variant-matching class for each variant', async () => {
		for (const variant of variants) {
			const { container } = await setup({ variant });

			expect(container.querySelector(`span.${variant}`)).toBeInTheDocument();
		}
	});

	it('renders a decorative icon for each variant', async () => {
		for (const variant of variants) {
			const { container } = await setup({ variant });

			const svg = container.querySelector('svg');
			expect(svg).toBeInTheDocument();
			expect(svg).toHaveAttribute('aria-hidden', 'true');
		}
	});
});

/*
 * #434 -- the background tint has to carry variant signal on its own, not
 * just the border/icon/text. See tokens.spec.ts for why color-mix(in
 * oklch, ...) failed this: a 12% mix's hue lands on the shorter polar arc
 * toward --color-surface, dragging every variant to nearly the same hue.
 */
describe('the five variants read as visibly different background tints (#434)', () => {
	it.each(['light', 'dark'] as const)(
		'every pair of variants clears a 15-degree hue gap in the %s theme',
		async (theme) => {
			const hues = await withTheme(theme, async () => {
				const result = new Map<string, number>();
				for (const variant of variants) {
					const { container } = await setup({ variant });
					const bg = getComputedStyle(container.querySelector(`span.${variant}`)!).backgroundColor;
					result.set(variant, hueOf(pixelOf(bg)));
				}
				return result;
			});

			for (const [name, hue] of hues) expect(hue, `${name} has a real hue, not grey`).not.toBeNaN();

			for (const { a, b, gap } of pairwiseHueGaps(hues)) {
				expect(gap, `${a} vs ${b}`).toBeGreaterThanOrEqual(15);
			}
		}
	);
});

/*
 * The contrast floors tokens.spec.ts enforces on tokens.css in isolation --
 * WCAG SC 1.4.3 at 4.5:1 for text, SC 1.4.11 at 3:1 for a UI-component
 * boundary -- checked here against the actual mixed background each
 * variant renders, not the underlying token values.
 */
describe('text and border keep their contrast floor on the tinted background (#434)', () => {
	it.each(['light', 'dark'] as const)('holds for every variant in the %s theme', async (theme) => {
		await withTheme(theme, async () => {
			for (const variant of variants) {
				const { container } = await setup({ variant });
				const style = getComputedStyle(container.querySelector(`span.${variant}`)!);
				const background = pixelOf(style.backgroundColor);
				const text = pixelOf(style.color);
				const border = pixelOf(style.borderColor);

				expect(contrast(text, background), `${variant} text on its tint`).toBeGreaterThanOrEqual(4.5);
				expect(contrast(border, background), `${variant} border on its tint`).toBeGreaterThanOrEqual(3);
			}
		});
	});
});
