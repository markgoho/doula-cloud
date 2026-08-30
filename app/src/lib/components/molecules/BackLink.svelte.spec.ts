import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import BackLink from './BackLink.svelte';

type SetupOptions = Partial<ComponentProps<typeof BackLink>>;

async function setup(overrides: SetupOptions = {}) {
	return render(BackLink, { props: { href: '/clients/new/name', ...overrides } });
}

describe('BackLink.svelte', () => {
	it('says "Back" and goes where it is pointed', async () => {
		await setup();

		await expect.element(page.getByRole('link', { name: 'Back' })).toBeVisible();
		await expect
			.element(page.getByRole('link', { name: 'Back' }))
			.toHaveAttribute('href', '/clients/new/name');
	});

	// GOV.UK allows a back link that names where it goes, and #474's
	// "Back to your practices" is one this repo already writes by hand.
	it('takes a label naming where it goes', async () => {
		await setup({ label: 'Back to your practices' });

		await expect.element(page.getByRole('link', { name: 'Back to your practices' })).toBeVisible();
	});
});
