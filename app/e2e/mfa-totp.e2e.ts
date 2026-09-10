import { expect, test } from '@playwright/test';
import { enrollSecondFactor, verifyEmail } from './mfa';
import { seedFoundingOwner } from './staffSignup';
import { STUB_TOTP_CODE, stubTotpFactor } from './totpStub';

/*
 * #1132: the two TOTP screens #606 ships, driven in a browser for the
 * first time.
 *
 * The Firebase Auth emulator has no TOTP in any released version of
 * firebase-tools, so until this file every one of these screens was
 * covered only by unit specs with the Identity Platform SDK mocked out,
 * and every other e2e spec injected a session cookie rather than walking
 * /login. totpStub.ts closes that by relabeling the emulator's own
 * second factor at the browser's network boundary; its header records
 * what was ruled out first, what stays faked, and why.
 *
 * What each of these tests proves, and what it does not: the screen, the
 * Firebase JS SDK's TOTP code path, the session exchange and the Go
 * gate are all real, and the Owner landing on a Practice-scoped page is
 * the gate in api/internal/staffauth admitting a session that carries
 * `firebase.sign_in_second_factor` -- an Owner is barred from every one
 * of those routes without it, regardless of the Practice's own
 * mfa-required switch. What is *not* proven is that six particular
 * digits are the right six digits: no test in this repo can check a TOTP
 * code against a shared secret, because the emulator cannot hold one.
 */

test('An Owner sets up an authenticator app and lands on the Practice she was sent back to', async ({
	page,
	request
}) => {
	const { email, password, localId, practiceId } = await seedFoundingOwner(request, {
		practiceName: 'Riverside Doulas',
		staffName: 'Jamie Owner'
	});

	// Identity Platform refuses to enroll a second factor on an
	// unverified address, exactly as the live service does, and nothing
	// in a browser can click the link the emulator never sends.
	await verifyEmail(request, localId);
	await stubTotpFactor(page, request);

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();

	// An Owner with no second factor cannot reach her own Practice, so
	// the sign-in itself drives her here, carrying where she was headed.
	await expect(page).toHaveURL(new RegExp(String.raw`/mfa/enroll\?returnTo=`));

	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Continue' }).click();

	// #606's own AC: a person enrolling on the device she is reading the
	// screen on cannot scan her own screen, so the key is offered as text
	// beside the QR code rather than only inside it. That both are
	// rendered is the claim; the secret inside them is totpStub.ts's
	// fixture text, since the emulator has no enrollment session to
	// negotiate one with.
	await expect(
		page.getByRole('img', { name: 'QR code for setting up two-factor authentication in an authenticator app' })
	).toBeVisible();

	await page.getByLabel('Authenticator app code').fill(STUB_TOTP_CODE);
	await page.getByRole('button', { name: 'Confirm and turn on' }).click();

	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}`));
});

test('An enrolled Owner answers the code challenge at sign-in and is admitted', async ({ page, request }) => {
	const { email, password, idToken, localId, practiceId } = await seedFoundingOwner(request, {
		practiceName: 'Lakeside Doulas',
		staffName: 'Robin Owner'
	});

	await verifyEmail(request, localId);
	await enrollSecondFactor(request, idToken);
	await stubTotpFactor(page, request);

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();

	// The password is already proven, so the challenge step asks for one
	// thing and shows no email or password field to re-answer.
	const codeField = page.getByLabel('Authenticator app code');
	await expect(codeField).toBeVisible();
	await expect(page.getByLabel('Password')).toBeHidden();

	// A code that is not shaped like an authenticator app's output is a
	// sign-in failure, not an app error -- one sentence about the code,
	// and the password never asked for again. This is the *malformed*
	// case, which is as far as any test here can go: nothing in this repo
	// can tell a wrong six digits from a right six digits, because the
	// emulator holds no shared secret to check them against.
	await codeField.fill('123');
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect(
		page.getByText('The code is not correct. Enter the 6-digit code from your authenticator app.').first()
	).toBeVisible();
	await expect(page.getByLabel('Password')).toBeHidden();

	await codeField.fill(STUB_TOTP_CODE);
	await page.getByRole('button', { name: 'Continue' }).click();

	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}`));
});
