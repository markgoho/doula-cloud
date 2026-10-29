import { expect, test } from '@playwright/test';
import { API_URL } from './ports';
import { enterPractice, signIn } from './auth';
import { seedFoundingOwner } from './staffSignup';

// Exercises the Plan Templates list and its question pages (#2193) end-to-end against the real API:
// signup's seeded defaults, adding a question through its kind and its own page, moving it, and the
// plan-type switch. The pieces (planTemplate.ts, practiceQuestions.ts, PracticeQuestionList,
// PracticeQuestionForm) have their own Vitest coverage; this is the test that hits the real API.
test('Practice Owner adds a Plan Template question on its own page and moves it to the top', async ({
	page,
	request,
	context
}) => {
	const { idToken, practiceId } = await seedFoundingOwner(request);

	const staffHeaders = await signIn(request, API_URL, idToken);
	await enterPractice(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// Through Settings, which is where these screens live now that the
	// shell's six-item nav replaced the temporary header of links (#452).
	await page.getByRole('link', { name: 'Settings' }).first().click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/settings$`));
	// The index is a summary list (#2028): the row's Change is the way in.
	await page
		.locator('dt', { hasText: /^Plan Templates$/ })
		.locator('..')
		.getByRole('link', { name: 'Change', exact: true })
		.click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/settings/plan-templates$`));

	// The seeded Care Plan questions are rows, each with its own Change.
	const changes = page.getByRole('link', { name: 'Change', exact: true });
	await expect(changes.first()).toBeVisible();
	const seededCount = await changes.count();
	expect(seededCount).toBeGreaterThan(0);

	// Add a question asks the kind of answer first, then opens its page.
	await page.getByRole('link', { name: 'Add a question' }).click();
	await page.getByRole('radio', { name: 'Short answer' }).check();
	await page.getByRole('button', { name: 'Continue' }).click();
	await page.getByLabel('Question', { exact: true }).fill('Favorite music');
	await page.getByRole('button', { name: 'Save question' }).click();

	// Saved through the real API, and said on the list it lands on.
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/settings/plan-templates$`));
	await expect(page.getByRole('status').filter({ hasText: 'Care Plan questions saved.' })).toBeVisible();
	await expect(changes).toHaveCount(seededCount + 1);
	await expect(page.locator('dt').last()).toHaveText('Favorite music');

	// Its own page moves it to the top of the list.
	await changes.last().click();
	await page.getByLabel('Place in the list').selectOption({ label: `1 of ${seededCount + 1}` });
	await page.getByRole('button', { name: 'Save question' }).click();
	await expect(page.getByRole('status').filter({ hasText: 'Care Plan questions saved.' })).toBeVisible();
	await expect(page.locator('dt').first()).toHaveText('Favorite music');

	// The Birth Plan is a template of its own, read from the server.
	await page.getByRole('tab', { name: 'Birth Plan' }).click();
	await expect(page).toHaveURL(/\?plan=birth_plan$/);
	await expect(page.locator('dt').first()).not.toHaveText('Favorite music');
	await expect(page.getByText('Favorite music')).toHaveCount(0);
});
