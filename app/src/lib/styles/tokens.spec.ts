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

// The `color-scheme` value declared directly inside the block opened by
// `selector`. Not a custom property, so `block()`'s `--name: value` regex
// does not see it -- this is why the sync test above cannot catch it.
function colorScheme(selector: string): string {
	const start = source.indexOf(`${selector} {`);
	if (start === -1) throw new Error(`No block for selector: ${selector}`);
	const body = source.slice(start, source.indexOf('}', start));
	const match = /\bcolor-scheme\s*:\s*(\w+)\s*;/.exec(body);
	if (!match) throw new Error(`No color-scheme declared in ${selector}`);
	return match[1];
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

/*
 * `color-scheme` -- #438. Browser-rendered UI (scrollbars, the canvas
 * outside the document, default form-control chrome, spellcheck underlines)
 * reads this property, not our custom properties, so it needs its own
 * assertion: `block()`'s regex only ever sees `--name: value` pairs, and the
 * "two dark blocks stay in sync" test above cannot see a bare `color-scheme`
 * declaration at all -- a mismatch there would ship silently without this.
 */
describe('color-scheme tracks the resolved theme, not just the system preference', () => {
	it('is light by default', () => {
		expect(colorScheme(':root')).toBe('light');
	});

	it('is dark for the system-preference block', () => {
		expect(colorScheme(":root:not([data-theme='light'])")).toBe('dark');
	});

	it('is dark for the manual-override block, matching the system-preference block', () => {
		expect(colorScheme(":root[data-theme='dark']")).toBe('dark');
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

/*
 * The fluid scale -- #531, written in #540.
 *
 * Every type size and every spacing step is a `clamp()` that climbs the
 * ramp. The floor is the design decision and the only free number here;
 * everything else is arithmetic, so this re-derives it. A literal edited by
 * hand in `tokens.css` -- a ceiling nudged, a slope rounded, a step left
 * static when the rest moved -- fails here rather than shipping.
 *
 * `rem` on both ends of the clamp is deliberate: a person's own font-size
 * setting still moves the floor and the ceiling (WCAG 1.4.4). The growth
 * term is the one part that is not font-relative, because it is a share of
 * available space.
 */

function rem(value: string): number {
	const match = /^(-?[\d.]+)rem$/.exec(value);
	if (!match) throw new Error(`Not a plain rem length: ${value}`);
	return Number(match[1]);
}

const rampMin = rem(light.get('--ramp-min')!);
const rampMax = rem(light.get('--ramp-max')!);

type FluidStep = { floor: number; intercept: number; coefficient: number; ceiling: number };

function parseFluidStep(name: string): FluidStep {
	const value = light.get(name);
	if (value === undefined) throw new Error(`No such token: ${name}`);
	const match = /^clamp\((-?[\d.]+)rem, (-?[\d.]+)rem \+ (-?[\d.]+)cqi, (-?[\d.]+)rem\)$/.exec(
		value
	);
	if (!match) throw new Error(`${name} is not a fluid step on the ramp: ${value}`);
	const [, floor, intercept, coefficient, ceiling] = match;
	return {
		floor: Number(floor),
		intercept: Number(intercept),
		coefficient: Number(coefficient),
		ceiling: Number(ceiling)
	};
}

// Every token the scale covers, with the growth factor its scale was given.
const typeSteps = [
	'--text-display-size',
	'--text-heading-lg-size',
	'--text-heading-size',
	'--text-subheading-size',
	'--text-body-size',
	'--text-body-sm-size',
	'--text-label-size',
	'--text-meta-size'
];
const spacingSteps = [
	'--space-1',
	'--space-2',
	'--space-3',
	'--space-4',
	'--space-5',
	'--space-6',
	'--space-7',
	'--space-8',
	'--space-10',
	'--space-12'
];

const TYPE_GROWTH = 1.2;
const SPACING_GROWTH = 1.5;

const fluidSteps = [
	...typeSteps.map((name) => [name, TYPE_GROWTH] as const),
	...spacingSteps.map((name) => [name, SPACING_GROWTH] as const)
];

describe('the ramp is declared once', () => {
	it('runs 320px to 1920px, and both ends are font-relative', () => {
		expect(rampMin).toBe(20);
		expect(rampMax).toBe(120);
	});

	it('writes those two numbers nowhere else -- every step derives from them', () => {
		/* A step that spelled out 320 or 1920 would be a second copy of the
		   ramp, and the two copies would drift the first time one moved. */
		const offenders = fluidSteps
			.map(([name]) => [name, light.get(name)!] as const)
			.filter(([, value]) => /\b(320|1920|20rem|120rem)\b/.test(value))
			.map(([name]) => name);
		expect(offenders).toStrictEqual([]);
	});
});

describe('every size and every space is a fluid step', () => {
	it('leaves no --text-*-size or --space-* static', () => {
		const covered = new Set(fluidSteps.map(([name]) => name));
		const uncovered = [...light]
			.filter(([name]) => /^--text-[\w-]+-size$/.test(name) || /^--space-\d+$/.test(name))
			.filter(([name]) => !covered.has(name))
			.map(([name]) => name);
		expect(uncovered).toStrictEqual([]);
	});

	it.each(fluidSteps)('%s climbs the ramp from its floor at the scale growth factor', (name, growth) => {
		const { floor, intercept, coefficient, ceiling } = parseFluidStep(name);

		// The ceiling is the floor times the growth factor, and nothing else.
		expect(ceiling, `${name} ceiling`).toBeCloseTo(floor * growth, 6);

		/* 100cqi is the container's inline size, so the preferred term is
		   `intercept + slope x 100cqi` with slope = (ceiling - floor) / span.
		   The span is 100rem, which makes the cqi coefficient exactly the
		   distance the step travels. */
		const span = rampMax - rampMin;
		const slope = (ceiling - floor) / span;
		expect(coefficient, `${name} growth term`).toBeCloseTo(slope * 100, 6);
		expect(intercept, `${name} intercept`).toBeCloseTo(floor - slope * rampMin, 6);
	});

	/* The 2.5x ceiling, and what it does and does not prove -- #545.
	 *
	 * WCAG 1.4.4 asks that text be *able* to reach 200%, not that the
	 * browser's 200% setting render it: "It is not required to achieve 200%
	 * text enlargement while remaining inside a specific breakpoint ... but
	 * it should still be possible to get 200% text enlargement in some way"
	 * (Understanding SC 1.4.4, Intent). Barvian's 2.5x rule is derived from
	 * the far end of the range: browsers zoom to 500%, deep zoom narrows the
	 * container below --ramp-min so every step sits on its `rem` floor, and
	 * reaching twice a value that may be as large as the ceiling therefore
	 * needs a zoom of 2 x the growth factor. 2 x 2.5 = 5.
	 *
	 * So this guard proves 200% is *reachable*. It is not a claim about the
	 * 200% setting, where a fluid step renders 1.6x to 2.0x -- and no growth
	 * factor above 1 changes that, because the `cqi` term is an absolute
	 * share of a container zoom does not widen. `zoom.svelte.spec.ts`
	 * asserts the reachability itself in a browser; this is its cheap
	 * arithmetic companion, and the one that catches a hand-edited literal.
	 *
	 * The bound covers spacing as well as type, but 1.4.4 is about text:
	 * spacing is held to the same ratio as this repo's own consistency rule,
	 * so a step and the room around it stay in proportion under zoom. */
	it.each(fluidSteps)('%s keeps its ceiling within 2.5x its floor, so 200% stays reachable', (name) => {
		const { floor, ceiling } = parseFluidStep(name);
		expect(ceiling / floor).toBeLessThanOrEqual(2.5);
	});
});
