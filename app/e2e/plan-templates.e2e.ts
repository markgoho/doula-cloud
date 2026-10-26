import { expect, test } from '@playwright/test';
import { API_URL } from './ports';
import { enterPractice, signIn } from './auth';
import { seedFoundingOwner } from './staffSignup';

// Exercises the settings screen from app/src/routes/practices/[practiceId]/settings/plan-templates
// end-to-end -- the pieces it's built from (planTemplate.ts, PracticeQuestionsEditor.svelte) have their
// own Vitest coverage, but this is the only test that renders the actual route and hits the real
// API, proving signup's seeded defaults, add/save, and the plan-type switch all work together.
test('Practice Owner can view seeded plan templates and add/save a field', async ({
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

	// Seeded default care_plan fields render.
	await expect(page.getByLabel('Question', { exact: true }).first()).not.toHaveValue('');
	expect(await page.getByLabel('Question', { exact: true }).count()).toBeGreaterThan(0);

	// Add a field, save, and switch plan types and back -- each switch
	// re-fetches from the server, proving the save round-tripped through
	// the real API rather than only updating client-side state.
	await page.getByRole('button', { name: 'Add a short answer' }).click();
	await page.getByLabel('Question', { exact: true }).last().fill('Favorite music');
	await page.getByRole('button', { name: 'Save' }).click();
	await expect(page.getByText('Saved.')).toBeVisible();

	await page.getByRole('tab', { name: 'Birth Plan' }).click();
	await expect(page.getByLabel('Question', { exact: true }).first()).not.toHaveValue('Favorite music');
	await page.getByRole('tab', { name: 'Care Plan' }).click();
	await expect(page.getByLabel('Question', { exact: true }).last()).toHaveValue('Favorite music');
});
