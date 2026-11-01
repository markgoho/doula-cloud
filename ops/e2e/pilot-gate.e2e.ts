import { expect, test } from '../../app/e2e/playwrightTest';
import { closePilotGate, openPilotGate } from '../../app/e2e/stack';
import { STUB_TOTP_CODE } from '../../app/e2e/totpStub';
import { signedInOperator } from './operator';

/*
 * The pilot gate opened through its check page against the local stack,
 * at 320px and wide. The stack opens the gate as it starts, so each walk
 * closes it first and the suite leaves it open. The wide walk also meets
 * a gate opened while its check page was showing, and is refused.
 */

const password = 'operator-pilot-gate-password';

test.afterEach(() => openPilotGate());

for (const width of [320, 1280]) {
	test(`An operator opens the pilot gate with a fresh code at ${width}px`, async ({ page, request }) => {
		await page.setViewportSize({ width, height: 800 });
		closePilotGate();
		const name = `Gate Operator ${crypto.randomUUID().slice(0, 8)}`;
		await signedInOperator(page, request, password, name);

		await page.getByRole('link', { name: 'Pilot gate' }).click();
		await expect(page.getByRole('heading', { name: 'Check before you open the pilot gate', level: 1 })).toBeVisible();
		await expect(page.getByText('Anyone who signs up can create a Practice', { exact: false })).toBeVisible();
		await expect(page.getByText('The gate stays open for good.', { exact: false })).toBeVisible();

		await page.getByRole('button', { name: 'Open the pilot gate' }).click();
		await expect(page.getByRole('alert')).toContainText('Enter your password');

		await page.getByLabel('Password').fill(password);
		await page.getByLabel('Authenticator app code').fill(STUB_TOTP_CODE);
		await page.getByRole('button', { name: 'Open the pilot gate' }).click();

		await expect(page.getByRole('heading', { name: 'Pilot gate open' })).toBeVisible();
		await expect(page.getByRole('status')).toContainText('Anyone who signs up can now create a Practice.');
		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);

		await page.reload();
		await expect(page.getByRole('heading', { name: 'The pilot gate is open', level: 1 })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Open the pilot gate' })).toHaveCount(0);

		await page.goto('/operator-log');
		const newest = page.getByRole('listitem').filter({ hasText: name }).first();
		await expect(newest.getByRole('heading', { name: 'Opened the pilot gate' })).toBeVisible();
	});
}

test('A gate opened while its check page shows is refused in words', async ({ page, request }) => {
	closePilotGate();
	await signedInOperator(page, request, password, 'Late Gate Operator');
	await page.goto('/pilot-gate');
	await expect(page.getByRole('heading', { name: 'Check before you open the pilot gate', level: 1 })).toBeVisible();

	openPilotGate();
	await page.getByLabel('Password').fill(password);
	await page.getByLabel('Authenticator app code').fill(STUB_TOTP_CODE);
	await page.getByRole('button', { name: 'Open the pilot gate' }).click();

	await expect(page.getByRole('alert')).toContainText('The pilot gate is already open.');
	await expect(page.getByRole('alert')).toContainText('nothing has changed');
});
