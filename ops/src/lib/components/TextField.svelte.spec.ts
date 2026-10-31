import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import TextField from './TextField.svelte';

describe('TextField', () => {
	it('labels its control and describes it by nothing when there is no hint or error', async () => {
		render(TextField, { id: 'email', label: 'Email address', type: 'email' });

		const input = page.getByLabelText('Email address');
		await expect.element(input).toBeVisible();
		await expect.element(input).not.toHaveAttribute('aria-describedby');
		await expect.element(input).not.toHaveAttribute('aria-invalid');
	});

	it('describes its control by its hint and its error, and marks it invalid', async () => {
		render(TextField, { id: 'code', label: 'Code', hint: 'From your app', error: 'Enter the code' });

		const input = page.getByLabelText('Code');
		await expect.element(input).toHaveAttribute('aria-describedby', 'code-hint code-error');
		await expect.element(input).toHaveAttribute('aria-invalid', 'true');
		await expect.element(page.getByText('Enter the code')).toBeVisible();
		await expect.element(page.getByText('From your app')).toBeVisible();
	});
});
