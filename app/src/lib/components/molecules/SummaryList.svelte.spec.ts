import { createRawSnippet, type Component } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { measureCap } from '../../../routes/style-guide/floor.js';
import { mountInFrame, overflowReport, sweep } from '../../../routes/style-guide/continuum.js';
import SummaryList, { type SummaryListRow } from './SummaryList.svelte';
// The continuum and cap tests below measure --measure, the container
// query and the real webfont, all of which come from the app's global
// stylesheet -- the same import DataTable.svelte.spec.ts uses.
import '#lib/styles/app.css';

// #530's fixture: a Practice pastes a referral link into a free-text
// field, and a URL has no break opportunity a browser will take.
const REFERRAL_URL = 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake';

// The cap a measure-criterion list is judged against, resolved the same
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

// Where a list's first value starts, in the window. A layout fact with no
// accessible signal (svelte-tests.md's querySelector exceptions).
function valueStart(root: HTMLElement): number {
	return root.querySelector(':scope dd')!.getBoundingClientRect().left;
}

const rows: SummaryListRow[] = [
	{ label: 'Care', value: 'Birth', changes: 'care' },
	{ label: 'Due', value: 'Nov 14, 2026' },
	{ label: 'Care phase', value: 'Active', changes: 'care phase' }
];

const changeLink = createRawSnippet<[SummaryListRow, string]>((row, describedBy) => ({
	render: () => `<a href="#${row().label}" aria-describedby="${describedBy()}">Change</a>`
}));

const detailText = createRawSnippet<[SummaryListRow]>((row) => ({
	render: () => `<p>Changing ${row().label}</p>`
}));

describe('SummaryList.svelte', () => {
	it('renders one row per item, label then value, in the order given', async () => {
		const { container } = await render(SummaryList, { props: { rows } });

		expect(container.querySelectorAll(':scope .row')).toHaveLength(3);
		expect([...container.querySelectorAll(':scope dt')].map((node) => node.textContent)).toEqual([
			'Care',
			'Due',
			'Care phase'
		]);
		await expect.element(page.getByText('Nov 14, 2026')).toBeVisible();
	});

	/*
	 * GOV.UK's Summary list: a row the reader may change carries the
	 * action, and a row she may not carries none -- no empty cell, so a
	 * screen reader does not meet a blank item.
	 */
	it('draws the action only on a row that names what it changes', async () => {
		const { container } = await render(SummaryList, { props: { rows, action: changeLink } });

		const actions = [...container.querySelectorAll(':scope .row')].map(
			(row) => row.querySelector(':scope .action a')?.textContent ?? ''
		);
		expect(actions).toEqual(['Change', '', 'Change']);
	});

	it('names what each action changes in visually-hidden text the action is described by', async () => {
		const { container } = await render(SummaryList, { props: { rows, action: changeLink } });

		const named = [...container.querySelectorAll(':scope .action a')].map((link) => {
			const describedBy = link.getAttribute('aria-describedby') ?? '';
			return container.querySelector(`#${CSS.escape(describedBy)}`)?.textContent;
		});
		expect(named).toEqual(['care', 'care phase']);
	});

	/*
	 * #2008: an answer to a question the Practice no longer asks. The
	 * drawing marks it under its label and says, where a Change would be,
	 * why there is none.
	 */
	it('marks a row with its tag on a line under the label, in that row', async () => {
		const { container } = await render(SummaryList, {
			props: { rows: [...rows, { label: 'Doula on call at booking', value: 'Priya Raman', tag: 'No longer collected' }] }
		});

		await expect.element(page.getByText('No longer collected')).toBeVisible();
		const notes = [...container.querySelectorAll(':scope .row')].map(
			(row) => row.querySelector(':scope .note')?.textContent?.trim() ?? ''
		);
		expect(notes).toEqual(['', '', '', 'No longer collected']);
	});

	it("draws a row's status, and no action, on a row nobody may change", async () => {
		const { container } = await render(SummaryList, {
			props: {
				rows: [
					{ ...rows[0]!, status: 'never drawn beside a Change' },
					{ label: 'Doula on call at booking', value: 'Priya Raman', status: 'Kept, not collected' }
				],
				action: changeLink
			}
		});

		await expect.element(page.getByText('Kept, not collected')).toBeVisible();
		await expect.element(page.getByText('never drawn beside a Change')).not.toBeInTheDocument();
		expect(container.querySelectorAll(':scope .row')[1]!.querySelector(':scope .action')).toBeNull();
		expect(page.getByRole('link', { name: 'Change' }).elements()).toHaveLength(1);
	});

	it("draws a row's status in a list with no action at all", async () => {
		await render(SummaryList, { props: { rows: [{ label: 'Old', value: 'kept', status: 'Kept, not collected' }] } });

		await expect.element(page.getByText('Kept, not collected')).toBeVisible();
	});

	it('draws no action cell at all without an action', async () => {
		const { container } = await render(SummaryList, { props: { rows } });

		expect(container.querySelectorAll(':scope .action')).toHaveLength(0);
	});

	it('shows a row its detail only while that row says so', async () => {
		const { container } = await render(SummaryList, {
			props: {
				rows: [rows[0]!, { ...rows[2]!, isDetailShown: true }],
				action: changeLink,
				detail: detailText
			}
		});

		await expect.element(page.getByText('Changing Care phase')).toBeVisible();
		expect(container.querySelectorAll(':scope .detail')).toHaveLength(1);
	});

	/*
	 * #464: keyed on index rather than on the label, because a record can
	 * legitimately show the same label twice, and a duplicate key collapsed
	 * both rows into one silently.
	 */
	it('renders both rows when two share a label', async () => {
		const { container } = await render(SummaryList, {
			props: {
				rows: [
					{ label: 'Phone', value: '(585) 555 0142' },
					{ label: 'Phone', value: '(585) 555 0199' }
				]
			}
		});

		expect([...container.querySelectorAll(':scope dd')].map((node) => node.textContent?.trim())).toEqual([
			'(585) 555 0142',
			'(585) 555 0199'
		]);
	});

	it('draws a row its value through the value snippet when given one', async () => {
		const linkValue = createRawSnippet<[SummaryListRow, number]>((row, index) => ({
			render: () => `<a href="#row-${index()}">${row().value}</a>`
		}));
		await render(SummaryList, { props: { rows, value: linkValue } });

		await expect.element(page.getByRole('link', { name: 'Nov 14, 2026' })).toHaveAttribute('href', '#row-1');
	});

	/*
	 * #2021: the values of two lists of the same width start on the same
	 * line, whatever each list's longest label is -- what lines up a check
	 * page's sections, or a Client hub's four lists.
	 */
	it('starts the values of two lists on the same line whatever their labels', async () => {
		const { container } = await render(SummaryList, {
			props: { rows: [{ label: 'Due', value: 'Nov 14, 2026' }] }
		});
		const { container: longer } = await render(SummaryList, {
			props: { rows: [{ label: 'Preferred name', value: 'Ada' }] }
		});

		expect(valueStart(container)).toBe(valueStart(longer));
	});

	it('puts every key above its value when asked to stack', async () => {
		const { container } = await render(SummaryList, {
			props: { rows, isStacked: true, action: changeLink }
		});

		const row = container.querySelector(':scope .row')!;
		const key = row.querySelector('dt')!.getBoundingClientRect();
		const value = row.querySelector('.value')!.getBoundingClientRect();
		const change = row.querySelector('.action')!.getBoundingClientRect();
		expect(value.top).toBeGreaterThanOrEqual(key.bottom);
		expect(value.left).toBe(key.left);
		// The Change stays on its own row, under the value, not the next row's.
		expect(change.top).toBeGreaterThanOrEqual(value.top);
		expect(change.bottom).toBeLessThanOrEqual(row.getBoundingClientRect().bottom);
	});

	/*
	 * Where the key and the value cannot share a line, a list stacks by
	 * itself -- read from its own width, not the window's (ADR-0024).
	 */
	it('stacks by itself in a narrow space and sets key beside value in a wide one', async () => {
		const { frame, remove } = await mountInFrame(SummaryList as unknown as Component, {
			rows
		});
		try {
			const placement = () => {
				const row = frame.querySelector('.row')!;
				const key = row.querySelector('dt')!.getBoundingClientRect();
				const value = row.querySelector('.value')!.getBoundingClientRect();
				return value.top >= key.bottom ? 'stacked' : 'beside';
			};
			frame.style.inlineSize = '288px';
			void frame.offsetWidth;
			expect(placement()).toBe('stacked');
			frame.style.inlineSize = '720px';
			void frame.offsetWidth;
			expect(placement()).toBe('beside');
		} finally {
			remove();
		}
	});

	/*
	 * #530: a value or a label holding one long unbreakable run (a pasted
	 * URL, #725) must never force this list's frame -- and so never any
	 * ancestor, up to the page -- wider than the space it is given. Swept
	 * across the whole available-space continuum (ADR-0025).
	 */
	it('never needs more room than it is given, for a value or label with no break opportunity', async () => {
		const { run, frame, remove } = await mountInFrame(SummaryList as unknown as Component, {
			rows: [
				{ label: 'Referral', value: REFERRAL_URL },
				{
					label: REFERRAL_URL,
					value: 'Epidural, as early as it can be given'
				}
			]
		});
		try {
			const found = sweep(frame, run.clientWidth);
			expect(found, found && overflowReport('SummaryList', found)).toBeUndefined();
		} finally {
			remove();
		}
	});

	/*
	 * #2021: the list, not the value, carries the measure, so every list
	 * on a page ends at one right edge.
	 */
	it('caps the list at --measure when given ample room', async () => {
		const { frame, remove } = await mountInFrame(SummaryList as unknown as Component, {
			rows: [{ label: 'Referral', value: REFERRAL_URL }]
		});
		try {
			frame.style.inlineSize = '2000px';
			void frame.offsetWidth;
			const probe = capProbe(frame);
			const list = frame.querySelector('dl');
			expect(list).not.toBeNull();
			const measurement = measureCap(2000, '--measure', list!, probe);
			expect(
				measurement.reachedPx - measurement.capPx,
				`list reached ${measurement.reachedPx}px, --measure is ${measurement.capPx}px`
			).toBeLessThanOrEqual(1);
		} finally {
			remove();
		}
	});

	it('draws a bordered card only when asked', async () => {
		const { container } = await render(SummaryList, { props: { rows, isBordered: true } });
		expect(container.querySelector('dl')).toHaveClass('bordered');

		const { container: plain } = await render(SummaryList, { props: { rows } });
		expect(plain.querySelector('dl')).not.toHaveClass('bordered');
	});
});
