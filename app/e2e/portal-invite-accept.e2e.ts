import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT } from './ports';
import { seedEngagement } from './stack';
import { signInEnrolled } from './mfa';
import { seedFoundingOwner } from './staffSignup';

const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

// This test drives the real provisioning path #90 added -- invite via the
// BFF, accept through the UI, then land in the portal -- rather than
// stack.ts's seedClientPortalUser fixture, which portalClient.ts's
// seedPortalClient keeps using for specs that need a signed-in Client
// but aren't proving provisioning itself. #617: no email or password
// step any more -- the invitation is the first magic link, so accepting
// it is nothing but pressing Continue.
test('Client-portal invite -> accept -> login lands on their engagement-scoped URL', async ({
	page,
	request
}) => {
	// Random suffix, not just Date.now(): see staffSignup.ts for why
	// millisecond-only uniqueness collides across parallel workers.
	const clientEmail = `client-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;

	const { idToken: staffIdToken, localId: staffUID, practiceId } = await seedFoundingOwner(request);

	// #606: an Owner is gated behind a second factor at every Practice-scoped
	// route (see mfa.ts's signInEnrolled doc comment), including the
	// createClient and portal-invite calls below.
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

	const invite = await request.post(
		`${API_URL}/api/practices/${practiceId}/engagements/${engagementId}/portal-invite`,
		{ headers: staffHeaders }
	);
	const inviteBody = await invite.text();
	expect(invite.ok(), `portal invite failed: ${invite.status()} ${inviteBody}`).toBe(true);
	const { inviteToken } = JSON.parse(inviteBody);

	await page.goto(`/portal/accept-invite?token=${inviteToken}`);
	await page.getByRole('button', { name: 'Continue' }).click();

	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
	await expect(page.locator('h1')).toHaveText('Welcome to Riverside Doulas');
});
