import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import SecondFactorNeeded from './SecondFactorNeeded.svelte';

type SetupOptions = Partial<ComponentProps<typeof SecondFactorNeeded>>;

async function setup(overrides: SetupOptions = {}) {
	return render(SecondFactorNeeded, {
		props: {
			message: 'You need two-factor authentication before you can delete this Practice.',
			returnTo: '/practices/p-1/settings/delete',
			...overrides
		}
	});
}

describe('SecondFactorNeeded.svelte', () => {
	it('says what the act needs before anyone tries it', async () => {
		await setup();

		await expect
			.element(page.getByText('You need two-factor authentication before you can delete this Practice.'))
			.toBeVisible();
	});

	// #1532: enrollment sends her back to the screen she left, through the
	// same `returnTo` the Practice layout already hands `/mfa/enroll`.
	it('links to enrollment, carrying the screen to come back to', async () => {
		await setup();

		const link = page.getByRole('link', { name: 'Set up two-factor authentication' });
		await expect.element(link).toBeVisible();
		await expect
			.element(link)
			.toHaveAttribute('href', '/mfa/enroll?returnTo=%2Fpractices%2Fp-1%2Fsettings%2Fdelete');
	});
});
