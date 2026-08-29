import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';

const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

// Exercises the settings screen from app/src/routes/practices/[practiceId]/settings/contract-template
// end-to-end -- the pieces it's built from (contractTemplate.ts, ContractTemplateEditor.svelte) have
// their own Vitest coverage, but this is the only test that renders the actual route and hits the real
// API, proving signup's seeded default and edit/save work together.
test('Practice Owner can view the seeded contract template and edit/save it', async ({ page, request }) => {
	const email = `contract-template-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const password = 'password123';

	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password, returnSecureToken: true } }
	);
	expect(signUp.ok()).toBe(true);
	const { idToken } = await signUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${idToken}` },
		data: { practiceName: 'Riverside Doulas', staffName: 'Jamie Owner', staffEmail: email , workState: 'NY' }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();
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
