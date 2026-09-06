import { expect, test, type APIRequestContext } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT, PREVIEW_SERVER_ORIGIN } from './ports';
import { signIn, sessionCookieFrom } from './auth';
import { readStaffInviteToken } from './stack';
import { enrollSecondFactor, verifyEmail } from './mfa';
import { seedContractorDoula, PORTAL_CLIENT_PASSWORD } from './portalClient';

// #606: TOTP MFA for Staff, required for Owners always, and raisable to
// required-for-everyone per Practice by the mfa-required switch. The
// Firebase Auth emulator has no TOTP path (see mfa.ts's doc comment), so
// these specs get a session carrying `firebase.sign_in_second_factor`
// the same way mfa.ts does for every fixture that needs one -- through
// the emulator's PHONE_SMS enrolment -- and prove the gate itself
// (api/internal/staffauth/middleware.go) via direct API calls rather
// than the product's own login/enrolment screens, which are separate
// work landing on this branch at the same time as this file.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

/**
 * Provisions a fresh Practice with an enrolled Owner: signs up, verifies
 * the email (mfaEnrollmentStart refuses an unverified one), enrols a
 * phone second factor, and signs in with the resulting claim-carrying
 * token so ownerHeaders can call any Practice-scoped route immediately --
 * an Owner is gated by #606 regardless of the Practice's own switch, so
 * every other seeding step in these specs (inviting Staff, throwing the
 * switch) needs this first.
 */
async function seedEnrolledOwner(
	request: APIRequestContext,
	practiceName: string
): Promise<{ practiceId: string; ownerEmail: string; ownerHeaders: { Cookie: string } }> {
	const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	const ownerEmail = `owner-${unique}@example.com`;

	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: ownerEmail, password: PORTAL_CLIENT_PASSWORD, returnSecureToken: true } }
	);
	expect(signUp.ok(), `owner signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken, localId } = await signUp.json();

	await verifyEmail(request, localId);
	const enrolledIdToken = await enrollSecondFactor(request, idToken);

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${enrolledIdToken}` },
		data: { practiceName, staffName: 'Riley Owner', workState: 'NY' }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `owner staff/signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	const ownerHeaders = await signIn(request, API_URL, enrolledIdToken);
	return { practiceId, ownerEmail, ownerHeaders };
}

/** GET .../session is the lightest membership-gated route: any Staff
 * holding a Membership at practiceId may call it, so it exercises
 * exactly #606's gate and nothing else a stricter role check might also
 * refuse for an unrelated reason. */
function sessionURL(practiceId: string): string {
	return `${API_URL}/api/practices/${practiceId}/session`;
}

async function expectMFARequired(request: APIRequestContext, url: string, headers: { Cookie: string }) {
	const response = await request.get(url, { headers });
	expect(response.status(), `expected 403 MFA_REQUIRED, got ${response.status()}`).toBe(403);
	const body = await response.json();
	expect(body.code).toBe('MFA_REQUIRED');
}

async function expectAdmitted(request: APIRequestContext, url: string, headers: { Cookie: string }) {
	const response = await request.get(url, { headers });
	expect(response.ok(), `expected 200, got ${response.status()} ${await response.text()}`).toBe(true);
}

test('a Doula with no second factor is barred once the Practice requires MFA for all Staff', async ({
	page,
	request
}) => {
	const { practiceId, ownerHeaders } = await seedEnrolledOwner(request, 'Riverside Doulas');

	const throwSwitch = await request.put(`${API_URL}/api/practices/${practiceId}/mfa-required`, {
		headers: { ...ownerHeaders, 'X-Confirmed': 'true' },
		data: { required: true }
	});
	expect(throwSwitch.status(), `mfa-required PUT failed: ${throwSwitch.status()} ${await throwSwitch.text()}`).toBe(
		204
	);

	const doula = await seedContractorDoula(request, practiceId, ownerHeaders);

	// The tight proof: the gate itself, independent of anything the app's
	// own login/enrolment screens (landing on this branch alongside this
	// file) do with the refusal.
	await expectMFARequired(request, sessionURL(practiceId), doula.headers);

	// The product-facing proof: apiFetchWithSession (app/src/lib/api.ts,
	// already built ahead of this file) routes a live MFA_REQUIRED
	// refusal to /mfa/enroll rather than treating it as a signed-out
	// session, carrying returnTo so enrolment can send her back to the
	// Practice she was trying to reach -- checked here, not just the bare
	// pathname, so a redirect that lands on /mfa/enroll for the wrong
	// reason (or clobbers returnTo with /mfa/enroll itself, the shape of
	// the race api.spec.ts's own guard tests pin down) cannot pass this
	// silently. She has no second factor enrolled at all yet, so the
	// plain password sign-in itself completes in one step -- it is only
	// the Practice landing page's own first Practice-scoped fetch that
	// trips the gate and redirects her.
	await page.goto('/login');
	await page.getByLabel('Email').fill(doula.email);
	await page.getByLabel('Password').fill(PORTAL_CLIENT_PASSWORD);
	await page.getByRole('button', { name: 'Log in' }).click();
	const returnTo = encodeURIComponent(`/practices/${practiceId}`);
	await expect(page).toHaveURL(new RegExp(String.raw`/mfa/enroll\?returnTo=${returnTo}$`));
});

test('the same Doula is admitted once her session carries a second factor', async ({ request, context }) => {
	const { practiceId, ownerHeaders } = await seedEnrolledOwner(request, 'Riverside Doulas');

	const throwSwitch = await request.put(`${API_URL}/api/practices/${practiceId}/mfa-required`, {
		headers: { ...ownerHeaders, 'X-Confirmed': 'true' },
		data: { required: true }
	});
	expect(throwSwitch.status()).toBe(204);

	const doula = await seedContractorDoula(request, practiceId, ownerHeaders);
	await expectMFARequired(request, sessionURL(practiceId), doula.headers);

	// accept-invite already verified her email as a side effect of
	// AcceptInviteHandler (#613) -- enrolSecondFactor needs nothing more
	// than her own idToken from that same acceptance.
	const enrolledIdToken = await enrollSecondFactor(request, doula.idToken);
	const enrolledHeaders = await signIn(request, API_URL, enrolledIdToken);

	await expectAdmitted(request, sessionURL(practiceId), enrolledHeaders);

	// A browser-level proof too, via a direct cookie injection rather
	// than a second interactive /login walk: once mfaInfo exists on an
	// account, every future password sign-in demands the same
	// PHONE_SMS challenge the emulator issued it with, which the
	// product's own login screen is built to resolve against a TOTP
	// hint, not a phone one (see mfa.ts's doc comment) -- so this is the
	// one place in this file that cannot walk /login a second time for
	// the same identity. `secure: false` sidesteps whether Playwright's
	// request-context cookie jar treats loopback as a trustworthy
	// origin the same way a real browser does; the server never reads
	// this attribute back off an inbound Cookie header.
	const token = enrolledHeaders.Cookie.replace('__session=', '');
	await context.addCookies([
		{ name: '__session', value: token, url: PREVIEW_SERVER_ORIGIN, httpOnly: true, secure: false, sameSite: 'Lax' }
	]);
	const page = await context.newPage();
	await page.goto(`/practices/${practiceId}`);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));
});

test('one identity, two Practices: MFA required at one and not the other', async ({ request }) => {
	// X signs up as Owner at Practice A -- unenrolled for now, since the
	// point of this spec is what her session looks like before and after
	// she gets a second factor, at both Practices at once.
	const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	const xEmail = `split-owner-${unique}@example.com`;

	const xSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: xEmail, password: PORTAL_CLIENT_PASSWORD, returnSecureToken: true } }
	);
	expect(xSignUp.ok(), `X signUp failed: ${xSignUp.status()} ${await xSignUp.text()}`).toBe(true);
	const { idToken: xIdToken } = await xSignUp.json();

	const signupA = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${xIdToken}` },
		data: { practiceName: 'Practice A', staffName: 'Sasha Owner', workState: 'NY' }
	});
	const signupABody = await signupA.text();
	expect(signupA.ok(), `Practice A signup failed: ${signupA.status()} ${signupABody}`).toBe(true);
	const { practiceId: practiceIdA } = JSON.parse(signupABody);
	const xHeadersAtA = await signIn(request, API_URL, xIdToken);

	// Y signs up as Owner at Practice B, enrols (an Owner is always
	// gated, switch or no switch), and invites X's own email as a plain
	// Doula there -- one identity holding roles at two Practices, per
	// #606's own two-Practice-split criterion.
	const { practiceId: practiceIdB, ownerHeaders: yHeaders } = await seedEnrolledOwner(request, 'Practice B');

	const invite = await request.post(`${API_URL}/api/practices/${practiceIdB}/staff/invitations`, {
		headers: yHeaders,
		data: { email: xEmail, roles: ['doula'], employmentType: 'contractor' }
	});
	const inviteBody = await invite.text();
	expect(invite.ok(), `invite to Practice B failed: ${invite.status()} ${inviteBody}`).toBe(true);
	const { invitationId } = JSON.parse(inviteBody);
	const inviteToken = readStaffInviteToken(invitationId);
	expect(inviteToken, `no pending invite token for ${invitationId}`).toBeTruthy();

	const accept = await request.post(`${API_URL}/api/staff/accept-invite`, {
		headers: { Authorization: `Bearer ${xIdToken}` },
		data: { inviteToken, name: 'Sasha Owner', workState: 'NY' }
	});
	const acceptBody = await accept.text();
	expect(accept.ok(), `X accept-invite at Practice B failed: ${accept.status()} ${acceptBody}`).toBe(true);

	// Un-enrolled: barred at A (Owner), admitted at B (plain Doula,
	// switch never thrown there).
	await expectMFARequired(request, sessionURL(practiceIdA), xHeadersAtA);
	const xHeadersAtB = sessionCookieFrom(accept, 'X accept-invite at Practice B');
	await expectAdmitted(request, sessionURL(practiceIdB), xHeadersAtB);

	// Enrolled: a single fresh sign-in, carrying the claim, admits her to
	// both -- the gate reads the session row, not a per-Practice fact.
	const enrolledIdToken = await enrollSecondFactor(request, xIdToken);
	const xEnrolledHeaders = await signIn(request, API_URL, enrolledIdToken);
	await expectAdmitted(request, sessionURL(practiceIdA), xEnrolledHeaders);
	await expectAdmitted(request, sessionURL(practiceIdB), xEnrolledHeaders);
});
