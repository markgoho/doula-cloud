import { expect, test, type Page } from '@playwright/test';
import { seedInvoice } from './stack';
import { seedPortalClient, signInPortalClient } from './portalClient';

// #1568: at 320px every signed-in portal screen scrolled the whole
// document sideways, because the top bar's nav row laid five items on one
// line. The continuum sweep could not see it: it measures a route inside a
// frame, and the shell is not in the frame. This walks the real
// authenticated layout instead, signed in as a Client, so the shell and
// each route are measured together, the way she meets them.
//
// ADR-0024 makes 320px complete and usable a conformance commitment, so
// the width is the commitment itself, not a device.
test.use({ viewport: { width: 320, height: 640 } });

// The five destinations the authenticated layout passes for a birth
// Engagement, seedEngagement's own default kind, which offers a Birth plan.
const NAV_LABELS = ['Your care', 'Messages', 'Birth plan', 'Contract', 'Notifications'];

async function expectNoSidewaysScroll(page: Page): Promise<void> {
	const { scrollWidth, clientWidth } = await page.evaluate(() => ({
		scrollWidth: document.documentElement.scrollWidth,
		clientWidth: document.documentElement.clientWidth
	}));
	expect(scrollWidth, `${page.url()} scrolls sideways at 320px`).toBe(clientWidth);
}

test('no signed-in portal screen scrolls sideways at 320px', async ({ page, request }) => {
	const { practiceId, engagementId, clientEmail } = await seedPortalClient(request, 'Riverside Doulas');
	const invoiceId = seedInvoice(practiceId, engagementId, {
		reference: 'A4B2-0011',
		stripeInvoiceId: 'in_e2e_open_320'
	});

	await signInPortalClient(page, request, clientEmail);
	const hub = `/portal/engagements/${engagementId}`;
	await expect(page).toHaveURL(new RegExp(`${hub}$`));

	const nav = page.getByRole('navigation', { name: 'Your care' });
	for (const label of NAV_LABELS) {
		await expect(nav.getByRole('link', { name: label, exact: true })).toBeVisible();
	}

	// Reachable by keyboard, not only visible: Tab from the top of the page
	// meets every nav item in turn.
	const reached = new Set<string>();
	for (let press = 0; press < 12 && reached.size < NAV_LABELS.length; press++) {
		await page.keyboard.press('Tab');
		const name = await page.evaluate(() => {
			const focused = document.activeElement;
			return focused?.closest('nav') ? (focused.textContent?.trim() ?? '') : '';
		});
		if (NAV_LABELS.includes(name)) reached.add(name);
	}
	expect(reached).toEqual(new Set(NAV_LABELS));

	const routes = [
		hub,
		`${hub}/messages`,
		`${hub}/birth-plan`,
		`${hub}/contract`,
		`${hub}/notifications`,
		`${hub}/invoices`,
		`${hub}/invoices/${invoiceId}`,
		`${hub}/sign-in-address`
	];
	for (const path of routes) {
		await page.goto(path);
		// Settled before measuring: a route's own content loads after the
		// shell, and it is the loaded screen that has to fit.
		await page.waitForLoadState('networkidle');
		await expect(page.getByRole('main')).not.toBeEmpty();
		await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);
		await expectNoSidewaysScroll(page);
	}
});
