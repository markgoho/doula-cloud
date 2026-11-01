import { expect, test, type APIRequestContext } from '../../app/e2e/playwrightTest';
import { EMULATOR_URL, randomPhoneNumber } from '../../app/e2e/mfa';
import { seedStaffAtPractices } from '../../app/e2e/stack';
import { STUB_TOTP_CODE } from '../../app/e2e/totpStub';
import { signedInOperator } from './operator';

/*
 * A two-factor reset walked through its check page against the local
 * stack. The Staff member's account is in the emulator's Staff pool with
 * a second factor enrolled, and doula-ops asks the Admin SDK on that pool
 * to clear it. The emulator refuses the body that clears a second factor
 * (app/e2e/mfa.ts says why), so the walk ends on the refusal and finds
 * nothing changed; the handler specs in api/cmd/ops hold the reset itself.
 */

const password = 'operator-two-factor-password';
const EMULATOR_ADMIN = { Authorization: 'Bearer owner' };

async function staffAccountWithSecondFactor(request: APIRequestContext): Promise<string> {
	const created = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/projects/doula-cloud/accounts`, {
		headers: EMULATOR_ADMIN,
		data: {
			email: `staff-${crypto.randomUUID()}@doula-cloud.invalid`,
			password: 'staff-two-factor-password',
			emailVerified: true,
			mfaInfo: [{ phoneInfo: randomPhoneNumber(), displayName: 'Phone' }]
		}
	});
	expect(created.ok(), await created.text()).toBe(true);
	const { localId } = await created.json();
	return localId;
}

async function secondFactorCount(request: APIRequestContext, localId: string): Promise<number> {
	const lookup = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/projects/doula-cloud/accounts:lookup`, {
		headers: EMULATOR_ADMIN,
		data: { localId: [localId] }
	});
	expect(lookup.ok()).toBe(true);
	const { users } = await lookup.json();
	return users[0].mfaInfo?.length ?? 0;
}

test('An operator checks and sends a two-factor reset with a fresh code', async ({ page, request }) => {
	const localId = await staffAccountWithSecondFactor(request);
	expect(await secondFactorCount(request, localId)).toBe(1);
	const staffId = seedStaffAtPractices(localId, 'Priya', 'Raman', ['Lakeside Birth Collective', 'Rooted Doula Care']);
	await signedInOperator(page, request, password, 'Two-Factor Operator');

	await page.getByRole('link', { name: 'Reset two-factor authentication' }).click();
	await page.getByRole('button', { name: 'Find' }).click();
	await expect(page.getByRole('alert').filter({ hasText: 'There is a problem' })).toContainText(
		'Enter the email address the person signs in with'
	);
	await page.getByLabel('Sign-in email address').fill(`STAFF-${staffId}@doula-cloud.invalid`);
	await page.getByRole('button', { name: 'Find' }).click();
	await expect(page.getByRole('status')).toContainText('Lakeside Birth Collective, Rooted Doula Care');
	await page.getByRole('link', { name: 'Priya Raman' }).click();
	await expect(page).toHaveURL(new RegExp(`/staff/${staffId}/two-factor$`));
	await expect(page.getByRole('heading', { name: 'Check before you reset two-factor authentication' })).toBeVisible();
	const answers = page.getByRole('definition');
	await expect(answers.filter({ hasText: 'Priya Raman' })).toBeVisible();
	await expect(answers.filter({ hasText: 'Lakeside Birth Collective, Rooted Doula Care' })).toBeVisible();
	await expect(page.getByText('You can see this person now, on a live video call.')).toBeVisible();
	await expect(page.getByText(/government photo ID shown on that call matches/)).toBeVisible();

	await page.getByRole('button', { name: 'Reset two-factor authentication' }).click();
	await expect(page.getByRole('alert').filter({ hasText: 'There is a problem' })).toContainText('Enter your password');
	expect(await secondFactorCount(request, localId)).toBe(1);

	await page.getByLabel('Password').fill(password);
	await page.getByLabel('Authenticator app code').fill(STUB_TOTP_CODE);
	await page.getByRole('button', { name: 'Reset two-factor authentication' }).click();

	await expect(page.getByRole('alert').filter({ hasText: 'There is a problem' })).toContainText(
		'Identity Platform did not reset two-factor authentication, so nothing has changed. Try again.'
	);
	await expect(page.getByRole('heading', { name: 'Check before you reset two-factor authentication' })).toBeVisible();
	expect(await secondFactorCount(request, localId)).toBe(1);
});
