import { expect, test } from '../../app/e2e/playwrightTest';
import { signedInOperator } from './operator';

/*
 * An address let into the pilot, walked through its check page against
 * the local stack, at 320px and wide: the check page states the address
 * as it will be stored, Change returns to it, and a second add of the
 * same address is refused with the date it was added.
 */

for (const width of [320, 1280]) {
	test(`An operator adds an address to the pilot allowlist at ${width}px`, async ({ page, request }) => {
		await page.setViewportSize({ width, height: 800 });
		const name = `Pilot Operator ${crypto.randomUUID().slice(0, 8)}`;
		const local = `ana-${crypto.randomUUID().slice(0, 8)}`;
		const address = `${local}@example.com`;
		await signedInOperator(page, request, 'operator-pilot-password', name);

		await page.getByRole('link', { name: 'Pilot allowlist' }).click();
		await expect(page.getByRole('heading', { name: 'Add an address to the pilot allowlist', level: 1 })).toBeVisible();
		await page.getByRole('button', { name: 'Continue' }).click();
		await expect(page.getByRole('alert').filter({ hasText: 'There is a problem' })).toContainText('Enter an email address');

		await page.getByLabel('Email address').fill(`  ${local.toUpperCase()}@Example.com `);
		await page.getByRole('button', { name: 'Continue' }).click();

		await expect(page.getByRole('heading', { name: 'Check the address before you add it' })).toBeVisible();
		await expect(page.getByRole('definition').filter({ hasText: address })).toBeVisible();
		await page.getByRole('button', { name: 'Change', exact: true }).click();
		await expect(page.getByLabel('Email address')).toBeFocused();
		await page.getByRole('button', { name: 'Continue' }).click();

		await page.getByRole('button', { name: 'Add the address' }).click();
		await expect(page.getByRole('heading', { name: 'Address added' })).toBeVisible();
		await expect(page.getByRole('status')).toContainText(`${address} is on the pilot allowlist.`);
		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);

		await page.getByRole('link', { name: 'Add another address' }).click();
		await page.getByLabel('Email address').fill(address);
		await page.getByRole('button', { name: 'Continue' }).click();
		await page.getByRole('button', { name: 'Add the address' }).click();
		await expect(page.getByRole('alert').filter({ hasText: 'There is a problem' })).toContainText(`${address} is already on the pilot allowlist.`);

		await page.goto('/operator-log');
		const newest = page.getByRole('listitem').filter({ hasText: name }).first();
		await expect(newest.getByRole('heading', { name: 'Added to the pilot allowlist' })).toBeVisible();
		await expect(newest).toContainText(address);
	});
}
