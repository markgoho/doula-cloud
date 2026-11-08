import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it, vi } from 'vitest';
import SignOutConfirmation from './SignOutConfirmation.svelte';

interface SetupOptions {
	isSubmitting?: boolean;
}

async function setup({ isSubmitting = false }: SetupOptions = {}) {
	const onConfirm = vi.fn();
	const onCancel = vi.fn();
	await render(SignOutConfirmation, {
		warning: 'You will be signed out of Riverside Doula Collective.',
		isSubmitting,
		onConfirm,
		onCancel
	});
	return { onConfirm, onCancel };
}

describe('SignOutConfirmation', () => {
	it('names the consequence under a heading and offers both ways out', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { name: 'Before you continue' })).toBeVisible();
		await expect
			.element(page.getByText('You will be signed out of Riverside Doula Collective.'))
			.toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Continue and sign out' })).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Cancel' })).toBeVisible();
	});

	it('takes focus on the heading as it appears', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { name: 'Before you continue' })).toHaveFocus();
	});

	it('calls onConfirm when she continues', async () => {
		const { onConfirm, onCancel } = await setup();

		await page.getByRole('button', { name: 'Continue and sign out' }).click();

		expect(onConfirm).toHaveBeenCalledOnce();
		expect(onCancel).not.toHaveBeenCalled();
	});

	it('calls onCancel when she stays', async () => {
		const { onConfirm, onCancel } = await setup();

		await page.getByRole('button', { name: 'Cancel' }).click();

		expect(onCancel).toHaveBeenCalledOnce();
		expect(onConfirm).not.toHaveBeenCalled();
	});

	it('shows the continue button busy while the sign-out is in flight', async () => {
		await setup({ isSubmitting: true });

		await expect.element(page.getByRole('button', { name: 'Continue and sign out' })).toBeVisible();
	});
});
