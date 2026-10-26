import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Heading from './Heading.svelte';

type SetupOptions = Partial<ComponentProps<typeof Heading>>;

async function setup({ level = 1, text = 'Page title', ...rest }: SetupOptions = {}) {
	const { container } = await render(Heading, { level, text, ...rest });
	return { container };
}

describe('Heading.svelte', () => {
	it('renders the text as visible content', async () => {
		await setup({ text: 'Page title' });

		await expect.element(page.getByText('Page title')).toBeVisible();
	});

	it.each([1, 2, 3, 4, 5, 6] as const)('renders level %i as its own heading tag', async (level) => {
		const { container } = await setup({ level });

		expect(container.querySelector(`h${level}`)).toBeVisible();
	});

	/*
	 * The default map is the point of the component: a route that only knows
	 * its document outline still gets the right size, so `variant` is a
	 * deliberate override rather than a thing every call site must remember.
	 */
	it.each([
		[1, 'page'],
		[2, 'section'],
		[3, 'card'],
		[4, 'card'],
		[5, 'card'],
		[6, 'card']
	] as const)('defaults level %i to the %s variant', async (level, variant) => {
		const { container } = await setup({ level });

		expect(container.querySelector(`h${level}.variant-${variant}`)).toBeVisible();
	});

	it.each(['page', 'section', 'card'] as const)(
		'lets %s be chosen independently of the heading level',
		async (variant) => {
			const { container } = await setup({ level: 3, variant });

			expect(container.querySelector(`h3.variant-${variant}`)).toBeVisible();
		}
	);

	/*
	 * #2018: the signed-out steps (#816) replace a form with a heading that
	 * must take focus, and the heading is still the atom.
	 */
	it('takes focus when it appears, without joining the tab order', async () => {
		await setup({ level: 2, text: 'Before you continue', focusOnAppearing: true });

		const heading = page.getByRole('heading', { level: 2, name: 'Before you continue' });
		await expect.element(heading).toHaveFocus();
		await expect.element(heading).toHaveAttribute('tabindex', '-1');
	});

	it('neither takes focus nor a tabindex unless asked to', async () => {
		await setup({ level: 2, text: 'Section' });

		const heading = page.getByRole('heading', { level: 2, name: 'Section' });
		await expect.element(heading).not.toHaveFocus();
		await expect.element(heading).not.toHaveAttribute('tabindex');
	});

	it('does not expose the display step, which the OverviewHub Template owns', async () => {
		const { container } = await setup({ level: 1 });

		expect(container.querySelector('[class*="display"]')).toBeNull();
	});
});
