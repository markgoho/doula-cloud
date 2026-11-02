import { expect, test } from '../../app/e2e/playwrightTest';
import { EMULATOR_URL } from '../../app/e2e/mfa';
import { OPS_TENANT_ID, SEEDED_OPERATOR } from '../../app/e2e/stack';

/*
 * #2556: what dev:full promises a developer. The seeded operator signs in
 * with no totpStub.ts: the page sends the emulator's phone factor a code,
 * the developer reads it from the emulator's list, and types it.
 */
test('The seeded operator signs in by hand with the code the emulator lists', async ({ page, request }) => {
	await page.goto('/sign-in');
	await page.getByLabel('Email address').fill(SEEDED_OPERATOR.email);
	await page.getByLabel('Password').fill(SEEDED_OPERATOR.password);
	await page.getByRole('button', { name: 'Sign in' }).click();

	await expect(page.getByRole('heading', { name: 'Enter the code from the Auth emulator' })).toBeVisible();
	const list = page.getByRole('link', { name: 'Open its list of codes' });
	await expect(list).toHaveAttribute(
		'href',
		`${EMULATOR_URL}/emulator/v1/projects/doula-cloud/tenants/${OPS_TENANT_ID}/verificationCodes`
	);

	const listing = await request.get((await list.getAttribute('href'))!);
	const { verificationCodes } = await listing.json();
	const newest = verificationCodes.findLast(
		(entry: { phoneNumber: string }) => entry.phoneNumber === SEEDED_OPERATOR.phoneNumber
	);
	await page.getByLabel('Emulator code').fill(newest.code);
	await page.getByRole('button', { name: 'Continue' }).click();

	await expect(page.getByText(`You are signed in as ${SEEDED_OPERATOR.name}`)).toBeVisible();
});
