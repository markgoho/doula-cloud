import { expect, test, type FrameLocator, type Page } from '@playwright/test';
import { openMagicLink, seedPortalClient } from './portalClient';

// #1558: every Portal screen carries the link to the Privacy Policy, in
// the same footer, and the link is complete and usable at every width the
// Portal is read at -- a desktop window, the 320px conformance floor
// (ADR-0024), and a narrow column of a Practice's own website, which is
// the one place the Portal is read inside someone else's page.
//
// The walk signs a Client in through the real magic link, so the first
// sign-in screen is walked too, before its Continue is pressed.

const PRIVACY_URL = 'https://doula.cloud/privacy';
const LINK_NAME = 'Privacy Policy (opens in new tab)';

type Surface = Page | FrameLocator;

async function expectPrivacyLink(surface: Surface, where: string): Promise<void> {
	const link = surface.getByRole('contentinfo').getByRole('link', { name: LINK_NAME });
	await expect(link, `${where}: no Privacy Policy link`).toBeVisible();
	await expect(link).toHaveAttribute('href', PRIVACY_URL);
	await expect(link).toHaveAttribute('target', '_blank');
	// Its own text says so, not only its accessible name.
	await expect(link).toHaveText(LINK_NAME);
	// ADR-0053: a notice. Nothing in the footer asks her to agree.
	await expect(surface.getByRole('contentinfo').getByRole('checkbox')).toHaveCount(0);
	await expect(surface.getByRole('contentinfo').getByRole('button')).toHaveCount(0);
}

async function expectNoSidewaysScroll(page: Page, where: string): Promise<void> {
	const { scrollWidth, clientWidth } = await page.evaluate(() => ({
		scrollWidth: document.documentElement.scrollWidth,
		clientWidth: document.documentElement.clientWidth
	}));
	expect(scrollWidth, `${where} scrolls sideways`).toBe(clientWidth);
}

function portalScreens(engagementId: string): string[] {
	const hub = `/portal/engagements/${engagementId}`;
	return [
		hub,
		`${hub}/messages`,
		`${hub}/birth-plan`,
		`${hub}/contract`,
		`${hub}/notifications`,
		`${hub}/invoices`,
		`${hub}/sign-in-address`
	];
}

// The screens a Client reaches with no session. Without a token, the
// invitation and confirmation screens draw their own refusal, which is
// still a Portal screen she can land on.
const SIGNED_OUT_SCREENS = ['/portal/login', '/portal/accept-invite', '/portal/confirm-sign-in-address'];

async function walk(page: Page, request: Parameters<typeof seedPortalClient>[0], width: number): Promise<void> {
	for (const path of SIGNED_OUT_SCREENS) {
		await page.goto(path);
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
		await expectPrivacyLink(page, `${path} at ${width}px`);
		await expectNoSidewaysScroll(page, `${path} at ${width}px`);
	}

	const { engagementId, clientEmail } = await seedPortalClient(request, `Riverside Doulas ${width}`);
	await openMagicLink(page, request, clientEmail);
	await expectPrivacyLink(page, `the first sign-in screen at ${width}px`);
	await expectNoSidewaysScroll(page, `the first sign-in screen at ${width}px`);
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	for (const path of portalScreens(engagementId)) {
		await page.goto(path);
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
		await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);
		await expectPrivacyLink(page, `${path} at ${width}px`);
		await expectNoSidewaysScroll(page, `${path} at ${width}px`);
	}
}

test.describe('at 1280px', () => {
	test.use({ viewport: { width: 1280, height: 800 } });

	test('every Portal screen carries the Privacy Policy link in its footer', async ({ page, request }) => {
		await walk(page, request, 1280);
	});
});

test.describe('at 320px', () => {
	test.use({ viewport: { width: 320, height: 640 } });

	test('every Portal screen carries the Privacy Policy link in its footer', async ({ page, request }) => {
		await walk(page, request, 320);
	});
});

// The column is the 320px floor itself (ADR-0024): narrower than that is
// not a width the Portal commits to.
const COLUMN_WIDTH = 320;

/*
 * A Practice's own website, with the Portal in a narrow sidebar column
 * beside the site's own content. The host page is drawn on the app's own
 * origin so the frame keeps the Client's session.
 */
async function expectFitsInColumn(page: Page, path: string): Promise<void> {
	await page.evaluate(
		({ source, width }) => {
			document.body.innerHTML = `
				<div style="display:flex;gap:24px;padding:16px">
					<article style="flex:1"><h2>About our practice</h2></article>
					<aside style="inline-size:${width}px;flex:none">
						<iframe title="Client portal" src="${source}" style="inline-size:100%;block-size:760px;border:0"></iframe>
					</aside>
				</div>`;
		},
		{ source: path, width: COLUMN_WIDTH }
	);
	const frame = page.frameLocator('iframe[title="Client portal"]');
	const where = `${path} in a ${COLUMN_WIDTH}px column`;
	await expect(frame.getByRole('heading', { level: 1 })).toBeVisible();
	await expect(frame.locator('[aria-busy="true"]')).toHaveCount(0);
	await expectPrivacyLink(frame, where);

	const fits = await frame.locator('html').evaluate((root) => {
		const link = root.querySelector(':scope footer a');
		return {
			scrollWidth: root.scrollWidth,
			clientWidth: root.clientWidth,
			right: link?.getBoundingClientRect().right ?? Infinity
		};
	});
	expect(fits.scrollWidth, `${where} scrolls sideways`).toBe(fits.clientWidth);
	expect(fits.right, `${where}: the link runs past the column`).toBeLessThanOrEqual(fits.clientWidth);
}

test('the link is complete and usable in a narrow column of a Practice website', async ({ page, request }) => {
	test.setTimeout(120_000);
	await page.setViewportSize({ width: 1280, height: 800 });

	await page.goto('/portal/login');
	for (const path of SIGNED_OUT_SCREENS) {
		await expectFitsInColumn(page, path);
	}

	const { engagementId, clientEmail } = await seedPortalClient(request, 'Riverside Doulas embedded');
	await openMagicLink(page, request, clientEmail);
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	for (const path of portalScreens(engagementId)) {
		await expectFitsInColumn(page, path);
	}
});

