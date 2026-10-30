import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ContractForm from './ContractForm.svelte';

interface SetupOptions {
	mergeFields?: string[];
	values?: Record<string, string>;
	readOnly?: boolean;
}

const defaultMergeFields = ['client_name', 'price'];

async function setup({ mergeFields = defaultMergeFields, values = {}, readOnly = false }: SetupOptions = {}) {
	const onValueChange = vi.fn();

	await render(ContractForm, { mergeFields, values, readOnly, onValueChange });

	return { onValueChange };
}

describe('ContractForm.svelte', () => {
	it('renders a labeled text input for each merge field', async () => {
		await setup();

		await expect.element(page.getByLabelText('Client name')).toBeInTheDocument();
		await expect.element(page.getByLabelText('Price')).toBeInTheDocument();
	});

	it('renders the stored value for a merge field', async () => {
		await setup({ values: { client_name: 'Jamie' } });

		await expect.element(page.getByLabelText('Client name')).toHaveValue('Jamie');
	});

	it('renders an empty input when a merge field has no stored value yet', async () => {
		await setup();

		await expect.element(page.getByLabelText('Price')).toHaveValue('');
	});

	it('falls back to the raw key as the label for an unknown merge field', async () => {
		await setup({ mergeFields: ['some_ad_hoc_token'] });

		await expect.element(page.getByLabelText('some_ad_hoc_token')).toBeInTheDocument();
	});

	it('calls onValueChange with the field key and new value when an input changes', async () => {
		const { onValueChange } = await setup();

		await page.getByLabelText('Client name').fill('Alex');

		expect(onValueChange).toHaveBeenCalledWith('client_name', 'Alex');
	});

	// #2222: a date field asks with a date control, so what is stored is a
	// YYYY-MM-DD day that fillProse can print as a calendar day.
	it('asks for each date field with a date control, and reports the day it holds', async () => {
		const { onValueChange } = await setup({
			mergeFields: ['engagement_start_date', 'engagement_end_date', 'price'],
			values: { engagement_start_date: '2026-10-27' }
		});

		await expect.element(page.getByLabelText('Engagement start date')).toHaveAttribute('type', 'date');
		await expect.element(page.getByLabelText('Engagement start date')).toHaveValue('2026-10-27');
		await expect.element(page.getByLabelText('Engagement end date')).toHaveAttribute('type', 'date');
		await expect.element(page.getByLabelText('Price')).toHaveAttribute('type', 'text');

		await page.getByLabelText('Engagement end date').fill('2027-04-05');

		expect(onValueChange).toHaveBeenCalledWith('engagement_end_date', '2027-04-05');
	});

	it('disables every input when readOnly is true', async () => {
		await setup({ readOnly: true });

		await expect.element(page.getByLabelText('Client name')).toBeDisabled();
		await expect.element(page.getByLabelText('Price')).toBeDisabled();
	});
});
