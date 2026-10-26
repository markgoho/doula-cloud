import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import FilterBar from './FilterBar.svelte';

// `createRawSnippet` renders one root element, so the field is one
// wrapper holding a label and its date control: what the bar needs from
// its children is only that they are named fields inside its form.
function fromField() {
	return createRawSnippet(() => ({
		render: () =>
			'<div><label for="from">From</label><input id="from" type="date" name="from" value="2026-10-26"></div>'
	}));
}

interface SetupOptions {
	empty?: boolean;
	isAppliedOnChange?: boolean;
	onSubmit?: (event: SubmitEvent) => void;
}

async function setup({
	empty,
	isAppliedOnChange,
	onSubmit = vi.fn((event: SubmitEvent) => event.preventDefault())
}: SetupOptions = {}) {
	await render(FilterBar, {
		action: '/practices/practice-1/schedule',
		onSubmit,
		legend: 'Narrow the schedule',
		status: 'Showing 2 scheduled visits.',
		empty,
		isAppliedOnChange,
		children: fromField()
	});
	return { onSubmit };
}

describe('FilterBar', () => {
	it('groups its fields under the legend', async () => {
		await setup();

		const group = page.getByRole('group', { name: 'Narrow the schedule' });
		await expect.element(group).toBeVisible();
		await expect.element(group.getByLabelText('From')).toHaveValue('2026-10-26');
	});

	// The fields are named and the form is a GET to the list's own URL, so
	// it narrows the list with no script running at all.
	it('is a GET form to the list’s own address', async () => {
		await setup();

		const form = page.getByRole('group', { name: 'Narrow the schedule' }).element().closest('form');
		expect(form).toHaveAttribute('method', 'get');
		expect(form).toHaveAttribute('action', '/practices/practice-1/schedule');
	});

	it('hands the submit to the route when Apply is pressed', async () => {
		const { onSubmit } = await setup();

		await page.getByRole('button', { name: 'Apply' }).click();

		expect(onSubmit).toHaveBeenCalledOnce();
	});

	// #2068: a bar whose control acts at once (a toggle) has no second press.
	it('draws no Apply when every control acts the moment it changes', async () => {
		await setup({ isAppliedOnChange: true });

		await expect.element(page.getByRole('group', { name: 'Narrow the schedule' })).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Apply' })).not.toBeInTheDocument();
	});

	it('says the result as its own status line', async () => {
		await setup();

		const status = page.getByRole('status');
		await expect.element(status).toBeVisible();
		await expect.element(status).toHaveTextContent('Showing 2 scheduled visits.');
	});

	// The list's empty state says it on screen (#2015); the line stays in
	// the document as a live region, so a screen reader still hears what
	// the new range found.
	it('keeps the status line for a screen reader only when the list is empty', async () => {
		await setup({ empty: true });

		await expect.element(page.getByRole('status')).toHaveClass('visually-hidden');
	});
});
