import { expect, test } from '@playwright/test';
import { API_URL } from './ports';
import { enterPractice, signIn } from './auth';
import { seedFoundingOwner } from './staffSignup';

// birth-plan.e2e.ts creates its Client with POST /api/practices/{id}/clients
// directly -- fixture setup, not automation of the Add Client form itself
// (#207's rule). This spec walks a new Client through the UI (#497), from
// the empty Practice to her Engagement's page (#1611), and then the
// Visits section there.
test('The first Client in three presses, then the Visits section', async ({ page, request, context }) => {
	const { idToken, practiceId } = await seedFoundingOwner(request);

	const staffHeaders = await signIn(request, API_URL, idToken);
	await enterPractice(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// docs/onboarding-route.md: from the empty Practice to First Value is
	// three presses on three screens (#1516, #1611). A press is a button or
	// a link; what she types and chooses inside a form is the answer, not a
	// press. The founding Owner is the only Doula at her Practice, so "Who
	// is the Doula?" opens answered and adds no press.

	// #1612: before she starts, the empty Practice says what uses a Credit
	// and how many the Practice has. Signup wrote the signup bonus and
	// nothing else, so the real ledger reads as Welcome credits.
	await expect(
		page.getByText(
			'Adding a Client is free. Starting work with a Client uses 1 Credit, and this Practice has 3 Welcome credits.',
			{ exact: true }
		)
	).toBeVisible();

	// Press 1, on the empty Practice: it opens the name question, with no
	// search in front of it (#1609).
	await page.getByRole('link', { name: 'Add your first Client' }).click();
	await expect(page.getByRole('heading', { level: 1, name: "What is the Client's name?" })).toBeVisible();
	await page.getByLabel('Given name').fill('Pat');
	await page.getByLabel('Family name (optional)').fill('Client');

	// Press 2, on the name question: it saves the Client and opens the
	// Start work form for her.
	await page.getByRole('button', { name: 'Save and continue' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/clients/[^/]+/engagement-requests/new$`));
	// #1710: the form says the save happened, and the message has focus.
	await expect(page.getByText('Pat saved as a Client.')).toBeFocused();
	await expect(page.getByRole('link', { name: "Go to Pat Client's record without starting work" })).toBeVisible();
	await expect(
		page.getByText(
			'Starting work with Pat Client uses 1 Credit. This Practice has 3 Welcome credits. After this, it has 2.',
			{ exact: true }
		)
	).toBeVisible();
	await page.getByLabel('Birth').check();
	await page.getByLabel('Due date').fill('2027-03-01');

	// Press 3, on the Start work form: an Owner is her own approver, so the
	// start is approved and the flow ends on the Engagement's page.
	await page.getByRole('button', { name: 'Start work with Pat Client' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/engagements/[^/?]+$`));
	await expect(page.getByText('Work with Pat started.')).toBeVisible();
	await expect(page.getByRole('heading', { level: 1, name: 'Pat' })).toBeVisible();
	// #1710: the message is shown once. A reload does not say it again.
	await page.reload();
	await expect(page.getByRole('heading', { level: 1, name: 'Pat' })).toBeVisible();
	await expect(page.getByText('Work with Pat started.')).toHaveCount(0);

	// #1511: the new Engagement has no Visit, Contract or Invoice yet, so
	// Schedule, Contracts and Invoices have no row for it. The Clients
	// list and her record each link to it.
	const engagementUrl = page.url();
	await page.getByRole('link', { name: 'Clients', exact: true }).click();
	await page.getByRole('link', { name: 'Birth', exact: true }).click();
	await expect(page).toHaveURL(engagementUrl);
	await page.getByRole('link', { name: 'Clients', exact: true }).click();
	await page.getByRole('link', { name: /^Pat/ }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/clients/[^/?]+$`));
	await page.getByRole('link', { name: 'Birth', exact: true }).click();
	await expect(page).toHaveURL(engagementUrl);
	await expect(page.getByRole('heading', { level: 1, name: 'Pat' })).toBeVisible();

	// DataTable renders a <table> and a card-view <dl> together for every
	// row (#564, responsive layout) -- getByRole('cell', ...) targets the
	// <table> tree specifically, since a plain getByText match on either
	// empty message is ambiguous between the two trees. Wider than the
	// Visits table's own floor (#1945), so the <table> is the tree on screen.
	await page.setViewportSize({ width: 3200, height: 900 });
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
