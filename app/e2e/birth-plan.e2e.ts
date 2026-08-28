import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { signIn } from './auth';
import { seedClientPortalUser, seedEngagement } from './stack';

// Exercises #65's critical path: Staff fills out a Birth Plan for an
// Engagement (through the real staff-side UI from #64), then the Client
// portal shows the matching read-only view. Provisions Practice/Client/
// Engagement and the two Identity Platform accounts the same way
// client-portal-login.e2e.ts does -- this test isn't re-proving login
// itself, just that both sides of the Birth Plan feature agree.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

test('Staff fills a Birth Plan, and the Client portal shows the matching read-only view', async ({
	page,
	request
}) => {
	// Random suffix, not just Date.now(): see staff-login.e2e.ts for why
	// millisecond-only uniqueness collides across parallel workers.
	const staffEmail = `staff-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const clientEmail = `client-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const password = 'password123';

	const staffSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: staffEmail, password, returnSecureToken: true } }
	);
	expect(staffSignUp.ok(), `staffSignUp failed: ${staffSignUp.status()} ${await staffSignUp.text()}`).toBe(true);
	const { idToken: staffIdToken } = await staffSignUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${staffIdToken}` },
		data: { practiceName: 'Riverside Doulas', staffName: 'Jamie Owner', staffEmail }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `staff signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	// Everything after signup is cookie-authenticated (#151).
	const staffHeaders = await signIn(request, API_URL, staffIdToken);

	const createClient = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
		headers: staffHeaders,
		data: { givenName: 'Pat', familyName: 'Client', email: clientEmail }
	});
	const createClientBody = await createClient.text();
	expect(createClient.ok(), `create client failed: ${createClient.status()} ${createClientBody}`).toBe(
		true
	);
	const { id: clientId } = JSON.parse(createClientBody);
	const engagementId = seedEngagement(clientId, practiceId);

	// Staff side: log in, create the Birth Plan (signup seeds a default
	// template per Practice per #63), fill one field, and save.
	await page.goto('/login');
	await page.getByLabel('Email').fill(staffEmail);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// The Clients list no longer links each row to an Engagement (#397 --
	// the Client detail page each row will link to is #400's separate,
	// not-yet-built screen), so this test's only route to the Engagement
	// is a direct navigation rather than the old click-through.
	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/engagements/${engagementId}$`));

	await page.getByRole('button', { name: 'Create Birth Plan' }).click();
	await page
		.getByLabel('Preferences for atmosphere (music, lighting, etc.)')
		.fill('Soft lighting, calm music');
	const saveButton = page.getByRole('button', { name: 'Save Birth Plan' });
	await saveButton.click();
	// The click only dispatches the event -- handleSavePlan's PUT is async,
	// and the button stays disabled (planBusy) until it resolves. Wait for
	// it to re-enable so the save has actually round-tripped before this
	// test switches to the Client-portal session below.
	await expect(saveButton).toBeEnabled();

	// Client side: a separate Identity Platform account, linked to the same
	// Client via client_portal_users, viewing the read-only Birth Plan.
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

	await page.getByRole('link', { name: 'Birth Plan' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}/birth-plan$`));
	await expect(page.getByText('Soft lighting, calm music')).toBeVisible();
});
