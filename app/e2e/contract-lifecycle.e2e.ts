import { expect, test } from '@playwright/test';
import { seedPortalClient, signInPortalClient } from './portalClient';
import { enterPractice } from './auth';

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

	// Entering her Practice is a cookie injection rather than the /login
	// form, which this spec does not test -- see auth.ts's enterPractice
	// doc comment.
	//
	// The Practice side: build and send, walked through the real Engagement
	// screen.
	await enterPractice(page.context(), page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	// #2004: creating the Draft opens its merge fields at once.
	await page.getByRole('button', { name: 'Create a Draft Contract' }).click();
	await expect(page.getByRole('heading', { name: 'Change the Draft Contract' })).toBeVisible();

	// #258: client_name and practice_name are both resolved by
	// createContract (contract.go's resolveMergeFieldValues) from data the
	// product already holds, and practice_name already reads Riverside
	// Doulas -- proven directly, rather than filled in here. #967 resolves
	// price the same way, from the rate card seedPortalClient seeds
	// (portalClient.ts), so ContractForm renders no input for it at all
	// (editableMergeFields). The default seeded template's other two merge
	// fields (scope_of_service, the two engagement dates) have no such
	// column backing them, so Send's completeness check refuses until
	// they're filled by hand.
	await expect(page.getByLabel('Practice name')).toHaveValue('Riverside Doulas');
	await page.getByLabel('Scope of service').fill('12 prenatal visits');
	await page.getByLabel('Engagement start date').fill('2027-01-01');
	await page.getByLabel('Engagement end date').fill('2027-06-01');
	await page.getByRole('button', { name: 'Save Contract' }).click();
	await expect(page.getByText('Contract saved.')).toBeVisible();

	await page.getByRole('button', { name: 'Send Contract' }).click();
	await expect(page.getByText('Sent, waiting for the Client to sign')).toBeVisible();

	// The Client side: a separate browser context, since the Staff and
	// Client sessions share the one __session cookie name and would
	// otherwise clobber each other on the same origin.
	const clientContext = await browser.newContext();
	const clientPage = await clientContext.newPage();
	await signInPortalClient(clientPage, clientPage.request, clientEmail);
	await expect(clientPage).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	// The hub's Contract card (#2007), which asks her to sign one that
	// waits on her. Scoped to #main beside the shell's own nav.
	await clientPage.locator('#main').getByRole('link', { name: 'Read and sign your contract' }).click();
	// #212: the Client register's own label, not the Staff ContractStatus
	// component's raw "Status: sent".
	await expect(clientPage.getByText('Ready for your signature')).toBeVisible();
	await clientPage.getByRole('button', { name: 'I agree to sign electronically, continue' }).click();
	await clientPage.getByLabel('Full legal name').fill('Pat Client');
	await clientPage
		.getByLabel('I have read this contract and I am signing it electronically')
		.check();
	const [signResponse] = await Promise.all([
		clientPage.waitForResponse(
			(response) => response.url().includes('/contract/sign') && response.request().method() === 'POST'
		),
		clientPage.getByRole('button', { name: 'Sign' }).click()
	]);
	expect(signResponse.ok(), `sign failed: ${signResponse.status()}`).toBe(true);
	// #212: the Client register's label -- "Signed" reads the same as the
	// raw enum value here, but it comes from clientRegister.ts rather than
	// the Staff ContractStatus component's "Status: signed".
	await expect(clientPage.getByText('Signed', { exact: true })).toBeVisible();
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
