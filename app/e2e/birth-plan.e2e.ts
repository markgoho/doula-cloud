import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT } from './ports';
import { seedClientPortalUser, seedEngagement } from './stack';
import { signInEnrolled, enterPracticeAsEnrolled } from './mfa';
import { signInPortalClient } from './portalClient';
import { seedFoundingOwner } from './staffSignup';

// Exercises #65's critical path: Staff fills out a Birth Plan for an
// Engagement (through the real staff-side UI from #64), then the Client
// portal shows the matching read-only view. Provisions Practice/Staff and
// the Client-portal account the same way client-portal-login.e2e.ts does
// -- this test isn't re-proving login itself, just that both sides of the
// Birth Plan feature agree.
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

test('Staff fills a Birth Plan, and the Client portal shows the matching read-only view', async ({
	page,
	request,
	context
}) => {
	// Random suffix, not just Date.now(): see staffSignup.ts for why
	// millisecond-only uniqueness collides across parallel workers.
	const clientEmail = `client-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;

	const { idToken: staffIdToken, localId: staffUID, practiceId } = await seedFoundingOwner(request);

	// #606: an Owner is gated behind a second factor at every Practice-scoped
	// route (see mfa.ts's signInEnrolled doc comment), including the
	// createClient call right below.
	const staffHeaders = await signInEnrolled(request, staffIdToken, staffUID);

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
	await enterPracticeAsEnrolled(context, page, staffHeaders, practiceId);
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

	// Client side: a Portal Account (#617) linked to the same Client via
	// client_portal_users, viewing the read-only Birth Plan.
	seedClientPortalUser(clientEmail, clientId);

	await signInPortalClient(page, request, clientEmail);
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	// Scoped to the page: since the shell landed (#452) the portal nav
	// carries a Birth plan link too, and both are correct.
	await page.getByRole('main').getByRole('link', { name: 'Birth plan' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}/birth-plan$`));
	await expect(page.getByText('Soft lighting, calm music')).toBeVisible();

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
	// staffHeaders now 401s). A fresh signInEnrolled call, issued over
	// `request` rather than through `page`, mints an unrelated session
	// nothing here evicts.
	const freshStaffHeaders = await signInEnrolled(request, staffIdToken, staffUID);
	const staffPlanResponse = await request.get(
		`${API_URL}/api/practices/${practiceId}/engagements/${engagementId}/plans/birth_plan`,
		{ headers: freshStaffHeaders }
	);
	expect(staffPlanResponse.ok(), `Staff birth plan read failed: ${staffPlanResponse.status()}`).toBe(true);
	const staffPlanBody = await staffPlanResponse.json();
	expect(staffPlanBody.clientAcknowledgedAt, 'Staff read should show the Client acknowledgement').toBeTruthy();
});
