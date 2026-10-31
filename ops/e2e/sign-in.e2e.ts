import { expect, test, type APIRequestContext, type Page } from '../../app/e2e/playwrightTest';
import { EMULATOR_URL } from '../../app/e2e/mfa';
import { OPS_TENANT_ID } from '../../app/e2e/stack';
import { STUB_TOTP_CODE, stubTotpFactor } from '../../app/e2e/totpStub';

/*
 * The operator's sign-in walked in a browser against the local stack.
 * The emulator has no TOTP, so totpStub.ts relabels its phone factor at
 * the browser's network boundary; its header says what stays faked. The
 * screens, the Firebase SDK's TOTP path in the operator tenant, the
 * exchange and doula-ops's session are real.
 */

const password = 'operator-walk-password';

async function newOperatorLogin(request: APIRequestContext, name: string): Promise<string> {
	const email = `operator-${crypto.randomUUID()}@doula-cloud.invalid`;
	const created = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/projects/doula-cloud/tenants/${OPS_TENANT_ID}/accounts`,
		{
			headers: { Authorization: 'Bearer owner' },
			data: { email, password, displayName: name, emailVerified: true }
		}
	);
	expect(created.ok()).toBe(true);
	return email;
}

async function signInWithPassword(page: Page, email: string) {
	await page.goto('/sign-in');
	await page.getByLabel('Email address').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Sign in' }).click();
}

async function signOut(page: Page) {
	await page.getByRole('button', { name: 'Sign out' }).click();
	await expect(page.getByRole('heading', { name: 'You have signed out' })).toBeVisible();
	await page.goto('/');
	await expect(page).toHaveURL(/\/sign-in$/);
}

test('An operator with no authenticator app is sent to set one up, lands signed in, and signs out', async ({
	page,
	request
}) => {
	const email = await newOperatorLogin(request, 'Walk Operator');
	await stubTotpFactor(page, request);

	await signInWithPassword(page, email);

	await expect(page.getByRole('heading', { name: 'Set up an authenticator app' })).toBeVisible();
	await expect(page.getByRole('img', { name: 'QR code for setting up an authenticator app' })).toBeVisible();
	await page.getByLabel('Authenticator app code').fill(STUB_TOTP_CODE);
	await page.getByRole('button', { name: 'Confirm and sign in' }).click();

	await expect(page.getByText(`You are signed in as Walk Operator (${email}).`)).toBeVisible();
	await signOut(page);
});

test('An enrolled operator answers the code challenge, lands signed in, and signs out', async ({ page, request }) => {
	const email = await newOperatorLogin(request, 'Enrolled Operator');
	await stubTotpFactor(page, request);
	await signInWithPassword(page, email);
	await page.getByLabel('Authenticator app code').fill(STUB_TOTP_CODE);
	await page.getByRole('button', { name: 'Confirm and sign in' }).click();
	await signOut(page);

	await signInWithPassword(page, email);
	await expect(page.getByRole('heading', { name: 'Enter the code from your authenticator app' })).toBeVisible();
	await page.getByLabel('Authenticator app code').fill(STUB_TOTP_CODE);
	await page.getByRole('button', { name: 'Continue' }).click();

	await expect(page.getByText(`You are signed in as Enrolled Operator (${email}).`)).toBeVisible();
	await signOut(page);
});

test('A wrong password is refused on the sign-in screen', async ({ page, request }) => {
	const email = await newOperatorLogin(request, 'Refused Operator');
	await page.goto('/sign-in');
	await page.getByLabel('Email address').fill(email);
	await page.getByLabel('Password').fill('not-the-password');
	await page.getByRole('button', { name: 'Sign in' }).click();

	await expect(page.getByRole('alert')).toContainText('Enter a correct email address and password');
});

test('Signing in with nothing entered says what to enter', async ({ page }) => {
	await page.goto('/sign-in');
	await page.getByRole('button', { name: 'Sign in' }).click();

	const summary = page.getByRole('alert');
	await expect(summary).toContainText('Enter your email address');
	await expect(summary).toContainText('Enter your password');
});
