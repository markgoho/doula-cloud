import { expect, test, type APIRequestContext } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT } from './ports';
import { MAILBOX_URL, WORKER_SECRET, readStaffInviteToken } from './stack';
import { signIn } from './auth';
import { enrollSecondFactor, enterPracticeAsEnrolled, verifyEmail } from './mfa';
import { seedFoundingOwner } from './staffSignup';

const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

/*
 * #615's recovery path, walked as the two people who actually walk it
 * (#694): an Owner vouching for a doula whose phone is gone, and that
 * doula spending the code the Owner reads out to her.
 *
 * ## The two steps this spec cannot walk in the browser, and why
 *
 * Both are emulator gaps, named here rather than quietly worked around.
 *
 * **The step-up re-authentication is TOTP.** The Firebase Auth emulator
 * implements MFA for PHONE_SMS only -- e2e/mfa.ts's own header comment
 * records this at length -- so an Owner signing in through any of this
 * product's screens against the emulator is challenged for a factor the
 * product's TOTP-only UI cannot resolve. That is why every spec in this
 * suite injects the Owner's session cookie rather than walking /login,
 * and it is why the vouch POST here is sent with a freshly minted
 * emulator token instead of by pressing "Send the code" -- with the same
 * two credentials that button sends, so the request shape is still what
 * is under test.
 * [#1132](https://github.com/markgoho/doula-cloud/issues/1132) is the
 * ticket for closing this one.
 *
 * **A successful spend cannot complete.** See the second block comment
 * below, and [#1128](https://github.com/markgoho/doula-cloud/issues/1128).
 *
 * What is walked in the browser is everything the screens themselves own:
 * the Owner reaching the vouch screen from the roster and being told
 * where the code lands before she asks for it, the mail arriving at her
 * address and at nobody else's, and the spend screen -- reachable signed
 * out, from the log-in screen, answering a wrong code and an unknown
 * address with one sentence.
 */
test('An Owner vouches for a locked-out doula, and the code reaches her and nobody else', async ({
	page,
	request,
	context
}) => {
	const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	const doulaEmail = `doula-${unique}@example.com`;
	const doulaName = 'Robin Doula';
	const password = 'password123';

	// Fixture setup, not the seam under test (#207).
	const {
		email: ownerEmail,
		idToken: ownerIdToken,
		localId: ownerUID,
		practiceId
	} = await seedFoundingOwner(request, {
		practiceName: 'Riverside Doulas',
		staffName: 'Jamie Owner'
	});

	await verifyEmail(request, ownerUID);
	const ownerHeaders = await signIn(request, API_URL, await enrollSecondFactor(request, ownerIdToken));
	await enterPracticeAsEnrolled(context, page, ownerHeaders, practiceId);

	// A second Staff member to vouch *for*: the roster action does not
	// exist without one, and an Owner vouching for herself is not the act
	// this screen is about.
	const doulaStaffId = await inviteAndAcceptDoula();

	// The Owner's way in is the roster, so that is where this starts.
	await enterPracticeAsEnrolled(context, page, ownerHeaders, practiceId);
	await page.goto(`/practices/${practiceId}/staff`);
	// By href rather than by accessible name: every member row carries a
	// link reading "Send a recovery code", and which row it is comes from
	// the visually-hidden name joined by aria-describedby (#515) -- which
	// no Playwright role query can select on.
	await page.locator(`a[href$="/staff/${doulaStaffId}/mfa-recovery"]`).first().click();

	await expect(page.getByRole('heading', { name: `Help ${doulaName} sign in again` })).toBeVisible();
	// #615's AC, and the misreading the screen exists to prevent: the code
	// goes to the Owner, and she has to know that before she asks for it.
	await expect(page.getByText(`It does not go to ${doulaName}`)).toBeVisible();
	// The address in the sentence, not the one in the shell's avatar menu,
	// which is present on every Staff screen and hidden until it is opened.
	await expect(page.getByText(`emails it to you, at ${ownerEmail}`)).toBeVisible();

	await page.getByRole('button', { name: 'Send a recovery code' }).click();
	await expect(page.getByText(`The code comes to you, at ${ownerEmail}.`)).toBeVisible();
	// The step-up itself is a real screen, which is the AC. Pressing
	// through it is what the emulator cannot do -- see the header comment.
	await expect(page.getByLabel('Password')).toBeVisible();

	// The same request that button sends, with the same two credentials
	// the endpoint refuses without: a step-up token minted seconds ago,
	// and the confirmation header.
	const stepUpToken = await enrollSecondFactor(request, ownerIdToken);
	const vouch = await request.post(
		`${API_URL}/api/practices/${practiceId}/staff/${doulaStaffId}/mfa-recovery/vouch`,
		{ headers: { ...ownerHeaders, Authorization: `Bearer ${stepUpToken}`, 'X-Confirmed': 'true' } }
	);
	expect(vouch.ok(), `vouch failed: ${vouch.status()} ${await vouch.text()}`).toBe(true);

	// Nothing fires by itself locally (#762): deployed this is reached by
	// ADR-0013's nudge and by process-outbox-drain (#481).
	const drained = await request.post(
		`${API_URL}/api/internal/notifications/process-mfa-recovery-outbox`,
		{ headers: { 'X-Internal-Secret': WORKER_SECRET } }
	);
	expect(drained.ok(), 'draining the MFA-recovery outbox failed').toBe(true);

	// It arrives at the Owner's address, never the doula's -- asserted
	// both ways, because "went to the right person" and "did not go to the
	// wrong one" are two different claims.
	const doulaInbox = await request.get(`${MAILBOX_URL}/api/messages?to=${encodeURIComponent(doulaEmail)}`);
	expect(await doulaInbox.json(), 'the recovery code was mailed to the locked-out doula').toEqual([]);

	const ownerInbox = await request.get(`${MAILBOX_URL}/api/messages?to=${encodeURIComponent(ownerEmail)}`);
	const [message] = await ownerInbox.json();
	expect(message, `no recovery mail reached ${ownerEmail}`).toBeTruthy();
	expect(message.subject).toBe('Doula Cloud: account recovery code');
	const code = /Recovery code: (\S+)/.exec(message.text)?.[1];
	expect(code, `no code in the recovery mail:\n${message.text}`).toBeTruthy();

	/*
	 * The doula's half, and the reason it starts by throwing the session
	 * away: she is locked out. She holds no session, and spending a code
	 * mints her none either -- Identity Platform challenges the second
	 * factor on every sign-in while one exists, so the factor has to go
	 * before a sign-in can succeed.
	 *
	 * A wrong code rather than the one that just arrived, and that is the
	 * *second* emulator gap this spec has to route around rather than a
	 * shortcut. A successful spend ends in
	 * authn.FirebaseVerifier.ClearSecondFactors, whose Admin SDK call
	 * always marshals `mfa.enrollments` as JSON null (the SDK's own
	 * validateAndFormatMfaSettings leaves the slice nil however it is
	 * called); the emulator's schema refuses null for a repeated field
	 * with "must be array". Production Identity Platform accepts it:
	 * [#1128](https://github.com/markgoho/doula-cloud/issues/1128) ran
	 * that exact body against the real doula-cloud project on 2026-09-10
	 * and got a 200 with the enrollment actually gone on the read-back,
	 * so the null is emulator strictness over proto3-JSON and the shipped
	 * call is right as written. The success path therefore stays
	 * unwalkable here -- the fix would be to send an array purely to
	 * satisfy the emulator, which would leave the emulator, not
	 * production, deciding the shape of a live call. See the second named
	 * limitation in mfa.ts. What *is* walked is everything this screen
	 * owns: reachable signed out, from the log-in screen, posting the
	 * shape the endpoint reads, and rendering the one sentence #168
	 * allows.
	 */
	await context.clearCookies();
	await page.goto('/login');
	await page.getByRole('link', { name: 'Use a recovery code' }).click();

	await expect(page.getByRole('heading', { name: 'Use a recovery code' })).toBeVisible();
	await page.getByLabel('Email').fill(doulaEmail);
	await page.getByLabel('Recovery code').fill('00000000');
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect(page.getByText('this code is invalid or has expired').first()).toBeVisible();

	// The same sentence for an address the service has never heard of --
	// #168's whole point, and the one thing a screen can get wrong by
	// being helpful.
	await page.getByLabel('Email').fill(`nobody-${unique}@example.com`);
	await page.getByLabel('Recovery code').fill(code!);
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect(page.getByText('this code is invalid or has expired').first()).toBeVisible();

	/**
	 * Invites a doula through the real invite screen and accepts through
	 * the real accept screen, returning her staff id off the roster. The
	 * fixture rather than the seam (#207), but walked rather than seeded
	 * because there is no seeding helper for a second Staff member and the
	 * two screens are already covered elsewhere.
	 */
	async function inviteAndAcceptDoula(): Promise<string> {
		await page.goto(`/practices/${practiceId}/invite`);
		await page.getByLabel('Their email').fill(doulaEmail);
		const [inviteResponse] = await Promise.all([
			page.waitForResponse(
				(response) =>
					response.url().endsWith('/staff/invitations') && response.request().method() === 'POST'
			),
			page.getByRole('button', { name: 'Send invite' }).click()
		]);
		expect(inviteResponse.ok(), `invite send failed: ${inviteResponse.status()}`).toBe(true);
		const { invitationId } = await inviteResponse.json();

		// readStaffInviteToken throws, naming both places it looked, when the
		// token is in neither -- so there is nothing left here to assert.
		const token = await readStaffInviteToken(invitationId);

		await page.goto(`/accept-invite?token=${token}`);
		await page.getByLabel('Email').fill(doulaEmail);
		await page.getByLabel('Password').fill(password);
		await page.getByRole('button', { name: 'Continue' }).click();
		await expect(page.getByRole('heading', { name: 'Tell us about yourself' })).toBeVisible();
		await page.getByLabel('Your name').fill(doulaName);
		await page.getByLabel('Which state do you work from?').selectOption('New York');
		await page.getByRole('button', { name: 'Accept invite' }).click();
		await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

		return readDoulaStaffId(request, ownerHeaders, practiceId, doulaEmail);
	}
});

/**
 * Reads one Staff member's id off the roster the Owner can already see --
 * the vouch path needs it, and the screen this spec drives gets it from
 * the same place.
 */
async function readDoulaStaffId(
	request: APIRequestContext,
	headers: { Cookie: string },
	practiceId: string,
	email: string
): Promise<string> {
	const roster = await request.get(`${API_URL}/api/practices/${practiceId}/staff`, { headers });
	expect(roster.ok(), `reading the roster failed: ${roster.status()}`).toBe(true);
	const { members } = await roster.json();
	const member = members.find((entry: { email: string }) => entry.email === email);
	expect(member, `no roster entry for ${email}`).toBeTruthy();
	return member.staffId;
}
