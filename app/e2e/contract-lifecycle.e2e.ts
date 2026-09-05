import { expect, test } from '@playwright/test';
import { seedPortalClient, PORTAL_CLIENT_PASSWORD } from './portalClient';
import { enterPracticeAsEnrolled } from './mfa';

// Contract signing became testable at #234, which put a fake-gcs-server in
// compose.e2e.yaml -- before that the object store made signing 500. This is
// the first spec to walk the Contract lifecycle: build and send on the
// Practice side, the Client signing in the portal, and the Signed PDF
// coming back afterward.
test('A Contract can be built, sent, signed by the Client, and its Signed PDF retrieved', async ({
	page,
	browser
}) => {
	// Fixture setup, not the seam under test: provisions a Practice, a Staff
	// Owner, a Client + Engagement, and a Client-portal account, exactly as
	// client-portal-login.e2e.ts does.
	const { practiceId, engagementId, staffHeaders, clientEmail } = await seedPortalClient(
		page.request,
		'Riverside Doulas'
	);

	// #606: seedPortalClient's Owner is already enrolled (portalClient.ts),
	// so entering her Practice is a cookie injection rather than the plain
	// /login form -- see mfa.ts's enterPracticeAsEnrolled doc comment.
	//
	// The Practice side: build and send, walked through the real Engagement
	// screen.
	await enterPracticeAsEnrolled(page.context(), page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	await page.getByRole('button', { name: 'Create Draft Contract' }).click();
	await expect(page.getByText('Status: draft')).toBeVisible();

	// client_name is prefilled by createContract (contract.go's
	// prefillClientName); practice_name is not, so filling it in is what
	// proves the form's own values round-trip through Save.
	await page.getByLabel('Practice name').fill('Riverside Doulas');
	await page.getByRole('button', { name: 'Save Contract' }).click();

	await page.getByRole('button', { name: 'Send Contract' }).click();
	await expect(page.getByText('Status: sent')).toBeVisible();

	// The Client side: a separate browser context, since the Staff and
	// Client sessions share the one __session cookie name and would
	// otherwise clobber each other on the same origin.
	const clientContext = await browser.newContext();
	const clientPage = await clientContext.newPage();
	await clientPage.goto('/portal/login');
	await clientPage.getByLabel('Email').fill(clientEmail);
	await clientPage.getByLabel('Password').fill(PORTAL_CLIENT_PASSWORD);
	await clientPage.getByRole('button', { name: 'Log in' }).click();
	await expect(clientPage).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	// Scoped to #main: the shell's own nav carries a same-named "Contract"
	// link too.
	await clientPage.locator('#main').getByRole('link', { name: 'Contract' }).click();
	await expect(clientPage.getByText('Status: sent')).toBeVisible();
	await clientPage.getByRole('button', { name: 'I agree to sign electronically, continue' }).click();
	await clientPage.getByLabel('Full legal name').fill('Pat Client');
	await clientPage
		.getByLabel('I have read this Contract and I am signing it electronically')
		.check();
	const [signResponse] = await Promise.all([
		clientPage.waitForResponse(
			(response) => response.url().includes('/contract/sign') && response.request().method() === 'POST'
		),
		clientPage.getByRole('button', { name: 'Sign' }).click()
	]);
	expect(signResponse.ok(), `sign failed: ${signResponse.status()}`).toBe(true);
	await expect(clientPage.getByText('Status: signed')).toBeVisible();
	await clientContext.close();

	// The Signed PDF comes back on the Practice side's own endpoint
	// (staffauth ownerAndAdmin-gated) -- proven directly, since neither
	// screen renders a download link for it yet. A relative path, not
	// the BFF's own host directly: the __session cookie is scoped to the
	// app's own origin, which vite's own proxy forwards /api/* through to
	// the BFF (vite.config.ts) -- a request straight to the BFF's host
	// would carry no cookie at all.
	const staffPdf = await page.request.get(
		`/api/practices/${practiceId}/engagements/${engagementId}/contract/pdf`
	);
	expect(staffPdf.ok(), `staff pdf fetch failed: ${staffPdf.status()} ${await staffPdf.text()}`).toBe(true);
	expect(staffPdf.headers()['content-type']).toBe('application/pdf');
});
