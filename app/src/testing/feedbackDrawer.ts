import { page } from 'vitest/browser';
import { expect, vi } from 'vitest';
import { RefusalError } from '#lib/formErrors.js';

export async function sendNotWorkingFeedback(openDrawer: () => Promise<void>) {
	await openDrawer();
	await page.getByLabelText('Something is not working').click();
	await page.getByRole('button', { name: 'Send feedback' }).click();
}

export async function expectRefusalKeepsDrawerOpen(
	setup: (options: { onSend: () => Promise<void> }) => Promise<unknown>,
	openDrawer: () => Promise<void>,
	dialogName: string
) {
	const refusal = 'There is a problem with the service. Try again in a few minutes.';
	const onSend = vi.fn().mockRejectedValue(new RefusalError(refusal));
	await setup({ onSend });

	await openDrawer();
	await page.getByLabelText('An idea or a request').click();
	await page.getByLabelText('Tell us more').fill('Kept on a refusal.');
	await page.getByRole('button', { name: 'Send feedback' }).click();

	await expect.element(page.getByText(refusal)).toBeVisible();
	await expect.element(page.getByRole('dialog', { name: dialogName })).toBeVisible();
	await expect.element(page.getByLabelText('Tell us more')).toHaveValue('Kept on a refusal.');
}
