import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import WarningText from './WarningText.svelte';

describe('WarningText', () => {
	it('renders the message', async () => {
		await render(WarningText, { message: 'This cannot be undone.' });

		await expect.element(page.getByText('This cannot be undone.')).toBeVisible();
	});

	it('keeps its icon decorative', async () => {
		const { container } = await render(WarningText, { message: 'This cannot be undone.' });

		const icon = container.querySelector('svg');
		expect(icon).toHaveAttribute('aria-hidden', 'true');
	});

	it('carries "Warning" for a screen reader', async () => {
		await render(WarningText, { message: 'This cannot be undone.' });

		await expect.element(page.getByText('Warning', { exact: true })).toBeInTheDocument();
	});
});
