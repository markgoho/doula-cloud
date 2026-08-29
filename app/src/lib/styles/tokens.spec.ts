import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

/*
 * The contrast gate for `tokens.css` -- wayfinder #417.
 *
 * The design brief sets two floors and calls them non-negotiable: WCAG 2.2
 * SC 1.4.3 at 4.5:1 for body text, and SC 1.4.11 at 3:1 for the boundary of
 * a user-interface component. The brief then broke both, because a palette
 * written by eye cannot be checked by eye. This file is why #417's answer is
 * "verified" rather than "asserted": edit a value and the arithmetic runs.
 *
 * Note what SC 1.4.11 does NOT cover: a decorative divider is neither a
 * UI component nor a meaningful graphic, so `outline-variant` is exempt and
 * is deliberately absent from the 3:1 assertions. Requiring 3:1 there would
 * drag the hairline to a mid-grey and destroy the direction's "containers
 * are declared by an edge, never a fill or a shadow".
 */

const source = readFileSync(new URL('tokens.css', import.meta.url), 'utf8');

type Oklch = { l: number; c: number; h: number };

// Every `--name: value` pair inside the block opened by `selector`.
function block(selector: string): Map<string, string> {
	const start = source.indexOf(`${selector} {`);
	if (start === -1) throw new Error(`No block for selector: ${selector}`);
	const body = source.slice(start + selector.length + 2, source.indexOf('}', start));
	const declarations = new Map<string, string>();
	for (const [, name, value] of body.matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) {
		declarations.set(name, value.trim().replaceAll(/\s+/g, ' '));
	}
	return declarations;
}

function parseOklch(value: string): Oklch {
	const match = /^oklch\(\s*([\d.]+)%\s+([\d.]+)\s+([\d.]+)\s*\)$/.exec(value);
	if (!match) throw new Error(`Not a plain oklch() value: ${value}`);
	return { l: Number(match[1]) / 100, c: Number(match[2]), h: Number(match[3]) };
}

// OKLab -> linear sRGB, then gamma-encode and clamp to the sRGB gamut.
function toSrgb({ l, c, h }: Oklch): [number, number, number] {
	const hRad = (h * Math.PI) / 180;
	const a = c * Math.cos(hRad);
	const b = c * Math.sin(hRad);
	const long = (l + 0.3963377774 * a + 0.2158037573 * b) ** 3;
	const medium = (l - 0.1055613458 * a - 0.0638541728 * b) ** 3;
	const short = (l - 0.0894841775 * a - 1.291485548 * b) ** 3;

	const linear = [
		4.0767416621 * long - 3.3077115913 * medium + 0.2309699292 * short,
		-1.2684380046 * long + 2.6097574011 * medium - 0.3413193965 * short,
		-0.0041960863 * long - 0.7034186147 * medium + 1.707614701 * short
	];

	return linear.map((channel) => {
		const encoded =
			channel <= 0.0031308 ? 12.92 * channel : 1.055 * channel ** (1 / 2.4) - 0.055;
		return Math.min(1, Math.max(0, encoded));
	}) as [number, number, number];
}

// WCAG 2.x relative luminance, which is defined on gamma-encoded sRGB.
function luminance(value: string): number {
	const [r, g, b] = toSrgb(parseOklch(value)).map((channel) =>
		channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4
	);
	return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
	const [high, low] = [luminance(a), luminance(b)].toSorted((x, y) => y - x);
	return (high + 0.05) / (low + 0.05);
}

function colorNamesIn(declarations: Map<string, string>): string[] {
	return declarations
		.keys()
		.filter((name) => name.startsWith("--color-"))
		.toArray()
		.toSorted((a, b) => a.localeCompare(b));
}

const light = block(':root');
const darkBySystem = block(":root:not([data-theme='light'])");
const darkByOverride = block(":root[data-theme='dark']");

const themes = [
	['light', light],
	['dark (system preference)', darkBySystem],
	['dark (manual override)', darkByOverride]
] as const;

// Anything a person can end up reading text on top of.
const textGrounds = [
	'--color-surface-bright',
	'--color-surface',
	'--color-surface-container',
	'--color-surface-container-high',
	'--color-surface-container-highest'
];

const bodyTextRoles = [
	'--color-on-surface',
	'--color-on-surface-variant',
	'--color-on-surface-muted',
	'--color-primary',
	'--color-error',
	'--color-status',
	'--color-info',
	'--color-warning',
	'--color-neutral'
];

describe('tokens.css is authored in OKLCH only', () => {
	/* Checks declaration values, not the whole file: issue references like
	   #417 in a comment are valid hex characters and are not colors. */
	it.each(themes)('%s declares every --color-* as a plain oklch()', (_name, palette) => {
		const offenders = [...palette]
			.filter(([name]) => name.startsWith('--color-'))
			.filter(([, value]) => !/^oklch\([\d.]+% [\d.]+ [\d.]+\)$/.test(value));
		expect(offenders).toStrictEqual([]);
	});
});

describe('the two dark blocks stay in sync', () => {
	it('declares identical values, so a manual override matches the system theme', () => {
		expect([...darkByOverride]).toStrictEqual([...darkBySystem]);
	});

	it('covers exactly the color tokens the light theme declares', () => {
		expect(colorNamesIn(darkBySystem)).toStrictEqual(colorNamesIn(light));
	});

	it('varies color only -- type, spacing, shape and motion are declared once', () => {
		const nonColor = darkBySystem
			.keys()
			.filter((name) => !name.startsWith('--color-'))
			.toArray();
		expect(nonColor).toStrictEqual([]);
	});
});

describe.each(themes)('%s', (_name, palette) => {
	it('uses neither pure white nor pure black', () => {
		const extremes = [...palette]
			.filter(([name]) => name.startsWith('--color-'))
			.map(([name, value]) => [name, toSrgb(parseOklch(value))] as const)
			.filter(([, rgb]) => rgb.every((c) => c === 1) || rgb.every((c) => c === 0))
			.map(([name]) => name);
		expect(extremes).toStrictEqual([]);
	});

	it.each(bodyTextRoles)('%s reads at 4.5:1 on every surface (SC 1.4.3)', (role) => {
		for (const ground of textGrounds) {
			const ratio = contrast(palette.get(role)!, palette.get(ground)!);
			expect(ratio, `${role} on ${ground}`).toBeGreaterThanOrEqual(4.5);
		}
	});

	it('--color-outline bounds a control at 3:1 on every surface (SC 1.4.11)', () => {
		for (const ground of textGrounds) {
			const ratio = contrast(palette.get('--color-outline')!, palette.get(ground)!);
			expect(ratio, `outline on ${ground}`).toBeGreaterThanOrEqual(3);
		}
	});

	it.each([
		['--color-on-primary', '--color-primary'],
		['--color-on-primary', '--color-primary-hover'],
		['--color-on-error', '--color-error']
	])('%s reads at 4.5:1 on %s', (text, fill) => {
		expect(contrast(palette.get(text)!, palette.get(fill)!)).toBeGreaterThanOrEqual(4.5);
	});

	it('keeps a card visibly lifted off the page ground', () => {
		const card = palette.get('--color-surface-bright')!;
		const ground = palette.get('--color-surface')!;
		expect(luminance(card)).not.toBeCloseTo(luminance(ground), 3);
	});
});
