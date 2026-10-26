import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import SummaryList, { type SummaryListRow } from './SummaryList.svelte';

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

	it('draws a bordered card only when asked', async () => {
		const { container } = await render(SummaryList, { props: { rows, isBordered: true } });
		expect(container.querySelector('dl')).toHaveClass('bordered');

		const { container: plain } = await render(SummaryList, { props: { rows } });
		expect(plain.querySelector('dl')).not.toHaveClass('bordered');
	});
});
