import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ConfirmDialog from './ConfirmDialog.svelte';

interface SetupOptions {
	onConfirm?: () => void | Promise<void>;
	onCancel?: () => void;
}

async function setup({ onConfirm = vi.fn(), onCancel }: SetupOptions = {}) {
	await render(ConfirmDialog, {
		open: true,
		title: 'Remove from Practice',
		consequence: 'This cannot be undone.',
		confirmLabel: 'Remove from Practice',
		onConfirm,
		onCancel
	});
	return { onConfirm };
}

describe('ConfirmDialog', () => {
	it('states the title, the consequence and the named action', async () => {
		await setup();

		await expect
			.element(page.getByRole('heading', { name: 'Remove from Practice' }))
			.toBeVisible();
		await expect.element(page.getByText('This cannot be undone.')).toBeVisible();
		await expect
			.element(page.getByRole('button', { name: 'Remove from Practice' }))
			.toBeVisible();
	});

	it('never uses a generic confirm label', async () => {
		await setup();

		await expect.element(page.getByRole('button', { name: 'OK' })).not.toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Confirm' })).not.toBeInTheDocument();
	});

	it('confirms the action, awaiting onConfirm before closing', async () => {
		let resolveConfirm!: () => void;
		const onConfirm = vi.fn(() => new Promise<void>((resolve) => (resolveConfirm = resolve)));
		await setup({ onConfirm });

		const confirmButton = page.getByRole('button', { name: 'Remove from Practice' });
		await confirmButton.click();

		expect(onConfirm).toHaveBeenCalled();
		// Still open and busy while the promise is in flight.
		await expect.element(page.getByRole('dialog')).toBeVisible();
		await expect.element(confirmButton).toHaveAttribute('aria-busy', 'true');

		resolveConfirm();

		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();
	});

	it('cancels without calling onConfirm', async () => {
		const onCancel = vi.fn();
		const { onConfirm } = await setup({ onCancel });

		await page.getByRole('button', { name: 'Cancel' }).click();

		expect(onConfirm).not.toHaveBeenCalled();
		expect(onCancel).toHaveBeenCalled();
		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();
	});

	it('leaves the dialog open when onConfirm rejects', async () => {
		const onConfirm = vi.fn().mockRejectedValue(new Error('boom'));
		await setup({ onConfirm });

		await page.getByRole('button', { name: 'Remove from Practice' }).click();

		expect(onConfirm).toHaveBeenCalled();
		await expect.element(page.getByRole('dialog')).toBeVisible();
		await expect
			.element(page.getByRole('button', { name: 'Remove from Practice' }))
			.toHaveAttribute('aria-busy', 'false');
	});
});
