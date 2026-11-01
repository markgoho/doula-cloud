import { expect, type APIRequestContext, type Page } from '../../app/e2e/playwrightTest';
import { EMULATOR_URL } from '../../app/e2e/mfa';
import { OPS_TENANT_ID } from '../../app/e2e/stack';
import { STUB_TOTP_CODE, stubTotpFactor } from '../../app/e2e/totpStub';

/*
 * An operator login made in the emulator's operator tenant and signed in
 * through the sign-in page. The emulator has no TOTP, so totpStub.ts
 * relabels its phone factor at the browser's network boundary.
 */
export async function signedInOperator(
	page: Page,
	request: APIRequestContext,
	password: string,
	displayName: string
): Promise<string> {
	const email = `operator-${crypto.randomUUID()}@doula-cloud.invalid`;
	const created = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/projects/doula-cloud/tenants/${OPS_TENANT_ID}/accounts`,
		{
			headers: { Authorization: 'Bearer owner' },
			data: { email, password, displayName, emailVerified: true }
		}
	);
	expect(created.ok()).toBe(true);
	await stubTotpFactor(page, request);
	await page.goto('/sign-in');
	await page.getByLabel('Email address').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Sign in' }).click();
	await page.getByLabel('Authenticator app code').fill(STUB_TOTP_CODE);
	await page.getByRole('button', { name: 'Confirm and sign in' }).click();
	await expect(page.getByText(`You are signed in as ${displayName}`)).toBeVisible();
	return email;
}
