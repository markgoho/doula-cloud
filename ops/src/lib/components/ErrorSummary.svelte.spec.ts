import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import ErrorSummary from './ErrorSummary.svelte';

describe('ErrorSummary', () => {
	it('shows nothing when there is no error', async () => {
		render(ErrorSummary, { errors: [] });

		await expect.element(page.getByRole('alert')).not.toBeInTheDocument();
	});

	it('links each error to its field, shows one with no field as text, and takes focus', async () => {
		render(ErrorSummary, {
			errors: [{ message: 'Enter your email address', field: 'email' }, { message: 'Sorry, there is a problem' }]
		});

		const summary = page.getByRole('alert');
		await expect.element(summary).toBeVisible();
		await expect.element(summary).toHaveFocus();
		await expect.element(page.getByRole('link', { name: 'Enter your email address' })).toHaveAttribute('href', '#email');
		await expect.element(page.getByText('Sorry, there is a problem')).toBeVisible();
	});
});
