import { expect, test } from '@playwright/test';
import { seedEngagement } from './stack';
import { signInEnrolled, enterPracticeAsEnrolled } from './mfa';
import { seedFoundingOwner } from './staffSignup';

// birth-plan.e2e.ts creates its Client with POST /api/practices/{id}/clients
// directly -- fixture setup, not automation of the Add Client form itself
// (#207's rule). This is the first spec to walk the intake form (#497) and
// the Visits section through the UI.
test('Add Client form and Visits section', async ({ page, request, context }) => {
	const { idToken, localId, practiceId } = await seedFoundingOwner(request);

	// #606: an Owner is gated behind a second factor at every Practice-scoped
	// route, so entering her own Practice can no longer be driven through the
	// plain /login form (see mfa.ts's signInEnrolled doc comment).
	const staffHeaders = await signInEnrolled(request, idToken, localId);
	await enterPracticeAsEnrolled(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// Intake is one question per route (#466, ADR-0017). Only the name is
	// answered here: ADR-0017 makes the save free with a given name alone,
	// and "Save and come back later" is the escape every question page
	// offers, so this walks the shortest real path through the sequence.
	await page.goto(`/practices/${practiceId}/clients/new`);
	await expect(page.getByRole('heading', { level: 1, name: /name\?$/ })).toBeVisible();
	await page.getByLabel('Given name').fill('Pat');
	await page.getByLabel('Family name').fill('Client');
	await page.getByRole('button', { name: 'Save and come back later' }).click();

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
