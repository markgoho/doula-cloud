import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PilotFeedback from './PilotFeedback.svelte';

async function setup({ roleAndPractice }: { roleAndPractice?: string } = {}) {
	const onSend = vi.fn().mockResolvedValue(undefined);
	await render(PilotFeedback, {
		email: 'alex@example.test',
		bannerSentence: 'This is new.',
		controlText: 'Tell us what you think',
		drawerHeading: 'Send feedback',
		introText: 'We read every piece.',
		destination: 'This goes to the team.',
		sentBy: 'the team',
		roleAndPractice,
		onSend
	});
	return { onSend };
}

describe('PilotFeedback', () => {
	it('shows the banner, and opens the drawer with its heading and intro', async () => {
		await setup();

		await expect.element(page.getByText('This is new.', { exact: false })).toBeVisible();
		await page.getByRole('button', { name: 'Tell us what you think' }).click();

		await expect.element(page.getByRole('dialog', { name: 'Send feedback' })).toBeVisible();
		await expect.element(page.getByText('We read every piece.')).toBeVisible();
		await expect.element(page.getByText('This goes to the team.')).toBeVisible();
	});

	it('closes the drawer and says who will reply once the feedback is sent', async () => {
		const { onSend } = await setup();

		await page.getByRole('button', { name: 'Tell us what you think' }).click();
		await page.getByRole('radio').first().click();
		await page.getByLabelText('Tell us more').fill('The save button is hard to find.');
		await page.getByRole('button', { name: /^Send/ }).click();

		await expect.element(page.getByText('alex@example.test', { exact: false })).toBeVisible();
		expect(onSend).toHaveBeenCalledOnce();
	});
});
