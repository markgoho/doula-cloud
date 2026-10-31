import { createRawSnippet, type ComponentProps } from 'svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import MenuButton from './MenuButton.svelte';

interface SetupOptions {
	label?: string;
	align?: 'start' | 'end';
	iconOnly?: boolean;
	children?: ComponentProps<typeof MenuButton>['children'];
}

async function setup({
	label = 'Open the menu',
	align,
	iconOnly,
	children = createRawSnippet(() => ({ render: () => '<p>panel content</p>' }))
}: SetupOptions = {}) {
	await render(MenuButton, {
		label,
		align,
		iconOnly,
		icon: 'caret-down',
		children
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
	 * argument for using the platform here is that these three behaviors
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
	it('claims neither a menu role nor a popup relationship it does not honor', async () => {
		const { trigger } = await setup();

		await expect.element(trigger).not.toHaveAttribute('aria-haspopup');
		await expect.element(page.getByRole('menu')).not.toBeInTheDocument();
	});

	it.each(['start', 'end'] as const)('hangs from the %s edge', async (align) => {
		const { trigger } = await setup({ align });

		await trigger.click();

		await expect.element(page.getByText('panel content')).toBeVisible();
	});

	/*
	 * #2335: a link in the panel that moves to another page left the panel
	 * open over it. SvelteKit's client-side navigation keeps the top bar
	 * mounted, so nothing a full page load does for free closes it. The
	 * router is stood in for by a listener on the document, where
	 * SvelteKit's own sits, so the press is a real link press that does not
	 * take the test runner's frame with it. The panel shuts before the
	 * router runs, so the focus the router then resets to the new page is
	 * not taken back to the trigger by a later close (#2235's order).
	 */
	it('shuts as a link inside it is pressed, before the router runs', async () => {
		const { trigger } = await setup({
			children: createRawSnippet(() => ({ render: () => '<p><a href="/elsewhere">Somewhere else</a></p>' }))
		});
		await trigger.click();
		let openWhenRouted: boolean | undefined;
		const route = (event: MouseEvent) => {
			event.preventDefault();
			openWhenRouted = document.querySelector('[popover]')?.matches(':popover-open');
		};
		document.addEventListener('click', route);

		try {
			await page.getByRole('link', { name: 'Somewhere else' }).click();
		} finally {
			document.removeEventListener('click', route);
		}

		expect(openWhenRouted).toBe(false);
		await expect.element(page.getByText('Somewhere else')).not.toBeVisible();
		await expect.element(trigger).toHaveAttribute('aria-expanded', 'false');
	});

	it('stays open for a press inside it that is not on a link', async () => {
		const { trigger } = await setup();
		await trigger.click();

		await page.getByText('panel content').click();

		await expect.element(page.getByText('panel content')).toBeVisible();
	});

	it('hides the label when the trigger carries its own face', async () => {
		await setup({ iconOnly: true, label: 'Your account' });

		// Still real DOM text, so the accessible name comes from the document
		// rather than from aria-label.
		await expect.element(page.getByRole('button', { name: 'Your account' })).toBeInTheDocument();
	});
});
