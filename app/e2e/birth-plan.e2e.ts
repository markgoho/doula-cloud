import { expect, test, type Page } from '@playwright/test';
import { API_URL } from './ports';
import { seedClientPortalUser, seedEngagement } from './stack';
import { enterPractice, signIn } from './auth';
import { signInPortalClient } from './portalClient';
import { seedFoundingOwner, uniqueEmail } from './staffSignup';

// Exercises #65's critical path: Staff fills out a Birth Plan for an
// Engagement (through the real staff-side UI from #64), then the Client
// portal shows the matching read-only view. Provisions Practice/Staff and
// the Client-portal account the same way client-portal-login.e2e.ts does
// -- this test isn't re-proving login itself, just that both sides of the
// Birth Plan feature agree.

/**
 * The printed Birth Plan, in print media (#2209): whose it is (her legal
 * name), whose Practice (seedFoundingOwner's default), and the day it last
 * changed. The day is the save this test just made, read in the Practice's
 * zone, so only its shape is asserted here; the unit specs and the BFF's
 * own tests pin the exact day.
 */
async function expectPrintedMasthead(page: Page) {
	await page.emulateMedia({ media: 'print' });
	const main = page.getByRole('main');
	try {
		await expect(main.getByText('Pat Client', { exact: true })).toBeVisible();
		await expect(main.getByText('Riverside Doulas', { exact: true })).toBeVisible();
		await expect(main.getByText('Last changed')).toBeVisible();
		await expect(main.getByText(/^[A-Z][a-z]{2} \d{1,2}, \d{4}$/)).toBeVisible();
		await expect(main.getByRole('button', { name: 'Print' })).toBeHidden();
	} finally {
		await page.emulateMedia({ media: 'screen' });
	}
}

test('Staff fills a Birth Plan, and the Client portal shows the matching read-only view', async ({
	page,
	request,
	context
}) => {
	const clientEmail = uniqueEmail('client');

	const { idToken: staffIdToken, practiceId } = await seedFoundingOwner(request);

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

	// Staff side: create the Birth Plan (signup seeds a default template
	// per Practice per #63), fill one field, and save.
	await enterPractice(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// The Clients list no longer links each row to an Engagement (#397 --
	// the Client detail page each row will link to is #400's separate,
	// not-yet-built screen), so this test's only route to the Engagement
	// is a direct navigation rather than the old click-through.
	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/engagements/${engagementId}$`));

	// #2004: starting the Birth Plan opens its questions at once.
	await page.getByRole('button', { name: 'Start the Birth Plan' }).click();
	await page
		.getByLabel('Preferences for atmosphere (music, lighting, etc.)')
		.fill('Soft lighting, calm music');
	await page.getByRole('button', { name: 'Save Birth Plan' }).click();
	// The click only dispatches the event -- handleSavePlan's PUT is async,
	// and the status that says so appears only once it resolves. Wait for
	// it so the save has actually round-tripped before this test switches
	// to the Client-portal session below.
	await expect(page.getByText('Birth Plan saved.')).toBeVisible();

	// #2209: the Doula's printout names the Client in full, the Practice,
	// and when the plan last changed, and the controls stay off the paper.
	await page.goto(`/practices/${practiceId}/engagements/${engagementId}/birth-plan`);
	await expectPrintedMasthead(page);

	// Client side: a Portal Account (#617) linked to the same Client via
	// client_portal_users, viewing the read-only Birth Plan.
	seedClientPortalUser(clientEmail, clientId);

	await signInPortalClient(page, request, clientEmail);
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	// The hub's own Birth Plan card (#2007), scoped to the page: the
	// portal nav carries a Birth plan link too (#452).
	await page.getByRole('main').getByRole('link', { name: 'Read your birth plan' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}/birth-plan$`));
	await expect(page.getByText('Soft lighting, calm music')).toBeVisible();
	// #2209: her own print says the same three things, with the portal
	// header, the only other place the Practice is named, hidden on paper.
	await expectPrintedMasthead(page);

	// #301: she confirms she's read it -- the one control this v1 gives
	// her -- and the confirmation replaces the button with a status line
	// naming when.
	await page.getByRole('button', { name: "I've read this" }).click();
	await expect(page.getByRole('status')).toContainText("You confirmed you've read this on");

	// Staff's own view picks up the same fact -- the AC4 visibility half
	// of #301. Re-authenticated fresh rather than reusing staffHeaders:
	// signInPortalClient above signed the Client in on this same `page`,
	// and #610's cross-population check (sessionmint.go) evicts whatever
	// session the browser's cookie already named the moment that Client
	// sign-in happened -- so the original Staff session is gone server-
	// side, not just shadowed client-side (a raw request with the old
	// staffHeaders now 401s). A fresh signIn call, issued over
	// `request` rather than through `page`, mints an unrelated session
	// nothing here evicts.
	const freshStaffHeaders = await signIn(request, API_URL, staffIdToken);
	const staffPlanResponse = await request.get(
		`${API_URL}/api/practices/${practiceId}/engagements/${engagementId}/plans/birth_plan`,
		{ headers: freshStaffHeaders }
	);
	expect(staffPlanResponse.ok(), `Staff birth plan read failed: ${staffPlanResponse.status()}`).toBe(true);
	const staffPlanBody = await staffPlanResponse.json();
	expect(staffPlanBody.clientAcknowledgedAt, 'Staff read should show the Client acknowledgment').toBeTruthy();
});
