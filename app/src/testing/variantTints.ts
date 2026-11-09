import { expect } from 'vitest';

export type Rgb = [number, number, number];
export type Theme = 'light' | 'dark';

export const themes: readonly Theme[] = ['light', 'dark'];

// The real, browser-resolved sRGB of any CSS color syntax getComputedStyle hands back.
export function pixelOf(cssColor: string): Rgb {
	const canvas = document.createElement('canvas');
	canvas.width = 1;
	canvas.height = 1;
	const context = canvas.getContext('2d')!;
	context.fillStyle = cssColor;
	context.fillRect(0, 0, 1, 1);
	const [r, g, b] = context.getImageData(0, 0, 1, 1).data;
	return [r, g, b];
}

// A gray pixel has no hue, so this returns NaN rather than an arbitrary 0.
function hueOf([r, g, b]: Rgb): number {
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
function luminance([r, g, b]: Rgb): number {
	const [rl, gl, bl] = [r, g, b]
		.map((channel) => channel / 255)
		.map((channel) => (channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4));
	return 0.2126 * rl + 0.7152 * gl + 0.0722 * bl;
}

export function contrast(a: Rgb, b: Rgb): number {
	const [high, low] = [luminance(a), luminance(b)].toSorted((x, y) => y - x);
	return (high + 0.05) / (low + 0.05);
}

export async function withTheme<T>(theme: Theme, run: () => Promise<T>): Promise<T> {
	if (theme === 'dark') document.documentElement.dataset.theme = 'dark';
	try {
		return await run();
	} finally {
		delete document.documentElement.dataset.theme;
	}
}

// #434: every variant's background tint has a real hue, and every pair sits at least 15 degrees apart.
export async function expectDistinctTintHues<V extends string>(
	theme: Theme,
	variants: readonly V[],
	backgroundOf: (variant: V) => Promise<string>
) {
	const hues = await withTheme(theme, async () => {
		const result = new Map<string, number>();
		for (const variant of variants) {
			const background = await backgroundOf(variant);
			result.set(variant, hueOf(pixelOf(background)));
		}
		return result;
	});

	for (const [name, hue] of hues) expect(hue, `${name} has a real hue, not gray`).not.toBeNaN();

	for (const { a, b, gap } of pairwiseHueGaps(hues)) {
		expect(gap, `${a} vs ${b}`).toBeGreaterThanOrEqual(15);
	}
}
