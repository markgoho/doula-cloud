import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';

// The Firebase Auth emulator and the Go BFF -- both host processes -- see
// e2e/global-setup.ts and e2e/stack.ts for how these get started.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

test('Staff login lands on their practice-scoped URL', async ({ page, request }) => {
	// The random suffix (not just Date.now(), millisecond-resolution) avoids
	// EMAIL_EXISTS collisions with other *.e2e.ts files' own staff-<ts>@
	// emails when Playwright's parallel workers start within the same
	// millisecond of each other -- confirmed as a real, intermittent
	// failure across this suite, not a theoretical one.
	const email = `staff-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const password = 'password123';

	// Provision the account and Practice directly against the emulator and
	// BFF, the way createUserWithEmailAndPassword + POST /api/staff/signup
	// would from the signup page -- this test is about the *login* landing
	// behaviour, not re-proving signup.
	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password, returnSecureToken: true } }
	);
	expect(signUp.ok(), `signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
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
	await expect(page.locator('h1')).toHaveText('Welcome to Riverside Doulas');
});
