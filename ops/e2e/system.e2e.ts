import { expect, test } from '../../app/e2e/playwrightTest';
import { seedWebhookArrivals } from '../../app/e2e/stack';
import { signedInOperator } from './operator';

/*
 * The System section against the local stack at 320px and 1440px: what
 * the service is doing, with the page never wider than the screen.
 */

for (const width of [320, 1440]) {
	test(`An operator reads the System section at ${width}px`, async ({ page, request }, testInfo) => {
		await page.setViewportSize({ width, height: 900 });
		seedWebhookArrivals();
		await signedInOperator(page, request, 'operator-system-password', 'System Operator');

		await page.getByRole('link', { name: 'System' }).click();

		await expect(page.getByRole('heading', { name: 'System', level: 1 })).toBeVisible();
		for (const heading of ['Stripe', 'Webhooks', 'Stripe event types', 'Outboxes', 'Pilot gate', 'Pilot allowlist']) {
			await expect(page.getByRole('heading', { name: heading, level: 2, exact: true })).toBeVisible();
		}
		await expect(page.getByText('Platform account')).toBeVisible();

		const webhooks = page.getByRole('table', { name: /last arrival and its refusals/ });
		await expect(webhooks.getByRole('row', { name: /^Stripe Connect .* (EST|EDT)/ }).first()).toBeVisible();
		await expect(webhooks.getByRole('row', { name: /^Stripe platform .* \(\d+ signature\)/ })).toBeVisible();
		const events = page.getByRole('table', { name: /each event type DoulaCloud handles/ });
		await expect(events.getByRole('row', { name: /^invoice\.paid Stripe Connect .* (EST|EDT)$/ })).toBeVisible();
		await expect(events.getByRole('row', { name: /^refund\.created Stripe Connect Not seen yet$/ })).toBeVisible();
		await expect(page.getByRole('table', { name: /Rows waiting, sent, skipped/ }).getByRole('row', { name: /^Portal invite / })).toBeVisible();

		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
		await page.screenshot({ path: testInfo.outputPath(`system-${width}.png`), fullPage: true });
	});
}
