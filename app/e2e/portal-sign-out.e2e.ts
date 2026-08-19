import { expect, test, type APIRequestContext } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { signIn } from './auth';
import { seedClientPortalUser } from './stack';

// The Firebase Auth emulator and the Go BFF -- both host processes -- see
// e2e/global-setup.ts and e2e/stack.ts for how these get started.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

const PASSWORD = 'password123';

/**
 * Provisions everything a Client needs to log in to their portal: a
 * Practice with a Staff owner, a Client + Engagement at that Practice,
 * and an Identity Platform account linked to the Client. Done through the
 * API and stack.ts's fixture rather than the UI -- these tests are about
 * *sign-out*, not about re-proving signup, Client creation or portal
 * provisioning.
 */
async function seedPortalClient(request: APIRequestContext, practiceName: string) {
	// The random suffix (not just Date.now(), millisecond-resolution)
	// avoids EMAIL_EXISTS collisions with other *.e2e.ts files' emails when
	// Playwright's parallel workers start within the same millisecond.
	const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	const staffEmail = `portal-signout-staff-${unique}@example.com`;
	const clientEmail = `portal-signout-client-${unique}@example.com`;

	const staffSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: staffEmail, password: PASSWORD, returnSecureToken: true } }
	);
	expect(staffSignUp.ok(), `staffSignUp failed: ${staffSignUp.status()} ${await staffSignUp.text()}`).toBe(true);
	const { idToken: staffIdToken } = await staffSignUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${staffIdToken}` },
		data: { practiceName, staffName: 'Alex Owner', staffEmail }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `staff signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	// Everything after signup is cookie-authenticated (#151).
	const staffHeaders = await signIn(request, API_URL, staffIdToken);

	const createClient = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
		headers: staffHeaders,
		data: { name: 'Pat Client', email: clientEmail }
	});
	const createClientBody = await createClient.text();
	expect(createClient.ok(), `create client failed: ${createClient.status()} ${createClientBody}`).toBe(true);
	const { clientId, engagementId } = JSON.parse(createClientBody);

	const clientSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: clientEmail, password: PASSWORD, returnSecureToken: true } }
	);
	expect(clientSignUp.ok(), `clientSignUp failed: ${clientSignUp.status()} ${await clientSignUp.text()}`).toBe(true);
	const { localId: clientUID } = await clientSignUp.json();
	seedClientPortalUser(clientUID, clientId);

	return { clientEmail, engagementId };
}

test('a Client signs out and can no longer reach their Engagement', async ({ page, request }) => {
	const practiceName = 'Meadowbrook Doulas';
	const { clientEmail, engagementId } = await seedPortalClient(request, practiceName);

	await page.goto('/portal/login');
	await page.getByLabel('Email').fill(clientEmail);
	await page.getByLabel('Password').fill(PASSWORD);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
	await expect(page.getByRole('heading', { name: `Welcome to ${practiceName}` })).toBeVisible();

	// The control is in the portal authenticated layout, so it is on this
	// screen and on every other one under portal/engagements/[engagementId].
	await page.getByRole('button', { name: 'Sign out' }).click();

	// The *portal* login screen, not the Staff one at /login -- and the
	// plain one, with no sessionEnded flag: that is api.ts's "your session
	// expired under you" path, not a deliberate sign-out.
	await expect(page).toHaveURL(/\/portal\/login$/);

	// The cookie was the browser's only credential, so it is gone with the
	// session.
	const cookies = await page.context().cookies();
	expect(
		cookies.find((c) => c.name === '__session'),
		'browser still holds a __session cookie after signing out'
	).toBeFalsy();

	// Pressing Back is the case that matters for a borrowed phone: a
	// history pop, not a fresh navigation. Nothing on the Engagement screen
	// renders before its own authenticated fetch answers (the page has no
	// SSR'd copy to fall back on), so the 401 bounces the pop to the portal
	// login screen with no pregnancy or birth information ever painted.
	await page.goBack();
	await expect(page).toHaveURL(/\/portal\/login\?sessionEnded=true$/);
	await expect(page.getByText(`Welcome to ${practiceName}`)).toHaveCount(0);

	// And a fresh navigation to the Engagement is refused the same way.
	await page.goto(`/portal/engagements/${engagementId}`);
	await expect(page).toHaveURL(/\/portal\/login\?sessionEnded=true$/);
	await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible();
	await expect(page.getByText(`Welcome to ${practiceName}`)).toHaveCount(0);
});

// The stale-tab case from #153: two tabs share one cookie, so the second
// tab signs out against a session the first already ended. It must land on
// the portal login screen like any other sign-out, not report an error --
// the end-session endpoint is idempotent.
test('a Client second tab signing out after the first shows no error', async ({ page, request }) => {
	const { clientEmail, engagementId } = await seedPortalClient(request, 'Fernwood Doulas');

	await page.goto('/portal/login');
	await page.getByLabel('Email').fill(clientEmail);
	await page.getByLabel('Password').fill(PASSWORD);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	// A second tab on the same browser context, so it carries the same
	// __session cookie -- and holds it after the first tab signs out.
	const staleTab = await page.context().newPage();
	await staleTab.goto(`/portal/engagements/${engagementId}`);
	await expect(staleTab.getByRole('button', { name: 'Sign out' })).toBeVisible();

	await page.getByRole('button', { name: 'Sign out' }).click();
	await expect(page).toHaveURL(/\/portal\/login$/);

	await staleTab.getByRole('button', { name: 'Sign out' }).click();

	await expect(staleTab).toHaveURL(/\/portal\/login$/);
	await expect(staleTab.getByRole('alert')).toHaveCount(0);
});
