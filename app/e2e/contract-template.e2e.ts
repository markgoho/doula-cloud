import { expect, test } from '@playwright/test';
import { signInEnrolled, enterPracticeAsEnrolled } from './mfa';
import { seedFoundingOwner } from './staffSignup';

// Exercises the settings screen from app/src/routes/practices/[practiceId]/settings/contract-template
// end-to-end -- the pieces it's built from (contractTemplate.ts, ContractTemplateEditor.svelte) have
// their own Vitest coverage, but this is the only test that renders the actual route and hits the real
// API, proving signup's seeded default and edit/save work together.
test('Practice Owner can view the seeded contract template and edit/save it', async ({
	page,
	request,
	context
}) => {
	const { idToken, localId, practiceId } = await seedFoundingOwner(request);

	// #606: an Owner is gated behind a second factor at every Practice-scoped
	// route (see mfa.ts's signInEnrolled doc comment).
	const staffHeaders = await signInEnrolled(request, idToken, localId);
	await enterPracticeAsEnrolled(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// Through Settings, which is where these screens live now that the
	// shell's six-item nav replaced the temporary header of links (#452).
	await page.getByRole('link', { name: 'Settings' }).first().click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/settings$`));
	await page.getByRole('link', { name: 'Contract Template' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/settings/contract-template$`));

	// Seeded default prose renders.
	await expect(page.getByLabel('Contract template prose')).not.toHaveValue('');

	// Merge-field reference is visible.
	await expect(page.getByText('{{client_name}}')).toBeVisible();

	// Edit and save, then navigate away and back -- each visit re-fetches
	// from the server (the route's onMount), proving the save round-tripped
	// through the real API rather than only updating client-side state.
	// A client-side nav, not a full page.reload(), matches how Firebase Auth
	// state is exercised elsewhere in this suite (plan-templates.e2e.ts).
	await page.getByLabel('Contract template prose').fill('Updated agreement with {{client_name}}');
	await page.getByRole('button', { name: 'Save' }).click();
	await expect(page.getByText('Saved.')).toBeVisible();

	// Back is the Settings hub now, not the Practice overview: the way in
	// runs through it since the shell's six-item nav replaced the temporary
	// header of links (#452).
	await page.goBack();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/settings$`));
	await page.getByRole('link', { name: 'Contract Template' }).click();
	await expect(page.getByLabel('Contract template prose')).toHaveValue(
		'Updated agreement with {{client_name}}'
	);
});
