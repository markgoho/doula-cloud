import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import EmptyState from './EmptyState.svelte';

type SetupOptions = Partial<ComponentProps<typeof EmptyState>>;

async function setup(overrides: SetupOptions = {}) {
	return render(EmptyState, { props: { message: 'No Clients yet.', ...overrides } });
}

describe('EmptyState.svelte', () => {
	it('says what is not here yet, with no action when none is given', async () => {
		await setup();

		await expect.element(page.getByText('No Clients yet.')).toBeVisible();
		expect(page.getByRole('link').elements()).toHaveLength(0);
		expect(page.getByRole('button').elements()).toHaveLength(0);
	});

	// The icon is decoration: the line is the whole of what a screen
	// reader hears, so nothing in the accessible tree names the icon.
	it('hides its icon from assistive technology', async () => {
		await setup();

		expect(page.getByRole('img').elements()).toHaveLength(0);
	});

	it('offers a destination as a link', async () => {
		await setup({ action: { label: 'Add a Client', href: '/clients/new' } });

		await expect
			.element(page.getByRole('link', { name: 'Add a Client' }))
			.toHaveAttribute('href', '/clients/new');
	});

	// A list's live result line already says it, so the line is drawn here and heard there.
	it('draws a line a live line above already says, and leaves it to that line to be heard', async () => {
		await setup({ message: 'No Invoice matches "zzqx".', isMessageSpoken: false, action: { label: 'Show every Invoice', href: '/invoices' } });

		await expect.element(page.getByText('No Invoice matches "zzqx".')).toBeVisible();
		expect(page.getByRole('paragraph').elements()).toHaveLength(0);
		await expect.element(page.getByRole('link', { name: 'Show every Invoice' })).toBeVisible();
	});

	it('offers a change on the same screen as a button that runs it', async () => {
		const onClick = vi.fn();
		await setup({ action: { label: 'Add a question', onClick } });

		await page.getByRole('button', { name: 'Add a question' }).click();

		expect(onClick).toHaveBeenCalledOnce();
	});
});
