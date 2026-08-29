import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { signIn } from './auth';
import { seedEngagement } from './stack';

const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

// Drives ADR-0008's Offer flow (#317) through the two screens it adds:
// the make-an-offer panel on the Engagement, and the Doula's own inbox.
// offer.ts, OfferSection.svelte, and OfferInbox.svelte all have their own
// Vitest coverage; this is the only test that renders the real routes and
// hits the real API, proving the Offer lands, appears to the person
// offered, and mints her attachment when she takes it.
//
// One person plays both parts, and that is a real state rather than a
// shortcut: ADR-0008 has employment type and role as separate axes, so an
// Owner who also does the work holds `{owner,doula}`. It keeps the fixture
// to one sign-in without touching the database, which is the only other
// way to get a second Staff member in (the Staff invitation's token is
// deliberately mailed and never returned by the API).
test('Owner offers an Engagement to a Doula, who accepts it from her own inbox', async ({
	page,
	request
}) => {
	// Random suffix, not just Date.now(): see staff-login.e2e.ts for why
	// millisecond-only uniqueness collides across parallel workers.
	const email = `offer-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const password = 'password123';

	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password, returnSecureToken: true } }
	);
	expect(signUp.ok(), `signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken } = await signUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${idToken}` },
		data: { practiceName: 'Riverside Doulas', staffName: 'Jamie Owner', staffEmail: email , workState: 'NY' }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `staff signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId, staffId } = JSON.parse(signupBody);

	const headers = await signIn(request, API_URL, idToken);

	// She does the work as well as running the practice, and as a
	// contractor -- which is what makes the fee required.
	const membership = await request.patch(
		`${API_URL}/api/practices/${practiceId}/staff/${staffId}/membership`,
		{ headers, data: { roles: ['owner', 'doula'], employmentType: 'contractor' } }
	);
	expect(
		membership.ok(),
		`membership edit failed: ${membership.status()} ${await membership.text()}`
	).toBe(true);

	const createClient = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
		headers,
		data: { givenName: 'Rosa', familyName: 'Martinez', email: `client-${Date.now()}@example.com` }
	});
	const createClientBody = await createClient.text();
	expect(createClient.ok(), `create client failed: ${createClient.status()} ${createClientBody}`).toBe(true);
	const { id: clientId } = JSON.parse(createClientBody);
	const engagementId = seedEngagement(clientId, practiceId);

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	await expect(page.getByRole('heading', { name: 'Offers' })).toBeVisible();
	await expect(page.getByText('Nobody has been offered this work yet.')).toBeVisible();

	await page.getByLabel('Jamie Owner').check();
	await page.getByLabel('Fee (USD)').fill('450');
	await page.getByLabel('General area').fill('North side');
	await page.getByLabel('Due date').fill('2027-01-04');
	await page.getByLabel('Terms').fill('Two prenatal visits, on call from 38 weeks.');
	await page.getByRole('button', { name: 'Send Offer' }).click();

	// The Practice side now names who was asked and what she has said.
	await expect(page.getByText('Awaiting a decision')).toBeVisible();
	await expect(page.getByText('$450.00')).toBeVisible();

	// And her own inbox carries the four decidable facts, with no Client
	// name anywhere on it.
	await page.goto(`/practices/${practiceId}/offers`);
	await expect(page.getByRole('heading', { name: 'Your offers' })).toBeVisible();
	await expect(page.getByText('North side')).toBeVisible();
	await expect(page.getByText('2027-01-04')).toBeVisible();
	await expect(page.getByText('Two prenatal visits, on call from 38 weeks.')).toBeVisible();
	await expect(page.getByText('Rosa Martinez')).toHaveCount(0);

	await page.getByRole('button', { name: 'Accept' }).click();
	await expect(page.getByText('Accepted')).toBeVisible();
	await expect(page.getByRole('button', { name: 'Accept' })).toHaveCount(0);
	// #230's terminal rule, through the real stack: the Client's own
	// fields stop being served the moment the Offer leaves 'offered', so
	// the row keeps her fee and loses the area and the date.
	await expect(page.getByText('North side')).toHaveCount(0);
	await expect(page.getByText('2027-01-04')).toHaveCount(0);
	await expect(page.getByText('$450.00')).toBeVisible();

	// And the Practice side reads the same answer back.
	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	await expect(page.getByText('Accepted')).toBeVisible();
});
