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

	// Each route beside the read its own mount ends on, named rather than
	// waited out with `networkidle` (mountSettled.ts says why): the loaded
	// screen is the one that has to fit, not the shell before its content.
	const api = `/api/portal/engagements/${engagementId}`;
	const routes: Array<{ path: string; read: string }> = [
		{ path: hub, read: `${api}/visits` },
		{ path: `${hub}/messages`, read: `${api}/messages` },
		{ path: `${hub}/birth-plan`, read: `${api}/birth-plan` },
		{ path: `${hub}/contract`, read: `${api}/contract` },
		{ path: `${hub}/notifications`, read: `${api}/notification-preference` },
		{ path: `${hub}/invoices`, read: `${api}/invoices` },
		{ path: `${hub}/invoices/${invoiceId}`, read: `${api}/invoices/${invoiceId}` },
		{ path: `${hub}/sign-in-address`, read: '/api/portal/session' }
	];
	for (const { path, read } of routes) {
		const settled = page.waitForResponse((response) => new URL(response.url()).pathname === read);
		await page.goto(path);
		await settled;
		await expect(page.getByText('Loading...')).toHaveCount(0);
		await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);
		await expectNoSidewaysScroll(page);
	}
});
