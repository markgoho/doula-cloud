import { expect, test, type APIRequestContext, type Page } from '../../app/e2e/playwrightTest';
import { seedRefundablePractice } from '../../app/e2e/stack';
import { STUB_TOTP_CODE } from '../../app/e2e/totpStub';
import { signedInOperator } from './operator';

/*
 * A refund walked through its check page against the local stack. The
 * emulator has no TOTP, so totpStub.ts relabels its phone factor at the
 * browser's network boundary, for the sign-in and for the step-up alike.
 * The stack has no Stripe, so doula-ops refunds through its in-memory
 * fake; the ledger, the act and the Activity row are real.
 */

const password = 'operator-refund-password';

function signedIn(page: Page, request: APIRequestContext): Promise<string> {
	return signedInOperator(page, request, password, 'Refund Operator');
}

test('An operator refunds two Credits on the check page with a fresh code', async ({ page, request }) => {
	const practiceId = seedRefundablePractice('Lakeside Birth Collective', 'Renata', 'Alvarez');
	await signedIn(page, request);

	await page.goto(`/practices/${practiceId}/refund`);
	await expect(page.getByRole('heading', { name: 'Refund Credits' })).toBeVisible();
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect(page.getByRole('alert').filter({ hasText: 'There is a problem' })).toContainText('Choose the Owner who asked for the refund');
	await expect(page.getByRole('alert').filter({ hasText: 'There is a problem' })).toContainText('Enter the date the request arrived');

	await page.getByLabel('Number of Credits').fill('2');
	await page.getByLabel('Renata Alvarez').check();
	await page.getByLabel('Month').fill('10');
	await page.getByLabel('Day').fill('1');
	await page.getByLabel('Year').fill('2026');
	await page.getByRole('button', { name: 'Continue' }).click();

	await expect(page.getByRole('heading', { name: 'Check the refund before you make it' })).toBeVisible();
	const answers = page.getByRole('definition');
	await expect(answers.filter({ hasText: 'Lakeside Birth Collective' })).toBeVisible();
	await expect(answers.filter({ hasText: '$42.74, including $2.74 sales tax, to the card it was paid with' })).toBeVisible();
	await expect(answers.filter({ hasText: 'Renata Alvarez' })).toBeVisible();
	await expect(answers.filter({ hasText: 'October 1, 2026' })).toBeVisible();

	await page.getByRole('button', { name: 'Refund $42.74' }).click();
	await expect(page.getByRole('alert').filter({ hasText: 'There is a problem' })).toContainText('Enter your password');

	await page.getByLabel('Password').fill(password);
	await page.getByLabel('Authenticator app code').fill(STUB_TOTP_CODE);
	await page.getByRole('button', { name: 'Refund $42.74' }).click();

	await expect(page.getByRole('heading', { name: 'Refund made' })).toBeVisible();
	await expect(page.getByRole('status')).toContainText(
		'2 Credits refunded to Lakeside Birth Collective. $42.74 goes back to the card it was paid with, including $2.74 sales tax.'
	);
	await expect(page.getByRole('status')).toContainText('Asked for by Renata Alvarez on October 1, 2026.');
});

test('More Credits than the purchase holds are refused before the check page', async ({ page, request }) => {
	await signedIn(page, request);
	const practiceId = seedRefundablePractice('Spent Practice', 'Maya', 'Okonkwo');

	await page.goto(`/practices/${practiceId}/refund`);
	await page.getByLabel('Number of Credits').fill('6');
	await page.getByLabel('Maya Okonkwo').check();
	await page.getByLabel('Month').fill('10');
	await page.getByLabel('Day').fill('1');
	await page.getByLabel('Year').fill('2026');
	await page.getByRole('button', { name: 'Continue' }).click();

	await expect(page.getByRole('alert').filter({ hasText: 'There is a problem' })).toContainText('Number of Credits must be 5 or fewer');
});
