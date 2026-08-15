import { expect, test } from '@playwright/test';

// The Firebase Auth emulator and the Go BFF container -- see
// e2e/global-setup.ts and compose.e2e.yaml for how these get started.
const EMULATOR_URL = 'http://127.0.0.1:9099';
const API_URL = 'http://127.0.0.1:18080';

test('Staff login lands on their practice-scoped URL', async ({ page, request }) => {
	const email = `staff-${Date.now()}@example.com`;
	const password = 'password123';

	// Provision the account and Practice directly against the emulator and
	// BFF, the way createUserWithEmailAndPassword + POST /api/staff/signup
	// would from the signup page -- this test is about the *login* landing
	// behaviour, not re-proving signup.
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
	expect(signup.ok()).toBe(true);
	const { practiceId } = await signup.json();

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();

	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));
	await expect(page.locator('h1')).toHaveText('Welcome to Riverside Doulas');
});
