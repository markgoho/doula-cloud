import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PilotBanner from './PilotBanner.svelte';

type SetupOptions = Partial<ComponentProps<typeof PilotBanner>>;

async function setup({
	sentence = 'Doula Cloud is new, and you are one of the first to use it.',
	controlText = 'Tell us what is not working or what you need',
	open = false,
	controlsId = 'feedback-drawer',
	...rest
}: SetupOptions = {}) {
	const onOpenFeedback = vi.fn();
	await render(PilotBanner, { sentence, controlText, open, controlsId, onOpenFeedback, ...rest });
	return { onOpenFeedback };
}

describe('PilotBanner.svelte', () => {
	it('shows the Pilot tag', async () => {
		await setup();

		await expect.element(page.getByText('Pilot')).toBeVisible();
	});

	it('shows the sentence a caller passes', async () => {
		await setup({ sentence: 'This care portal is new.' });

		await expect.element(page.getByText('This care portal is new.', { exact: false })).toBeVisible();
	});

	it('shows the control text inline, as a button rather than a link, with the sentence-closing period folded into it', async () => {
		await setup({ controlText: 'Send us feedback' });

		// The period is part of the button's own accessible name, not a
		// separate text node after it: a `<button>` stays an atomic,
		// shrink-to-fit box even styled `display: inline`, so a period
		// placed after it can wrap onto a line of its own once the button
		// wraps -- confirmed on /style-guide/pilot-banner at 320px.
		const control = page.getByRole('button', { name: 'Send us feedback.' });
		await expect.element(control).toBeVisible();
	});

	it('calls onOpenFeedback when the control is clicked, and owns no drawer state of its own', async () => {
		const { onOpenFeedback } = await setup();

		await page.getByRole('button').click();

		expect(onOpenFeedback).toHaveBeenCalledOnce();
	});

	it('reflects the drawer being closed via aria-expanded', async () => {
		await setup({ open: false });

		await expect.element(page.getByRole('button')).toHaveAttribute('aria-expanded', 'false');
	});

	it('reflects the drawer being open via aria-expanded', async () => {
		await setup({ open: true });

		await expect.element(page.getByRole('button')).toHaveAttribute('aria-expanded', 'true');
	});

	it("names the caller's drawer via aria-controls", async () => {
		await setup({ controlsId: 'staff-feedback-drawer' });

		await expect
			.element(page.getByRole('button'))
			.toHaveAttribute('aria-controls', 'staff-feedback-drawer');
	});
});
