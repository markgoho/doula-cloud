/*
 * #1851: `Link` and `Button` hold a 44px hit target (#1828, #1805). It
 * once replaced a flex item's own minimum -- its words -- so a control in
 * a row with too little room shrank to 44px and its words drew over the
 * next one's: the Staff bar's nav did (#1850), while the row reported that
 * it fit. Each control now keeps its words and the row overflows instead,
 * which the continuum and floor checks can see.
 *
 * Its own file rather than a block in each atom's spec, because it needs
 * the real tokens (`app.css`) and those atom specs are written against the
 * component's styles alone.
 */
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import '#lib/styles/app.css';
import { findInlineSpill, TOLERANCE } from '../../../routes/style-guide/continuum.js';
import ControlsInRow from './harness/ControlsInRow.svelte';

async function setup(kind: 'link' | 'button', label: string, variant?: 'nav' | 'primary') {
	const { container } = await render(ControlsInRow, { kind, label, inlineSize: '8rem', variant });
	// The row has no role of its own: it is the harness's layout box.
	return { row: container.querySelector<HTMLElement>('.row')! };
}

// How many lines a control's words were drawn on: one per line box of its
// text. A word broken mid-letter adds a line the words alone would not.
function linesOf(control: Element): number {
	const tops = new Set<number>();
	const text = document.createTreeWalker(control, NodeFilter.SHOW_TEXT);
	while (text.nextNode()) {
		const range = document.createRange();
		range.selectNodeContents(text.currentNode);
		for (const rect of range.getClientRects()) tops.add(Math.round(rect.top));
	}
	return tops.size;
}

describe('a link whose words wrap, in a row with too little room for it (#1851)', () => {
	it('wraps between its words and never inside one', async () => {
		const { row } = await setup('link', 'Reassign everything', 'primary');

		const link = page.getByRole('link').first();
		await expect.element(link).toBeVisible();
		expect(linesOf(link.element())).toBe(2);
		expect(findInlineSpill(row)).toBeUndefined();
	});
});

describe.each([
	['link', 'A section with a long name'],
	['button', 'Send everything']
] as const)('a %s in a row with too little room for it (#1851)', (kind, label) => {
	it('never draws its words outside its own box', async () => {
		const { row } = await setup(kind, label);

		await expect.element(page.getByRole(kind).first()).toBeVisible();
		expect(findInlineSpill(row)).toBeUndefined();
		expect(row.scrollWidth).toBeGreaterThan(row.clientWidth);
	});

	it('still takes the 44px hit target with a label shorter than that', async () => {
		await setup(kind, 'A');

		const control = page.getByRole(kind).first();
		await expect.element(control).toBeVisible();
		// The floor is 44px less the control's own padding, both fluid, so
		// the sum lands within the browser's sub-pixel rounding of 44.
		expect(control.element().getBoundingClientRect().width).toBeGreaterThanOrEqual(44 - TOLERANCE);
	});
});
