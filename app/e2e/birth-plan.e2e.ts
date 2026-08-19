import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { seedClientPortalUser } from './stack';

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

	const createClient = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
		headers: { Authorization: `Bearer ${staffIdToken}` },
		data: { name: 'Pat Client', email: clientEmail }
	});
	const createClientBody = await createClient.text();
	expect(createClient.ok(), `create client failed: ${createClient.status()} ${createClientBody}`).toBe(
		true
	);
	const { clientId, engagementId } = JSON.parse(createClientBody);

	// Staff side: log in, create the Birth Plan (signup seeds a default
	// template per Practice per #63), fill one field, and save.
	await page.goto('/login');
	await page.getByLabel('Email').fill(staffEmail);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await page.getByRole('link', { name: 'Clients' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/clients$`));
	await page.getByRole('link', { name: 'Pat Client' }).click();
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

	// Staff's own login now leaves a __session cookie in the browser
	// (#149), and the Client-portal login below still authenticates by
	// Bearer token only (#150 migrates it next) -- authn.Begin prefers a
	// present cookie over a Bearer token, so without clearing it here the
	// Client-portal probe would resolve to the Staff identity instead and
	// 404 with "no matching client account". This is a real, if narrow,
	// production risk for the same browser mixing both populations during
	// the window between #149 and #150 shipping (also true, symmetrically,
	// within one population when two different people share a browser and
	// device before signing out -- see docs/adr/0004-bff-owned-sessions.md,
	// which is why a proper fix belongs to the session-storage redesign
	// there rather than a frontend patch here: `credentials: 'omit'` on
	// apiFetch looks like the obvious fix but also blocks the browser from
	// storing the Set-Cookie these same bootstrap responses send back,
	// confirmed by trying it -- every Staff login/signup/accept-invite
	// flow broke). #150 removes this specific case, since Client-portal
	// login will then mint its own cookie and overwrite this one, the same
	// way any fresh sign-in supersedes a stale session.
	await page.context().clearCookies();

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
