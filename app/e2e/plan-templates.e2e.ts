import { expect, test } from '@playwright/test';
import { API_URL } from './ports';
import { signIn } from './auth';
import { enterPracticeAsEnrolled } from './mfa';
import { seedFoundingOwner } from './staffSignup';

// Exercises the settings screen from app/src/routes/practices/[practiceId]/settings/plan-templates
// end-to-end -- the pieces it's built from (planTemplate.ts, PlanTemplateEditor.svelte) have their
// own Vitest coverage, but this is the only test that renders the actual route and hits the real
// API, proving signup's seeded defaults, add/save, and the plan-type switch all work together.
test('Practice Owner can view seeded plan templates and add/save a field', async ({
	page,
	request,
	context
}) => {
	const { idToken, practiceId } = await seedFoundingOwner(request);

	const staffHeaders = await signIn(request, API_URL, idToken);
	await enterPracticeAsEnrolled(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// Through Settings, which is where these screens live now that the
	// shell's six-item nav replaced the temporary header of links (#452).
	await page.getByRole('link', { name: 'Settings' }).first().click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/settings$`));
	await page.getByRole('link', { name: 'Plan Templates' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/settings/plan-templates$`));

	// Seeded default care_plan fields render.
	await expect(page.getByLabel('Field label').first()).not.toHaveValue('');
	expect(await page.getByLabel('Field label').count()).toBeGreaterThan(0);

	// Add a field, save, and switch plan types and back -- each switch
	// re-fetches from the server, proving the save round-tripped through
	// the real API rather than only updating client-side state.
	await page.getByRole('button', { name: 'Add field' }).click();
	await page.getByLabel('Field label').last().fill('Favorite music');
	await page.getByRole('button', { name: 'Save' }).click();
	await expect(page.getByText('Saved.')).toBeVisible();

	await page.getByRole('tab', { name: 'Birth Plan' }).click();
	await expect(page.getByLabel('Field label').first()).not.toHaveValue('Favorite music');
	await page.getByRole('tab', { name: 'Care Plan' }).click();
	await expect(page.getByLabel('Field label').last()).toHaveValue('Favorite music');
});
