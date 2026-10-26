import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import UnusableLink from './UnusableLink.svelte';

type SetupOptions = Partial<ComponentProps<typeof UnusableLink>>;

async function setup(overrides: SetupOptions = {}) {
	return render(UnusableLink, {
		props: {
			message: 'This link has expired or was already used.',
			next: { label: 'Ask for a new sign-in link', href: '/portal/login' },
			...overrides
		}
	});
}

describe('UnusableLink.svelte', () => {
	it('says what is wrong as an alert', async () => {
		await setup();

		await expect.element(page.getByRole('alert')).toHaveTextContent('This link has expired or was already used.');
	});

	it('offers a page to go to as a link', async () => {
		await setup();

		await expect
			.element(page.getByRole('link', { name: 'Ask for a new sign-in link' }))
			.toHaveAttribute('href', '/portal/login');
	});

	// A step taken away from DoulaCloud has no page, so no link is drawn
	// for it: a link to nowhere would be a lie about where it goes.
	it('says a step with no page as a line of text, not a link', async () => {
		await setup({ next: { text: 'Ask your Practice to send the invite again.' } });

		await expect.element(page.getByText('Ask your Practice to send the invite again.')).toBeVisible();
		await expect.element(page.getByRole('link')).not.toBeInTheDocument();
	});
});
