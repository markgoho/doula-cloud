import AxeBuilder from '@axe-core/playwright';
import { expect, test, type BrowserContext, type Page } from '@playwright/test';
import { seedFounderSession } from './founder';
import { signInEnrolled } from './mfa';
import { API_URL, PREVIEW_SERVER_ORIGIN } from './ports';
import { seedFoundingOwner } from './staffSignup';
import { WCAG_TAGS } from './wcag';

/*
 * The founder read page, end to end (#1526): a Staff member's piece of
 * Feedback, the list that holds it, and the page that shows every stored
 * field -- walked in the real shell, at the width ADR-0024 commits to.
 * The continuum sweep measures each route inside a frame, and the shell
 * (the top bar, the Pilot banner) is not in the frame.
 */
test.use({ viewport: { width: 320, height: 640 } });

const FEEDBACK_TEXT = 'The invoice total on this page does not match the contract.';

async function becomeSession(context: BrowserContext, headers: { Cookie: string }): Promise<void> {
	await context.clearCookies();
	await context.addCookies([
		{
			name: '__session',
			value: headers.Cookie.replace('__session=', ''),
			url: PREVIEW_SERVER_ORIGIN,
			httpOnly: true,
			secure: false,
			sameSite: 'Lax'
		}
	]);
}

async function expectNoSidewaysScroll(page: Page): Promise<void> {
	const { scrollWidth, clientWidth } = await page.evaluate(() => ({
		scrollWidth: document.documentElement.scrollWidth,
		clientWidth: document.documentElement.clientWidth
	}));
	expect(scrollWidth, `${page.url()} scrolls sideways at 320px`).toBe(clientWidth);
}

/*
 * The accessibility gate's own bar (wcag.ts), applied here rather than in
 * accessibility.e2e.ts: this file is the only one that may hold the
 * founder. Called once each screen's own content is on the page, so axe
 * never scans a half-loaded one.
 */
async function expectNoAccessibilityViolations(page: Page, screen: string): Promise<void> {
	const { violations } = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze();
	expect(
		violations.map((v) => `${v.id} (${v.impact}) on ${v.nodes.length}: ${v.nodes[0]?.target.join(' ')} -- ${v.help}`),
		`accessibility violations on ${screen}`
	).toEqual([]);
}

test('the founder reads a piece of Feedback, and nobody else finds the page', async ({ page, request, context }) => {
	// An Owner sends a piece from under her Practice, through the real route.
	const owner = await seedFoundingOwner(request, { practiceName: 'Riverside Doulas', staffName: 'Jamie Owner' });
	const ownerHeaders = await signInEnrolled(request, owner.idToken, owner.localId);
	const sent = await request.post(`${API_URL}/api/staff/feedback`, {
		headers: ownerHeaders,
		data: {
			kind: 'not_working',
			text: FEEDBACK_TEXT,
			page: {
				url: `/practices/${owner.practiceId}/invoices`,
				route: { id: '/practices/[practiceId]/invoices' }
			},
			appBuild: 'e2e-build',
			screenWidth: 320,
			practiceId: owner.practiceId
		}
	});
	expect(sent.status(), await sent.text()).toBe(201);
	const { id: feedbackId } = await sent.json();

	// She is an Owner with a second factor, and still not the founder:
	// both pages are the "not found" page a URL with no route behind it gets.
	await becomeSession(context, ownerHeaders);
	for (const path of ['/feedback', `/feedback/${feedbackId}`]) {
		await page.goto(path);
		await expect(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
	}

	// The founder finds it in the list.
	await becomeSession(context, await seedFounderSession(request));
	await page.goto('/feedback');
	await expect(page.getByRole('heading', { name: 'Feedback', level: 1 })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Issue not opened' })).toBeVisible();
	await expectNoSidewaysScroll(page);
	await expectNoAccessibilityViolations(page, '/feedback');

	/*
	 * A row's link is named by when the piece was sent, and other specs
	 * send pieces into this same stack, so the name does not pick one row
	 * out. The href does. DataTable keeps a table and a record view in the
	 * document and shows one, hence the visible filter.
	 */
	await page.locator(`a[href="/feedback/${feedbackId}"]`).filter({ visible: true }).first().click();

	// Every stored field, with the sender and the Practice named.
	await expect(page).toHaveURL(new RegExp(`/feedback/${feedbackId}$`));
	await expect(page.getByRole('heading', { name: /^Feedback sent/, level: 1 })).toBeVisible();
	await expect(page.getByText(FEEDBACK_TEXT)).toBeVisible();
	for (const value of [
		'Something is not working',
		'Jamie Owner',
		owner.email,
		'owner',
		'Riverside Doulas',
		`/practices/${owner.practiceId}/invoices`,
		'/practices/[practiceId]/invoices',
		'e2e-build',
		'320px'
	]) {
		await expect(page.getByRole('definition').filter({ hasText: value }).first()).toBeVisible();
	}
	await expectNoSidewaysScroll(page);
	await expectNoAccessibilityViolations(page, '/feedback/[feedbackId]');

	await page.getByRole('link', { name: 'Back to Feedback' }).click();
	await expect(page).toHaveURL(/\/feedback$/);
});
