import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import WarningText from './WarningText.svelte';

async function setup({ message = 'This cannot be undone.' }: { message?: string } = {}) {
	return await render(WarningText, { message });
}

describe('WarningText', () => {
	it('renders the message', async () => {
		await setup();

		await expect.element(page.getByText('This cannot be undone.')).toBeVisible();
	});

	it('keeps its icon decorative', async () => {
		const { container } = await setup();

		const icon = container.querySelector('svg');
		expect(icon).toHaveAttribute('aria-hidden', 'true');
	});

	it('carries "Warning" for a screen reader', async () => {
		await setup();

		await expect.element(page.getByText('Warning', { exact: true })).toBeInTheDocument();
	});
});
