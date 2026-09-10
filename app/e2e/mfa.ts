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
// #1132 re-ran that check rather than inheriting it, across
// firebase-tools 15.27.0 (vendored here), 15.28.1 (pinned at the repo
// root) and 15.30.0 (the newest published release as of 2026-09-10):
// `totp` appears in exactly one file under lib/emulator/auth/ in all
// three -- apiSpec.js, the generated OpenAPI schema -- and in none of
// operations.js, state.js, handlers.js, server.js or errors.js. The
// emulator's own words, so nobody has to derive them again:
// mfaEnrollment:start with `totpEnrollmentInfo` answers
// `400 INVALID_ARGUMENT : ((Missing phoneEnrollmentInfo.))`, and
// accounts:update with `mfa.enrollments[].totpInfo` answers
// `400 INVALID_MFA_PHONE_NUMBER : Invalid format.`
//
// A spec that needs a *screen* to meet TOTP -- the sign-in challenge,
// enrollment, or the step-up in front of an Owner vouch -- uses
// totpStub.ts instead of this file, which relabels the emulator's own
// second factor at the browser's network boundary. Its header says what
// stays faked. This file remains the way to get a session carrying the
// claim without a browser at all.
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
//
// The emulator's second named limitation -- **it refuses the body that
// clears a second factor**, which is why no spec here can walk a
// *successful* MFA-recovery spend. The clear goes through the API's
// authn.FirebaseVerifier.ClearSecondFactors, whose Admin SDK call puts
// `mfa.enrollments` on the wire as JSON null; the emulator's schema
// types that field `array` and answers `400 Invalid JSON payload
// received. /mfa/enrollments must be array`. Production Identity
// Platform accepts the same body and clears the factor -- #1128 probed
// it against the real project on 2026-09-10, and the fact and its whole
// reasoning live on ClearSecondFactors' own doc comment, which is the
// one place to correct if this is ever re-probed. The schema was read in
// both firebase-tools 15.27.0 (what app/node_modules holds) and 15.28.1
// (what package.json pins): apiSpec.js's
// GoogleCloudIdentitytoolkitV1MfaInfo types `enrollments` as `array`,
// with no null allowed, in each. The SDK side is
// firebase.google.com/go/v4 v4.21.0, its newest release.
export const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
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
 * Enrolls a phone second factor against idToken's identity and returns
 * the tokens the emulator issues for it -- an ID token carrying
 * `firebase.sign_in_second_factor: "phone"`, the one claim
 * api/internal/authn.secondFactorClaim reads, and the refresh token
 * whose stored record carries the same factor. Feed the ID token to
 * signIn() (auth.ts) to mint a session with second_factor recorded
 * true; there is no other way to get that claim onto a token against
 * this emulator. Most callers want enrollSecondFactor below, which is
 * this call with the refresh token dropped.
 *
 * Runs the real (v2) enrollment dance rather than the Admin-SDK-shaped
 * accounts:update `mfa.enrollments` shortcut on purpose: that shortcut
 * persists mfaInfo on the account for a *future* sign-in to challenge,
 * but never itself issues a token carrying the claim (only
 * mfaEnrollmentFinalize and a completed mfaSignIn call issueTokens with
 * a `secondFactor` argument) -- it would enroll the factor but leave the
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
export async function enrollPhoneFactor(
	request: APIRequestContext,
	idToken: string,
	phoneNumber = randomPhoneNumber()
): Promise<{ idToken: string; refreshToken: string }> {
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
	return JSON.parse(finalizeBody);
}

/**
 * enrollPhoneFactor's ID token alone, which is all any caller here has
 * ever wanted. The refresh token beside it matters only to totpStub.ts,
 * whose enrollment interception has to hand the browser a refresh token
 * whose stored record carries a second factor.
 */
export async function enrollSecondFactor(
	request: APIRequestContext,
	idToken: string,
	phoneNumber = randomPhoneNumber()
): Promise<string> {
	const { idToken: enrolledIdToken } = await enrollPhoneFactor(request, idToken, phoneNumber);
	return enrolledIdToken;
}

/**
 * Reads the code a pending phone verification (enrolment or sign-in)
 * would otherwise only reveal by SMS -- the emulator instead logs it to
 * its own stdout and exposes it on this debug listing, keyed by the same
 * sessionInfo the start call returned. Shared by enrollPhoneFactor
 * above and by totpStub.ts, which drives the sign-in-time MFA challenge
 * (mfaSignIn:start/finalize) rather than enrollment.
 */
export async function readVerificationCode(request: APIRequestContext, sessionInfo: string): Promise<string> {
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
 * verifyEmail + enrollSecondFactor + signIn, composed: the one call a
 * fixture makes when it wants an enrolled Owner's session (or any
 * Staff's, at a Practice that now requires MFA of everyone) without
 * spending a browser on the walk. Every future signInWithPassword for
 * that account demands a second-factor challenge, and against this
 * emulator the factor is PHONE_SMS, which the product's TOTP-only
 * screens cannot answer on their own -- a spec that wants those screens
 * driven installs totpStub.ts instead of calling this. Returns session
 * headers carrying second_factor -- good for calling any
 * Practice-scoped route directly, and for enterPracticeAsEnrolled below
 * to hand the browser.
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
 * equivalent of a /login walk, for a fixture that has no interest in
 * spending one (totpStub.ts is what a spec that does want the walk
 * installs). A direct cookie injection rather than a
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
 * A fixture-only US number in the 555 exchange, never a real dialable
 * one, with enough random digits that two Playwright workers enrolling
 * in the same instant don't collide. Seven random digits after `+1555`
 * rather than the four of the 555-01xx range reserved for fiction: the
 * emulator only parses the shape, nothing ever dials it, and the extra
 * three digits are what buy the collision margin.
 */
export function randomPhoneNumber(): string {
	const digits = String(Math.floor(Math.random() * 10_000_000)).padStart(7, '0');
	return `+1555${digits}`;
}
