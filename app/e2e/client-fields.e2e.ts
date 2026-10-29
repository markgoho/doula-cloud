import { expect, test } from '@playwright/test';
import { API_URL } from './ports';
import { enterPractice, signIn } from './auth';
import { seedFoundingOwner } from './staffSignup';

// The Client Fields list and its question pages (#2193) against the real API: a new Practice starts
// with none, a question is added through its kind and its own page, archived from its page, and
// collected again from the list.
test('Practice Owner adds, archives and collects again a Client field', async ({ page, request, context }) => {
	const { idToken, practiceId } = await seedFoundingOwner(request);

	const staffHeaders = await signIn(request, API_URL, idToken);
	await enterPractice(context, page, staffHeaders, practiceId);
	await page.goto(`/practices/${practiceId}/settings/client-fields`);

	await expect(page.getByText('No extra questions yet.')).toBeVisible();

	await page.getByRole('link', { name: 'Add a question' }).click();
	await page.getByRole('radio', { name: 'Choose one' }).check();
	await page.getByRole('button', { name: 'Continue' }).click();
	await page.getByLabel('Question', { exact: true }).fill('How did you hear about us?');
	await page.getByLabel('Answers to choose from').fill('A friend\nA search');
	await page.getByRole('button', { name: 'Save question' }).click();

	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/settings/client-fields$`));
	await expect(page.getByRole('status').filter({ hasText: 'Client Fields saved.' })).toBeVisible();
	await expect(page.locator('dt', { hasText: 'How did you hear about us?' })).toBeVisible();

	// Saved, its kind of answer is kept; archiving it is confirmed first.
	await page.getByRole('link', { name: 'Change', exact: true }).click();
	await expect(page.getByLabel('Kind of answer')).toBeDisabled();
	await page.getByRole('button', { name: 'Archive question' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Archive question' }).click();

	await expect(page.getByRole('heading', { name: 'Archived questions' })).toBeVisible();
	await expect(page.getByText('No extra questions yet.')).toBeVisible();

	await page.getByRole('button', { name: 'Collect again' }).click();
	await expect(page.getByRole('heading', { name: 'Archived questions' })).toHaveCount(0);
	await expect(page.getByRole('link', { name: 'Change', exact: true })).toHaveCount(1);
});
