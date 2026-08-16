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

	it('disables every input when readOnly is true', async () => {
		await setup({ readOnly: true });

		await expect.element(page.getByLabelText('Client name')).toBeDisabled();
		await expect.element(page.getByLabelText('Price')).toBeDisabled();
	});
});
