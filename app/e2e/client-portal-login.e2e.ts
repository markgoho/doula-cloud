import { expect, test } from '@playwright/test';
import { openMagicLink, seedPortalClient, signInPortalClient } from './portalClient';
import { enterPracticeAsEnrolled } from './mfa';
import { E2E_API_HOST, E2E_API_PORT } from './ports';

const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

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

/*
 * #610: a browser holds exactly one Doula Cloud session, so a doula who
 * is also a Client cannot be signed into both at once. She is told what
 * continuing costs before it happens, and the Practice session she
 * leaves is deleted rather than left live behind a cookie she no longer
 * holds.
 */
test('Signing into the portal over a live Staff session warns first, then evicts it', async ({
	page,
	context,
	request
}) => {
	const { clientEmail, engagementId, practiceId, staffHeaders } = await seedPortalClient(
		request,
		'Riverside Doulas'
	);

	// The same browser that is about to redeem a sign-in link is already
	// signed in to her Practice.
	await enterPracticeAsEnrolled(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await openMagicLink(page, request, clientEmail);
	await page.getByRole('button', { name: 'Continue' }).click();

	// Warned, not signed in: the link is not spent and the Practice
	// session is untouched until she says so.
	await expect(
		page.getByText('Continuing signs you out of your Practice in this browser.')
	).toBeVisible();
	await expect(page).toHaveURL(/\/portal\/sign-in\?token=/);
	const stillLive = await request.get(`${API_URL}/api/staff/session`, { headers: staffHeaders });
	expect(stillLive.ok(), 'the refused sign-in ended the Staff session anyway').toBe(true);

	await page.getByRole('button', { name: 'Continue and sign out' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	// Deleted, not left to expire: presenting the evicted token again
	// reaches nothing.
	const evicted = await request.get(`${API_URL}/api/staff/session`, { headers: staffHeaders });
	expect(evicted.status(), 'the evicted Staff session still verifies').toBe(401);
});
