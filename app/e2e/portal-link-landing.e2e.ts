import { expect, test } from '@playwright/test';
import { PREVIEW_SERVER_ORIGIN } from './ports';
import { PORTAL_CLIENT_PASSWORD, seedPortalClient } from './portalClient';

// #138's "Cookie attributes and lifetime" section: SameSite=Lax was chosen
// over Strict specifically because a Client portal link arrives by email,
// which the browser treats as a cross-site top-level navigation, and
// Strict withholds the cookie on that request.
//
// page.goto() has no initiator and is indistinguishable from typing the
// URL directly -- it would pass this test under Strict too. A data: URL
// has its own opaque origin, so clicking a plain <a> from one is a
// genuinely cross-site-initiated top-level GET, the shape a real email
// link produces (confirmed empirically against a throwaway probe server
// before writing this): Chromium withholds a Strict cookie on that
// top-level document request.
//
// That said, this app is a static SPA (adapter-static, no SSR) -- the
// document request itself never needs the cookie, only the same-origin
// `fetch('/api/portal/session')` the loaded page makes afterwards, and a
// same-origin fetch is always sent regardless of SameSite or how the tab
// got there. So today, this test would still pass even under
// SameSite=Strict: it proves the AC's literal wording ("arrives
// authenticated"), but it is not yet the regression guard #138 calls out
// for hardening that attribute. It becomes one the day any response this
// flow depends on is gated behind the cookie at the HTTP layer (e.g. SSR,
// or a direct top-level link to a BFF-served resource).
test('a signed-in Client following a portal link from another site arrives authenticated', async ({
	page,
	request
}) => {
	const practiceName = 'Cedar Grove Doulas';
	const { clientEmail, engagementId } = await seedPortalClient(request, practiceName);

	await page.goto('/portal/login');
	await page.getByLabel('Email').fill(clientEmail);
	await page.getByLabel('Password').fill(PORTAL_CLIENT_PASSWORD);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	const engagementURL = `${PREVIEW_SERVER_ORIGIN}/portal/engagements/${engagementId}`;
	await page.goto(`data:text/html,<a id="link" href="${engagementURL}">go</a>`);
	await page.click('#link');

	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
	await expect(page.getByRole('heading', { name: `Welcome to ${practiceName}` })).toBeVisible();
});
