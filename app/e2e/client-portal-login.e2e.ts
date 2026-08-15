import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { seedClientPortalUser } from './stack';

// The Firebase Auth emulator and the Go BFF container -- see
// e2e/global-setup.ts and compose.e2e.yaml for how these get started.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

test('Client-portal login lands on their engagement-scoped URL', async ({ page, request }) => {
	const staffEmail = `staff-${Date.now()}@example.com`;
	const clientEmail = `client-${Date.now()}@example.com`;
	const password = 'password123';

	// Provision a Practice + Staff (owner) via the emulator + BFF, then use
	// that Staff session to create a Client + Engagement at the Practice --
	// the same way #52's Staff-side create-Client flow would from the UI.
	// This test is about the Client-portal *login* landing behaviour, not
	// re-proving either of those.
	const staffSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: staffEmail, password, returnSecureToken: true } }
	);
	expect(staffSignUp.ok()).toBe(true);
	const { idToken: staffIdToken } = await staffSignUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${staffIdToken}` },
		data: { practiceName: 'Riverside Doulas', staffName: 'Jamie Owner', staffEmail }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `staff signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	const createClient = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
		headers: { Authorization: `Bearer ${staffIdToken}` },
		data: { name: 'Pat Client', email: clientEmail }
	});
	const createClientBody = await createClient.text();
	expect(createClient.ok(), `create client failed: ${createClient.status()} ${createClientBody}`).toBe(
		true
	);
	const { clientId, engagementId } = JSON.parse(createClientBody);

	// A separate Identity Platform account for the Client-portal login --
	// distinct from the Staff account above -- linked to that Client via
	// client_portal_users (see stack.ts for why this is seeded directly
	// rather than through a BFF endpoint).
	const clientSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: clientEmail, password, returnSecureToken: true } }
	);
	expect(clientSignUp.ok()).toBe(true);
	const { localId: clientUID } = await clientSignUp.json();
	seedClientPortalUser(clientUID, clientId);

	await page.goto('/portal/login');
	await page.getByLabel('Email').fill(clientEmail);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();

	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
	await expect(page.locator('h1')).toHaveText('Welcome to Riverside Doulas');
});
