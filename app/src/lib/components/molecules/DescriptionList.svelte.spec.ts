import type { Component, ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { measureCap } from '../../../routes/style-guide/floor.js';
import { mountInFrame, overflowReport, sweep } from '../../../routes/style-guide/continuum.js';
import DescriptionList from './DescriptionList.svelte';
// #530's own two tests below measure --measure and the real webfont, both
// of which come from the app's global stylesheet -- the same import
// DataTable.svelte.spec.ts uses for the identical reason.
import '#lib/styles/app.css';

type SetupOptions = Partial<ComponentProps<typeof DescriptionList>>;

const defaultItems = [
	{ label: 'Status', value: 'Active' },
	{ label: 'Created', value: '1/1/2026' }
];

async function setup({ items = defaultItems, ...rest }: SetupOptions = {}) {
	const { container } = await render(DescriptionList, { items, ...rest });
	return { container };
}

// #530's own fixture: a Practice pastes a referral link into a free-text
// field, and a URL has no break opportunity a browser will take -- the
// same value the style guide's own demo (`description-list/+page.svelte`)
// measures against.
const REFERRAL_URL =
	'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake';

// The cap a measure-criterion column is judged against, resolved the same
// way `floor.svelte.spec.ts` resolves it -- a probe inside the frame so it
// inherits the pairing's re-resolved base size, rather than a literal
// copied from `tokens.css`.
function capProbe(frame: HTMLElement): HTMLElement {
	const probe = document.createElement('div');
	probe.style.inlineSize = 'var(--measure)';
	probe.style.fontSize = 'var(--text-body-size)';
	frame.append(probe);
	return probe;
}

describe('DescriptionList.svelte', () => {
	it('renders each item as a label/value pair', async () => {
		await setup();

		await expect.element(page.getByText('Status')).toBeVisible();
		await expect.element(page.getByText('Active')).toBeVisible();
		await expect.element(page.getByText('Created')).toBeVisible();
		await expect.element(page.getByText('1/1/2026')).toBeVisible();
	});

	it('renders as a semantic description list', async () => {
		const { container } = await setup();

		expect(container.querySelectorAll('dt')).toHaveLength(2);
		expect(container.querySelectorAll('dd')).toHaveLength(2);
	});

	/*
	 * #464: keyed on index rather than on the label, because a record can
	 * legitimately show the same label twice -- two phone numbers, two
	 * Practice-defined fields a Practice named alike -- and a duplicate key
	 * collapsed both rows into one silently.
	 */
	it('renders both rows when two items share a label', async () => {
		const { container } = await render(DescriptionList, {
			items: [
				{ label: 'Phone', value: '(585) 555 0142' },
				{ label: 'Phone', value: '(585) 555 0199' }
			]
		});

		expect(container.querySelectorAll('dt')).toHaveLength(2);
		expect([...container.querySelectorAll('dd')].map((node) => node.textContent)).toEqual([
			'(585) 555 0142',
			'(585) 555 0199'
		]);
	});

	/*
	 * #530: a value holding one long unbreakable run (a pasted URL) must
	 * never force this component's own frame -- and so never any ancestor,
	 * up to the page -- wider than the space it is given. Swept across the
	 * whole available-space continuum (ADR-0025) rather than asserted at
	 * one chosen width, because a content floor here is exactly the defect
	 * this ticket fixes.
	 */
	it('never needs more room than it is given, for a value with no break opportunity', async () => {
		const { run, frame, remove } = await mountInFrame(DescriptionList as Component, {
			items: [{ label: 'Referral', value: REFERRAL_URL }]
		});
		try {
			const found = sweep(frame, run.clientWidth);
			expect(found, found && overflowReport('DescriptionList', found)).toBeUndefined();
		} finally {
			remove();
		}
	});

	/*
	 * #530's other half: the value track carries a readable upper bound.
	 * Given ample room, the value cell must not grow past `--measure` --
	 * this repo's existing cap on a readable line, the same one DataTable
	 * applies to its own cells -- onto one very long line.
	 */
	it('caps the value column at --measure when given ample room', async () => {
		const { frame, remove } = await mountInFrame(DescriptionList as Component, {
			items: [{ label: 'Referral', value: REFERRAL_URL }]
		});
		try {
			frame.style.inlineSize = '2000px';
			void frame.offsetWidth;
			const probe = capProbe(frame);
			const dd = frame.querySelector('dd');
			expect(dd).not.toBeNull();
			const measurement = measureCap(2000, '--measure', dd!, probe);
			expect(
				measurement.reachedPx - measurement.capPx,
				`value column reached ${measurement.reachedPx}px, --measure is ${measurement.capPx}px`
			).toBeLessThanOrEqual(1);
		} finally {
			remove();
		}
	});
});
