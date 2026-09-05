import { expect, test } from '@playwright/test';
import { seedPortalClient, signInPortalClient } from './portalClient';

test('Client-portal login lands on their engagement-scoped URL', async ({ page, request }) => {
	const practiceName = 'Riverside Doulas';
	const { clientEmail, engagementId } = await seedPortalClient(request, practiceName);

	// #617: a Client has no password and no Identity Platform account any
	// more -- signInPortalClient walks the real magic-link round trip
	// (request, drain, open the mailbox, click the link, press Continue).
	await signInPortalClient(page, request, clientEmail);

	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
	await expect(page.locator('h1')).toHaveText(`Welcome to ${practiceName}`);

	// HttpOnly means no script on the page -- including one smuggled in by
	// an XSS bug -- can read the session credential this way. Name only,
	// never a value: see session-cookie.e2e.ts for the attribute checks.
	const readableCookies = await page.evaluate(() => document.cookie);
	expect(readableCookies, 'session cookie is readable from document.cookie').not.toContain('__session');

	// Closing the tab and returning within the session lifetime leaves the
	// person signed in: the __session cookie lives on the browser context,
	// not the tab, so a fresh page navigating straight to the Engagement
	// URL should land there without a redirect to /portal/login.
	await page.close();
	const reopened = await page.context().newPage();
	await reopened.goto(`/portal/engagements/${engagementId}`);
	await expect(reopened).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
	await expect(reopened.locator('h1')).toHaveText(`Welcome to ${practiceName}`);
});
