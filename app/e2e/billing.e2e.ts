import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';

const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

// Exercises the Billing screen from app/src/routes/practices/[practiceId]/billing end-to-end --
// billing.ts has its own Vitest coverage, but this is the only test that renders the actual route
// and hits the real API, proving the signup-bonus grant (#74) surfaces through GetBalanceHandler
// (#75) and the page.
test('Staff member can view the signup-bonus balance and ledger history', async ({ page, request }) => {
	const email = `billing-${Date.now()}@example.com`;
	const password = 'password123';

	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password, returnSecureToken: true } }
	);
	expect(signUp.ok()).toBe(true);
	const { idToken } = await signUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${idToken}` },
		data: { practiceName: 'Riverside Doulas', staffName: 'Jamie Owner', staffEmail: email }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await page.getByRole('link', { name: 'Billing' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/billing$`));

	// A brand-new Practice's balance is the +3 signup_bonus grant from
	// staffauth.signup, and nothing else.
	await expect(page.getByText('Credit balance: 3')).toBeVisible();
	await expect(page.getByRole('cell', { name: 'signup_bonus' })).toBeVisible();
	await expect(page.getByRole('cell', { name: '+3' })).toBeVisible();
});
