import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { readStaffInviteToken } from './stack';

// The Firebase Auth emulator and the Go BFF -- both host processes -- see
// e2e/global-setup.ts and e2e/stack.ts for how these get started.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

// Every other spec that signs in does so as the Owner signup itself creates,
// who holds owner + office_manager + doula at once. This is the one spec
// that reaches the app as a Staff member holding a role set that is not
// that bundle -- a Doula invited and accepted through the real invite
// route -- and the one spec that asserts a role refusal (a 403 where a
// role rule says there must be one), rather than only success paths.
test('A Doula invited via the Staff invite route is refused an Owner-only action', async ({ page, request }) => {
	// Random suffix, not just Date.now(): see staff-login.e2e.ts for why
	// millisecond-only uniqueness collides across parallel workers.
	const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	const ownerEmail = `owner-${unique}@example.com`;
	const doulaEmail = `doula-${unique}@example.com`;
	const password = 'password123';

	// Fixture setup, not the seam under test (#207): the Owner side of this
	// spec is provisioned the way every other spec provisions its Practice.
	const ownerSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: ownerEmail, password, returnSecureToken: true } }
	);
	expect(ownerSignUp.ok(), `ownerSignUp failed: ${ownerSignUp.status()} ${await ownerSignUp.text()}`).toBe(true);
	const { idToken: ownerIdToken } = await ownerSignUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${ownerIdToken}` },
		data: { practiceName: 'Riverside Doulas', staffName: 'Jamie Owner', staffEmail: ownerEmail, workState: 'NY' }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	// The invite send is walked through the real screen (#189's "Invite a
	// Staff member" route, otherwise untouched by the suite): the default
	// role selection is 'doula' alone, which is the non-Owner bundle this
	// spec needs.
	await page.goto('/login');
	await page.getByLabel('Email').fill(ownerEmail);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await page.goto(`/practices/${practiceId}/invite`);
	await page.getByLabel('Their email').fill(doulaEmail);
	const [inviteResponse] = await Promise.all([
		page.waitForResponse((response) => response.url().endsWith('/staff/invitations') && response.request().method() === 'POST'),
		page.getByRole('button', { name: 'Send invite' }).click()
	]);
	expect(inviteResponse.ok(), `invite send failed: ${inviteResponse.status()}`).toBe(true);
	const { invitationId } = await inviteResponse.json();
	await expect(
		page.getByText(`Invited. An email with a link to join is on its way to ${doulaEmail}.`)
	).toBeVisible();

	// staffinvite.Queue never gets mailed in the e2e stack -- the token sits
	// on the pending outbox row, which is where readStaffInviteToken reads
	// it from (stack.ts).
	const inviteToken = readStaffInviteToken(invitationId);
	expect(inviteToken, `no pending staff_invite_outbox row for invitation ${invitationId}`).toBeTruthy();

	// Accepting is walked through the real screen too (#437's two-step
	// form): a brand-new person, so the signup branch and both questions on
	// step two.
	await page.goto(`/accept-invite?token=${inviteToken}`);
	await page.getByLabel('Email').fill(doulaEmail);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Continue' }).click();

	await expect(page.getByRole('heading', { name: 'Tell us about yourself' })).toBeVisible();
	await page.getByLabel('Your name').fill('Robin Doula');
	await page.getByLabel('Which state do you work from?').selectOption('New York');
	await page.getByRole('button', { name: 'Accept invite' }).click();

	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));
	await expect(page.locator('h1')).toHaveText('Welcome to Riverside Doulas');

	// Still signed in as the Doula: the same Owner-only screen that just
	// worked for Jamie refuses Robin outright -- RequireOwner's 403, the
	// role rule PR-B2 names in employed-doula.md's permission boundary.
	await page.goto(`/practices/${practiceId}/invite`);
	await page.getByLabel('Their email').fill(`someone-else-${unique}@example.com`);
	await page.getByRole('button', { name: 'Send invite' }).click();
	await expect(page.getByText('only a Practice Owner can do that')).toBeVisible();
});
