import { createRawSnippet } from 'svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Dialog from './Dialog.svelte';

const CONTENT = createRawSnippet(() => ({ render: () => '<p>dialog content</p>' }));

async function setup({ open = false } = {}) {
	return render(Dialog, { open, label: 'Example dialog', children: CONTENT });
}

describe('Dialog', () => {
	it('renders nothing visible when closed', async () => {
		await setup();

		await expect.element(page.getByText('dialog content')).not.toBeVisible();
	});

	it('shows its content once open', async () => {
		await setup({ open: true });

		await expect.element(page.getByRole('dialog')).toBeVisible();
		await expect.element(page.getByText('dialog content')).toBeVisible();
	});

	it('carries the label as its accessible name', async () => {
		await setup({ open: true });

		await expect.element(page.getByRole('dialog', { name: 'Example dialog' })).toBeVisible();
	});

	/*
	 * The `close` event fires on Escape and on any close() call, so this
	 * one assertion covers both -- and it's what keeps the bindable `open`
	 * in sync when the browser is the one that closed the dialog.
	 */
	it('closes on Escape and syncs `open` back to false', async () => {
		await setup({ open: true });
		await expect.element(page.getByRole('dialog')).toBeVisible();

		await userEvent.keyboard('{Escape}');

		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();
	});

	/*
	 * Real Chromium (vitest browser mode), not jsdom, so showModal()'s
	 * platform focus-return genuinely runs here -- this is not a stand-in
	 * for the Playwright e2e suite, it exercises the same mechanism.
	 */
	it('gives focus back to whatever opened it', async () => {
		const trigger = document.createElement('button');
		trigger.textContent = 'Open';
		document.body.append(trigger);
		trigger.focus();
		expect(document.activeElement).toBe(trigger);

		const { rerender } = await setup({ open: false });
		await rerender({ open: true, label: 'Example dialog', children: CONTENT });
		await expect.element(page.getByRole('dialog')).toBeVisible();

		await rerender({ open: false, label: 'Example dialog', children: CONTENT });
		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();

		expect(document.activeElement).toBe(trigger);
		trigger.remove();
	});
});
