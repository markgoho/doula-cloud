import { expect, type APIRequestContext, type BrowserContext, type Page } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT, PREVIEW_SERVER_ORIGIN } from './ports';
import { signIn } from './auth';

// The Firebase Auth emulator (firebase-tools 15.27, vendored in
// app/node_modules) implements MFA enrolment for PHONE_SMS only --
// mfaEnrollmentStart/Finalize assert phoneEnrollmentInfo and stamp every
// issued factor with the `phone` provider, and there is no TOTP path to
// drive against it. #606 ships TOTP as the product's only second factor
// (SMS is never reachable anywhere in the real flow -- see the product's
// own login/enrol screens), so this file exists purely to get a session
// carrying `firebase.sign_in_second_factor` for a fixture identity: the
// gate in api/internal/staffauth/middleware.go reads exactly that claim,
// off the session row, and does not care which provider produced it.
//
// MFA itself needs no enabling here: AgentProjectState.mfaConfig in
// firebase-tools' emulator/auth/state.js is hardcoded to
// `{state: "ENABLED", enabledProviders: ["PHONE_SMS"]}` for the default
// (non-tenant) project -- there is no firebase.json key for it, and none
// is needed. Confirmed empirically against a standalone
// `emulators:start --only auth --project doula-cloud` (the exact
// invocation stack.ts uses): accounts:signUp, the emailVerified update,
// and the full mfaEnrollment:start/finalize dance below all worked with
// no config change at all.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

// The e2e stack always starts the emulator against this one project (see
// stack.ts's `--project doula-cloud`), so the emulator's own debug/admin
// endpoints -- listVerificationCodes below, and every project-scoped
// path -- are addressed against it directly rather than threaded through
// as a parameter nothing here would ever vary.
const PROJECT_ID = 'doula-cloud';

/**
 * Marks localId's email verified via the emulator's privileged
 * accounts:update -- `Authorization: Bearer owner` is the magic value
 * server.js's auth middleware treats as an authenticated Admin-SDK
 * caller (the same thing a real Admin SDK call presents, minus real
 * Google credentials), which is what unlocks setting emailVerified
 * directly rather than through an email link nothing here can click.
 * mfaEnrollmentStart refuses to enrol an unverified email
 * (UNVERIFIED_EMAIL) exactly as the live service does, so this has to
 * run before enrollSecondFactor for a freshly signed-up fixture.
 */
export async function verifyEmail(request: APIRequestContext, localId: string): Promise<void> {
	const response = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:update`, {
		headers: { Authorization: 'Bearer owner' },
		data: { localId, emailVerified: true }
	});
	expect(response.ok(), `verifyEmail failed: ${response.status()} ${await response.text()}`).toBe(true);
}

/**
 * Enrols a phone second factor against idToken's identity and returns a
 * fresh ID token carrying `firebase.sign_in_second_factor: "phone"` --
 * the one claim api/internal/authn.secondFactorClaim reads. Feed the
 * returned token to signIn() (auth.ts) to mint a session with
 * second_factor recorded true; there is no other way to get that claim
 * onto a token against this emulator.
 *
 * Runs the real (v2) enrolment dance rather than the Admin-SDK-shaped
 * accounts:update `mfa.enrollments` shortcut on purpose: that shortcut
 * persists mfaInfo on the account for a *future* sign-in to challenge,
 * but never itself issues a token carrying the claim (only
 * mfaEnrollmentFinalize and a completed mfaSignIn call issueTokens with
 * a `secondFactor` argument) -- it would enrol the factor but leave the
 * caller with nothing to hand signIn().
 *
 * localId must already have a verified email (verifyEmail above) or
 * this fails with UNVERIFIED_EMAIL. The verification code the real
 * phone flow would text out is never sent anywhere -- the emulator only
 * logs it to its own stdout -- so this reads it back over the debug
 * `verificationCodes` listing endpoint instead, matching a sessionInfo
 * value nothing else in this fixture could guess.
 *
 * Give every fixture identity its own phoneNumber (randomPhoneNumber
 * below): mfaEnrollmentStart's own uniqueness check is scoped to the one
 * account, but two Playwright workers enrolling through the emulator at
 * the same instant are still better off never sharing one.
 */
export async function enrollSecondFactor(
	request: APIRequestContext,
	idToken: string,
	phoneNumber = randomPhoneNumber()
): Promise<string> {
	const start = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v2/accounts/mfaEnrollment:start?key=fake-key`,
		{ data: { idToken, phoneEnrollmentInfo: { phoneNumber } } }
	);
	const startBody = await start.text();
	expect(start.ok(), `mfaEnrollment:start failed: ${start.status()} ${startBody}`).toBe(true);
	const {
		phoneSessionInfo: { sessionInfo }
	} = JSON.parse(startBody);

	const code = await readVerificationCode(request, sessionInfo);

	const finalize = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v2/accounts/mfaEnrollment:finalize?key=fake-key`,
		{ data: { idToken, phoneVerificationInfo: { sessionInfo, code } } }
	);
	const finalizeBody = await finalize.text();
	expect(finalize.ok(), `mfaEnrollment:finalize failed: ${finalize.status()} ${finalizeBody}`).toBe(true);
	const { idToken: enrolledIdToken } = JSON.parse(finalizeBody);
	return enrolledIdToken;
}

/**
 * Reads the code a pending phone verification (enrolment or sign-in)
 * would otherwise only reveal by SMS -- the emulator instead logs it to
 * its own stdout and exposes it on this debug listing, keyed by the same
 * sessionInfo the start call returned. Shared by enrollSecondFactor
 * above and any future caller driving the sign-in-time MFA challenge
 * (mfaSignIn:start/finalize) rather than enrolment.
 */
async function readVerificationCode(request: APIRequestContext, sessionInfo: string): Promise<string> {
	const response = await request.get(`${EMULATOR_URL}/emulator/v1/projects/${PROJECT_ID}/verificationCodes`);
	expect(response.ok(), `listing verification codes failed: ${response.status()}`).toBe(true);
	const { verificationCodes } = await response.json();
	const match = verificationCodes.find(
		(entry: { sessionInfo: string; code: string }) => entry.sessionInfo === sessionInfo
	);
	expect(match, `no verification code logged for sessionInfo ${sessionInfo}`).toBeTruthy();
	return match.code;
}

/**
 * verifyEmail + enrollSecondFactor + signIn, composed: the one call every
 * fixture that used to walk /login as an Owner (or any Staff a Practice
 * now requires MFA from) needs instead, since that account can no longer
 * complete a plain password sign-in through the real login form (see
 * enrollSecondFactor's own doc comment on why -- every future
 * signInWithPassword for it now demands a PHONE_SMS challenge the
 * product's TOTP-only login screen cannot resolve). Returns session
 * headers carrying second_factor -- good for calling any Practice-scoped
 * route directly, and for enterPracticeAsEnrolled below to hand the
 * browser.
 */
export async function signInEnrolled(
	request: APIRequestContext,
	idToken: string,
	localId: string
): Promise<{ Cookie: string }> {
	await verifyEmail(request, localId);
	const enrolledIdToken = await enrollSecondFactor(request, idToken);
	return signIn(request, API_URL, enrolledIdToken);
}

/**
 * Puts headers' session cookie directly into context's cookie jar and
 * navigates page to practiceId's landing route -- the browser-level
 * equivalent of the /login walk these fixtures can no longer perform
 * once their Owner is enrolled. A direct cookie injection rather than a
 * second interactive sign-in on purpose: Playwright's request-context
 * `secure: false` sidesteps whether its own cookie jar treats loopback
 * as a trustworthy origin the way a real browser does (see auth.ts's
 * signIn doc comment on the same question) -- the server never reads
 * this attribute back off an inbound Cookie header, so it costs nothing
 * to relax here.
 */
export async function enterPracticeAsEnrolled(
	context: BrowserContext,
	page: Page,
	headers: { Cookie: string },
	practiceId: string
): Promise<void> {
	const token = headers.Cookie.replace('__session=', '');
	await context.addCookies([
		{ name: '__session', value: token, url: PREVIEW_SERVER_ORIGIN, httpOnly: true, secure: false, sameSite: 'Lax' }
	]);
	await page.goto(`/practices/${practiceId}`);
}

/**
 * A syntactically valid, fixture-only E.164 US number in the 555
 * exchange -- never a real, dialable number (555-01xx is the range
 * reserved for fiction, ITU/NANP) -- with enough random digits that two
 * Playwright workers enrolling in the same instant don't collide.
 */
function randomPhoneNumber(): string {
	const digits = String(Math.floor(Math.random() * 10_000_000)).padStart(7, '0');
	return `+1555${digits}`;
}
