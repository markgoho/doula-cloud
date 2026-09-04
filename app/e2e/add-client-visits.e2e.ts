import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { seedEngagement } from './stack';

const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

// birth-plan.e2e.ts creates its Client with POST /api/practices/{id}/clients
// directly -- fixture setup, not automation of the Add Client form itself
// (#207's rule). This is the first spec to walk the intake form (#497) and
// the Visits section through the UI.
test('Add Client form and Visits section', async ({ page, request }) => {
	const email = `add-client-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const password = 'password123';

	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password, returnSecureToken: true } }
	);
	expect(signUp.ok(), `signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken } = await signUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${idToken}` },
		data: { practiceName: 'Riverside Doulas', staffName: 'Jamie Owner', staffEmail: email, workState: 'NY' }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// The three-page intake sequence (#497, ADR-0017): name, then contact,
	// then date of birth, then the save.
	await page.goto(`/practices/${practiceId}/clients/new`);
	await page.getByLabel('Given name').fill('Pat');
	await page.getByLabel('Family name').fill('Client');
	await page.getByRole('button', { name: "Add contact details" }).click();

	await page.getByLabel('Email').fill(`client-${Date.now()}@example.com`);
	await page.getByRole('button', { name: /Add Pat's date of birth/ }).click();

	await page.getByRole('button', { name: /Save Pat's record/ }).click();

	// A brand-new Client with no prior match, so the save lands straight on
	// her Client detail hub (#494), never through the match-review steps.
	// The id segment is matched narrowly, excluding "new" itself, since the
	// intake route's own URL would otherwise satisfy a looser pattern before
	// the save navigates away from it.
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/clients/(?!new$)[^/]+$`));
	const clientId = new URL(page.url()).pathname.split('/').pop()!;

	// No spec built or read an Engagement's Visits section through the UI
	// before this (#207): seeding the Engagement directly is fixture setup
	// for that section, exactly the way birth-plan.e2e.ts seeds its own.
	const engagementId = seedEngagement(clientId, practiceId);

	// DataTable renders a <table> and a card-view <dl> together for every
	// row (#564, responsive layout) -- getByRole('cell', ...) targets the
	// <table> tree specifically, since a plain getByText match on either
	// empty message is ambiguous between the two trees.
	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	await expect(page.getByRole('cell', { name: 'No Visits yet.' })).toBeVisible();
	await page.getByRole('button', { name: 'Add a Visit' }).click();

	// Scoped to the Visits table, and exact: true within it. Two things
	// name the same Staff member on this screen: the Reassign cell's own
	// visually-hidden text, which `exact` excludes, and the per-record
	// Activity ledger #486 added after this test was written, which it
	// does not -- the ledger records who added the Visit, under the same
	// name, in its own table.
	await expect(
		page.getByLabel('Visits').getByRole('cell', { name: 'Jamie Owner', exact: true })
	).toBeVisible();
});
