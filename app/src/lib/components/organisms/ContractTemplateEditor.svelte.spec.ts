import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ContractTemplateEditor from './ContractTemplateEditor.svelte';
import { MERGE_FIELDS } from '#lib/contractTemplate.js';

interface SetupOptions {
	prose?: string;
}

async function setup({ prose = 'Agreement with {{client_name}}' }: SetupOptions = {}) {
	const onProseChange = vi.fn();
	await render(ContractTemplateEditor, { prose, onProseChange });
	return { onProseChange };
}

describe('ContractTemplateEditor.svelte', () => {
	it('renders the prose in the textarea', async () => {
		await setup({ prose: 'Some prose' });

		await expect.element(page.getByLabelText('Contract template prose')).toHaveValue('Some prose');
	});

	it('calls onProseChange when the textarea changes', async () => {
		const { onProseChange } = await setup({ prose: '' });

		await page.getByLabelText('Contract template prose').fill('New prose');

		expect(onProseChange).toHaveBeenCalledWith('New prose');
	});

	it('renders every merge-field placeholder token', async () => {
		await setup();

		for (const field of MERGE_FIELDS) {
			await expect.element(page.getByText(field.token)).toBeVisible();
		}
	});

	/*
	 * #2244: with no way to change the prose there is no field to type
	 * into. A reader gets the terms as text, and the tokens in them still
	 * explained.
	 */
	it('gives a reader the prose as text, with no field, and the tokens explained', async () => {
		await render(ContractTemplateEditor, { prose: 'Agreement with {{client_name}}.\n\nPaid by {{price}}.' });

		await expect.element(page.getByText('Agreement with {{client_name}}.', { exact: false })).toBeVisible();
		expect(page.getByRole('textbox').query()).toBeNull();
		await expect.element(page.getByText('Each Contract fills these in for its Client.')).toBeVisible();
		for (const field of MERGE_FIELDS) {
			await expect.element(page.getByText(field.token, { exact: true })).toBeVisible();
		}
	});
});
