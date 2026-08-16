import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ContractTemplateEditor from './ContractTemplateEditor.svelte';
import { MERGE_FIELDS } from './contractTemplate.js';

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
});
