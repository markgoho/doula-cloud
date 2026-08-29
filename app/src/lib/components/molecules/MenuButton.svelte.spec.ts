import { createRawSnippet } from 'svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import MenuButton from './MenuButton.svelte';

interface SetupOptions {
	label?: string;
	align?: 'start' | 'end';
	iconOnly?: boolean;
}

async function setup({ label = 'Open the menu', align, iconOnly }: SetupOptions = {}) {
	await render(MenuButton, {
		label,
		align,
		iconOnly,
		icon: 'caret-down',
		children: createRawSnippet(() => ({ render: () => '<p>panel content</p>' }))
	});
	return { trigger: page.getByRole('button', { name: label }) };
}

describe('MenuButton', () => {
	it('keeps the panel shut until the trigger is pressed', async () => {
		await setup();

		await expect.element(page.getByText('panel content')).not.toBeVisible();
	});

	it('opens the panel', async () => {
		const { trigger } = await setup();

		await trigger.click();

		await expect.element(page.getByText('panel content')).toBeVisible();
	});

	it('closes it again on a second press', async () => {
		const { trigger } = await setup();

		await trigger.click();
		await trigger.click();

		await expect.element(page.getByText('panel content')).not.toBeVisible();
	});

	/*
	 * Native `popover` handles Escape, and the browser returns focus to the
	 * trigger on its own. Asserted rather than assumed, because the whole
	 * argument for using the platform here is that these three behaviours
	 * come free -- if they did not, the component would owe them in script.
	 */
	it('closes on Escape and gives focus back to the trigger', async () => {
		const { trigger } = await setup();
		await trigger.click();
		await expect.element(page.getByText('panel content')).toBeVisible();

		await userEvent.keyboard('{Escape}');

		await expect.element(page.getByText('panel content')).not.toBeVisible();
		expect(document.activeElement).toBe(trigger.element());
	});

	/*
	 * Not every engine derives the expanded state from `popovertarget` yet,
	 * so the component mirrors it from the popover's own toggle event.
	 */
	it('reports the open state on the trigger', async () => {
		const { trigger } = await setup();

		await expect.element(trigger).toHaveAttribute('aria-expanded', 'false');
		await trigger.click();
		await expect.element(trigger).toHaveAttribute('aria-expanded', 'true');
	});

	/*
	 * `role="menu"` is a promise of arrow-key navigation between items, and
	 * these panels hold ordinary links and buttons that Tab already reaches.
	 * `aria-haspopup` would be the same promise on the trigger.
	 */
	it('claims neither a menu role nor a popup relationship it does not honour', async () => {
		const { trigger } = await setup();

		await expect.element(trigger).not.toHaveAttribute('aria-haspopup');
		await expect.element(page.getByRole('menu')).not.toBeInTheDocument();
	});

	it.each(['start', 'end'] as const)('hangs from the %s edge', async (align) => {
		const { trigger } = await setup({ align });

		await trigger.click();

		await expect.element(page.getByText('panel content')).toBeVisible();
	});

	it('hides the label when the trigger carries its own face', async () => {
		await setup({ iconOnly: true, label: 'Your account' });

		// Still real DOM text, so the accessible name comes from the document
		// rather than from aria-label.
		await expect.element(page.getByRole('button', { name: 'Your account' })).toBeInTheDocument();
	});
});
