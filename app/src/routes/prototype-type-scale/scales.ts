/*
 * PROTOTYPE (#1926) -- the five candidate type scales, and the one place
 * the prototype overrides the type tokens.
 *
 * Every candidate keeps body at 16px (#1722 Q5) and the same fluid growth
 * as tokens.css (#1722 Q6): a step's floor holds at a 320px container, its
 * ceiling is the floor times 1.2 at a 1920px container, and it climbs on
 * `cqi` between the two. Only the eight `--text-*-size` tokens move; the
 * weight, leading and tracking of each step stay as tokens.css has them.
 */

export const STEPS = [
	{ token: 'meta', use: 'Table timestamps, top bar Practice name' },
	{ token: 'label', use: 'Record page labels' },
	{ token: 'body-sm', use: 'Table rows, hints, errors, small buttons' },
	{ token: 'body', use: 'Body text, fields, buttons' },
	{ token: 'subheading', use: 'Card headings (h3), large buttons' },
	{ token: 'heading', use: 'Section headings (h2)' },
	{ token: 'heading-lg', use: 'Page titles (h1), the question' },
	{ token: 'display', use: 'The Practice overview title' }
] as const;

export type StepToken = (typeof STEPS)[number]['token'];

const GROWTH = 1.2;
const BODY = 16;
const BODY_INDEX = 3;

/**
Floors in px, smallest step first, in the order of STEPS.
*/
type Floors = readonly [number, number, number, number, number, number, number, number];

function round(px: number): number {
	return Math.round(px * 100) / 100;
}

/**
One ratio below body and one above it (the same ratio twice is a single-ratio scale).
*/
function fromRatios(below: number, above: number): Floors {
	return STEPS.map((_, index) => {
		const distance = index - BODY_INDEX;
		const ratio = distance < 0 ? below : above;
		return round(BODY * ratio ** distance);
	}) as unknown as Floors;
}

export interface Scale {
	key: string;
	name: string;
	note: string;
	floors: Floors;
}

export const SCALES: readonly Scale[] = [
	{
		key: 'hand',
		name: 'Hand-picked, fixed',
		note: "Today's steps with body moved to 16px and subheading moved up to 18px so the two stay distinct. No ratio.",
		floors: [12, 13, 14, 16, 18, 20, 28, 36]
	},
	{
		key: '1.125',
		name: 'One ratio, 1.125 (major second)',
		note: 'Every step is 1.125 times the one below it, from body at 16px.',
		floors: fromRatios(1.125, 1.125)
	},
	{
		key: '1.2',
		name: 'One ratio, 1.2 (minor third)',
		note: 'Every step is 1.2 times the one below it, from body at 16px.',
		floors: fromRatios(1.2, 1.2)
	},
	{
		key: '1.25',
		name: 'One ratio, 1.25 (major third)',
		note: 'Every step is 1.25 times the one below it, from body at 16px.',
		floors: fromRatios(1.25, 1.25)
	},
	{
		key: 'two',
		name: 'Two ratios, 1.067 below body and 1.25 above',
		note: 'A tight ratio for the dense steps under body, a larger one for the headings over it.',
		floors: fromRatios(1.067, 1.25)
	},
	{
		key: 'floor16',
		name: 'Nothing under 16px, 1.25 above',
		note: "The founder's pick to try: meta, label, body-sm and body all at 16px, the headings at 1.25 from there. Below body the roles differ only by weight and color, as tokens.css has them.",
		floors: [16, 16, 16, 16, 20, 25, 31.25, 39.06]
	}
];

/**
The candidate the page opens on.
*/
export const OPENING_KEY = 'floor16';

/**
 * The same arithmetic tokens.spec.ts holds tokens.css to: with the ramp
 * at 20rem to 120rem and a 1.2 growth factor, a floor F (in rem) is
 * `clamp(F, 0.96F + 0.2F cqi, 1.2F)`.
 */
function fixed(value: number): number {
	return Number(value.toFixed(4));
}

function fluid(px: number): string {
	const rem = px / 16;
	return `clamp(${fixed(rem)}rem, ${fixed(rem * 0.96)}rem + ${fixed(rem * 0.2)}cqi, ${fixed(rem * GROWTH)}rem)`;
}

/**
The inline style that overrides the eight size tokens for one scale.
*/
export function tokenStyle(scale: Scale): string {
	return STEPS.map((step, index) => `--text-${step.token}-size: ${fluid(scale.floors[index]!)}`).join(
		'; '
	);
}

/**
What a step renders at in a container this many px wide.
*/
export function sizeAt(floor: number, containerPx: number): number {
	return Math.min(Math.max(floor, 0.96 * floor + (floor * containerPx) / 8000), floor * GROWTH);
}

export function formatPx(px: number): string {
	return Number.isSafeInteger(px) ? `${px}` : px.toFixed(1);
}
