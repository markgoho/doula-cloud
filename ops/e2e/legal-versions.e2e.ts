import { expect, test } from '../../app/e2e/playwrightTest';
import {
	OPS_LEGAL_VERSION,
	countLegalNotices,
	publishLegalVersion,
	seedRefundablePractice,
	unpublishLegalVersion
} from '../../app/e2e/stack';
import { STUB_TOTP_CODE } from '../../app/e2e/totpStub';
import { signedInOperator } from './operator';

/*
 * A material version published through its check page against the local
 * stack, at 320px and wide. doula-ops's history holds OPS_LEGAL_VERSION
 * beyond the shipped one; doula-api's does not, so each walk takes the
 * publication and its emails back afterward. The wide walk also meets
 * the version published while its check page was showing.
 */

const password = 'operator-legal-password';
const { document, effective } = OPS_LEGAL_VERSION;

test.afterEach(() => unpublishLegalVersion(document, effective));

for (const width of [320, 1280]) {
	test(`An operator publishes a material version with a fresh code at ${width}px`, async ({ page, request }) => {
		await page.setViewportSize({ width, height: 800 });
		seedRefundablePractice(`Legal Walk Practice ${crypto.randomUUID().slice(0, 8)}`, 'Lena', 'Owner');
		const name = `Legal Operator ${crypto.randomUUID().slice(0, 8)}`;
		await signedInOperator(page, request, password, name);

		await page.getByRole('link', { name: 'Legal versions' }).click();
		await page.getByRole('link', { name: 'Check and publish the Privacy Policy that takes effect June 1, 2030' }).click();
		await expect(page.getByRole('heading', { name: 'Check the version before you publish it', level: 1 })).toBeVisible();
		await expect(page.getByText('Privacy Policy', { exact: true })).toBeVisible();
		await expect(page.getByText(OPS_LEGAL_VERSION.change)).toBeVisible();
		await expect(page.getByText('June 1, 2030', { exact: true })).toBeVisible();
		const owners = page.getByText(/^\d+ Owners?$/);
		await expect(owners).toBeVisible();
		const counted = Number((await owners.textContent())?.split(' ')[0]);
		expect(counted).toBeGreaterThan(0);

		await page.getByRole('button', { name: 'Publish' }).click();
		await expect(page.getByRole('alert')).toContainText('Enter your password');

		await page.getByLabel('Password').fill(password);
		await page.getByLabel('Authenticator app code').fill(STUB_TOTP_CODE);
		await page.getByRole('button', { name: 'Publish' }).click();

		await expect(page.getByRole('heading', { name: 'Version published' })).toBeVisible();
		await expect(page.getByRole('status')).toContainText('The Privacy Policy that takes effect June 1, 2030 is published.');
		await expect(page.getByRole('status')).toContainText(`${counted} ${counted === 1 ? 'Owner is' : 'Owners are'} emailed`);
		expect(countLegalNotices(document, effective)).toBe(counted);
		expect(await page.evaluate(() => globalThis.document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);

		await page.reload();
		await expect(page.getByRole('heading', { name: 'This version is published', level: 1 })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Publish' })).toHaveCount(0);

		await page.goto('/operator-log');
		const newest = page.getByRole('listitem').filter({ hasText: name }).first();
		await expect(newest.getByRole('heading', { name: 'Published a legal version' })).toBeVisible();
		await expect(newest).toContainText('The Privacy Policy that takes effect June 1, 2030, material');
	});
}

test('A version published while its check page shows is refused in words', async ({ page, request }) => {
	await signedInOperator(page, request, password, 'Late Legal Operator');
	await page.goto(`/legal-versions/${document}/${effective}`);
	await expect(page.getByRole('heading', { name: 'Check the version before you publish it', level: 1 })).toBeVisible();

	publishLegalVersion(document, effective, true, 'Other Legal Operator');

	await page.getByLabel('Password').fill(password);
	await page.getByLabel('Authenticator app code').fill(STUB_TOTP_CODE);
	await page.getByRole('button', { name: 'Publish' }).click();

	await expect(page.getByRole('alert')).toContainText('This version was already published on');
	await expect(page.getByRole('alert')).toContainText('nothing has changed');
});
