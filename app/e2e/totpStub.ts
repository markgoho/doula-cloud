import { expect, type APIRequestContext, type Page } from '@playwright/test';
import { E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';

/*
 * #1132: how a Playwright spec drives a real TOTP screen against an
 * emulator that has no TOTP.
 *
 * ## What was ruled out first
 *
 * The Firebase Auth emulator implements MFA for PHONE_SMS only, and no
 * newer release changes that. `totp` appears in exactly one file under
 * `lib/emulator/auth/` -- `apiSpec.js`, the generated OpenAPI schema --
 * in firebase-tools 15.27.0 (vendored in app/node_modules), 15.28.1
 * (pinned at the repo root) and 15.30.0 (the newest published release,
 * checked on 2026-09-10). `operations.js`, `state.js`, `handlers.js`,
 * `server.js` and `errors.js` hold zero occurrences in all three. The
 * emulator answers, verbatim:
 *
 * - `mfaEnrollment:start` with `totpEnrollmentInfo`:
 *   `400 INVALID_ARGUMENT : ((Missing phoneEnrollmentInfo.))`
 * - `accounts:update` with `mfa.enrollments[].totpInfo`:
 *   `400 INVALID_MFA_PHONE_NUMBER : Invalid format.`
 *
 * Nor can any debug endpoint be coaxed into minting the claim: the only
 * two calls in the emulator that put `firebase.sign_in_second_factor` on
 * an issued ID token, mfaEnrollmentFinalize and mfaSignInFinalize, pass
 * a literal `PROVIDER_PHONE`.
 *
 * ## What this file does instead
 *
 * It relabels the emulator's own second factor at the browser's network
 * boundary, and lets everything else be real. Two facts make that sound:
 * the emulator signs ID tokens with `algorithm: "none"` -- they are
 * unsigned -- and the e2e stack runs the Go BFF with
 * FIREBASE_AUTH_EMULATOR_HOST set, so its Admin SDK verifier skips
 * signature verification and reads the claim straight off the payload.
 * So a token can be handed back with `"phone"` rewritten to `"totp"` and
 * remain a token every layer downstream accepts.
 *
 * Real in a spec that installs this: the product's own /login and
 * /mfa/enroll screens, the Firebase JS SDK's entire TOTP code path
 * (getMultiFactorResolver, TotpMultiFactorGenerator, resolveSignIn,
 * multiFactor().enroll), TotpCodeField, the session exchange, and the
 * Go gate in api/internal/staffauth that reads the claim.
 *
 * **Faked, and never to be read as more:** the second factor's provider
 * string, and whether the six digits are the right six digits. Checking
 * a TOTP code against a shared secret is the live service's contract,
 * not this stub's -- so any well-formed 6-digit code is accepted and
 * anything else is refused as INVALID_CODE, which is the server error
 * the SDK maps to `auth/invalid-verification-code`. A spec that wants to
 * prove a *wrong* code is refused gets that from this stub's shape
 * check, and should say so where it asserts it.
 *
 * **Token refresh is out of reach.** `securetoken` refresh reissues from
 * the stored refresh-token record, whose secondFactor is still `phone`.
 * The session cookie is minted from the first token, so a spec that
 * signs in and lands is safe; one that lingers past a refresh is not.
 */

const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;

// The e2e stack always starts the emulator against this one project (see
// stack.ts's `--project doula-cloud`), the same constant mfa.ts pins for
// the same reason.
const PROJECT_ID = 'doula-cloud';

// A syntactically valid, fixture-only E.164 US number in the 555-01xx
// range reserved for fiction (ITU/NANP). The phone factor behind every
// relabeled token is enrolled against one of these; it is never
// dialable and never shown to anyone.
function randomPhoneNumber(): string {
	const digits = String(Math.floor(Math.random() * 10_000_000)).padStart(7, '0');
	return `+1555${digits}`;
}

/*
 * The base32 secret the enrollment screen renders as a QR code and
 * offers for copying. Fixture text with a recognizable shape rather than
 * a real seed: nothing ever computes a code from it (see this file's
 * header on what is faked), and a reader who finds it in a screenshot
 * should be able to tell at a glance that it is not a credential.
 */
const FIXTURE_SHARED_SECRET = 'DOULACLOUDE2ETOTPSTUBSECRET2AAA';

/**
 * A well-formed 6-digit code, for a spec that means "she typed what her
 * app showed".
 */
export const STUB_TOTP_CODE = '123456';

// Identity Platform's own path shapes, matched on pathname rather than a
// full-URL glob so the worktree port offset (ports.ts) and the SDK's
// `?key=` query are both irrelevant.
function matchesPath(suffix: string): (url: URL) => boolean {
	return (url) => url.pathname.endsWith(suffix);
}

/**
 * The emulator's error envelope, in the shape the JS SDK unwraps to an
 * `auth/...` code.
 */
function serverError(message: string) {
	return { error: { code: 400, message, errors: [{ message, reason: 'invalid', domain: 'global' }] } };
}

/**
 * Whether what the screen sent is shaped like an authenticator app's
 * output.
 */
function isWellFormedCode(code: unknown): boolean {
	return typeof code === 'string' && /^\d{6}$/.test(code);
}

/**
 * Rewrites `firebase.sign_in_second_factor` to `"totp"` on an emulator
 * ID token, leaving every other claim -- iss, aud, exp, sub, the
 * enrollment identifier -- exactly as issued. The emulator's tokens are
 * unsigned (`algorithm: "none"`), so the signature segment is empty and
 * there is nothing to re-sign; splicing the payload back in is the whole
 * operation.
 */
function relabelSecondFactor(idToken: string): string {
	const [header, payload, signature] = idToken.split('.', 3);
	const claims = JSON.parse(Buffer.from(payload, 'base64url').toString('utf8'));
	claims.firebase = { ...claims.firebase, sign_in_second_factor: 'totp' };
	const rewritten = Buffer.from(JSON.stringify(claims), 'utf8').toString('base64url');
	return [header, rewritten, signature].join('.');
}

/**
 * Runs the emulator's real phone sign-in challenge for a pending
 * credential and returns the tokens it issues. The verification code the
 * real flow would text out is only ever logged to the emulator's own
 * stdout, so it is read back over the debug listing -- the same
 * mechanism mfa.ts uses for enrollment.
 */
async function resolvePhoneChallenge(
	request: APIRequestContext,
	mfaPendingCredential: string,
	mfaEnrollmentId: string
): Promise<{ idToken: string; refreshToken: string }> {
	const start = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v2/accounts/mfaSignIn:start?key=fake-key`, {
		data: { mfaPendingCredential, mfaEnrollmentId }
	});
	expect(start.ok(), `mfaSignIn:start failed: ${start.status()} ${await start.text()}`).toBe(true);
	const {
		phoneResponseInfo: { sessionInfo }
	} = await start.json();

	const code = await readVerificationCode(request, sessionInfo);

	const finalize = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v2/accounts/mfaSignIn:finalize?key=fake-key`,
		{ data: { mfaPendingCredential, phoneVerificationInfo: { sessionInfo, code } } }
	);
	expect(finalize.ok(), `mfaSignIn:finalize failed: ${finalize.status()} ${await finalize.text()}`).toBe(true);
	return await finalize.json();
}

/**
 * Runs the emulator's real phone enrollment for an already-signed-in
 * identity and returns the ID token it issues, which is the only token
 * the emulator ever stamps a second factor onto. `idToken`'s identity
 * must already have a verified email (mfa.ts's verifyEmail) or the
 * emulator refuses with UNVERIFIED_EMAIL, exactly as the live service
 * does.
 */
async function enrollPhoneFactor(
	request: APIRequestContext,
	idToken: string
): Promise<{ idToken: string; refreshToken: string }> {
	const phoneNumber = randomPhoneNumber();
	const start = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v2/accounts/mfaEnrollment:start?key=fake-key`,
		{ data: { idToken, phoneEnrollmentInfo: { phoneNumber } } }
	);
	expect(start.ok(), `mfaEnrollment:start failed: ${start.status()} ${await start.text()}`).toBe(true);
	const {
		phoneSessionInfo: { sessionInfo }
	} = await start.json();

	const code = await readVerificationCode(request, sessionInfo);

	const finalize = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v2/accounts/mfaEnrollment:finalize?key=fake-key`,
		{ data: { idToken, phoneVerificationInfo: { sessionInfo, code } } }
	);
	expect(finalize.ok(), `mfaEnrollment:finalize failed: ${finalize.status()} ${await finalize.text()}`).toBe(true);
	return await finalize.json();
}

async function readVerificationCode(request: APIRequestContext, sessionInfo: string): Promise<string> {
	const response = await request.get(`${EMULATOR_URL}/emulator/v1/projects/${PROJECT_ID}/verificationCodes`);
	expect(response.ok(), `listing verification codes failed: ${response.status()}`).toBe(true);
	const { verificationCodes } = await response.json();
	const match = verificationCodes.find((entry: { sessionInfo: string; code: string }) => entry.sessionInfo === sessionInfo);
	expect(match, `no verification code logged for sessionInfo ${sessionInfo}`).toBeTruthy();
	return match.code;
}

/**
 * Makes page's browser see TOTP wherever the emulator would show
 * PHONE_SMS, for as long as the page lives. Install it before the first
 * navigation that could reach /login or /mfa/enroll.
 *
 * `request` is a Playwright APIRequestContext used for the stub's own
 * calls back to the emulator. It is deliberately a *different* context
 * from the page: page routes never apply to it, so the stub's own
 * traffic cannot re-enter its own handlers.
 *
 * Three interceptions, and nothing else touched:
 *
 * - `accounts:signInWithPassword` passes through, and any second factor
 *   the emulator names in its MFA-pending answer is relabeled from
 *   `phoneInfo` to `totpInfo`. That is what makes the JS SDK build a
 *   TotpMultiFactorInfo hint, so the screens meet the factor the product
 *   actually ships.
 * - `mfaEnrollment:start` answers a TOTP enrollment with a synthesized
 *   `totpSessionInfo`. The shared secret is fixture text: it is rendered
 *   into a QR code and shown for copying, and never checked against
 *   anything.
 * - `mfaEnrollment:finalize` and `mfaSignIn:finalize` check that the
 *   code is six digits, then let the emulator do the real work over its
 *   phone factor and hand back its token with the provider relabeled.
 */
export async function stubTotpFactor(page: Page, request: APIRequestContext): Promise<void> {
	await page.route(matchesPath('/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword'), async (route) => {
		const response = await route.fetch();
		const body = await response.json();
		if (Array.isArray(body.mfaInfo)) {
			body.mfaInfo = body.mfaInfo.map((enrollment: Record<string, unknown>) => {
				const relabeled = { ...enrollment, totpInfo: {} };
				delete relabeled.phoneInfo;
				return relabeled;
			});
		}
		await route.fulfill({ response, json: body });
	});

	await page.route(matchesPath('/identitytoolkit.googleapis.com/v2/accounts/mfaEnrollment:start'), async (route) => {
		const body = route.request().postDataJSON();
		if (!body?.totpEnrollmentInfo) {
			await route.fallback();
			return;
		}
		await route.fulfill({
			json: {
				totpSessionInfo: {
					sharedSecretKey: FIXTURE_SHARED_SECRET,
					verificationCodeLength: 6,
					hashingAlgorithm: 'SHA1',
					periodSec: 30,
					sessionInfo: 'totp-stub-session',
					finalizeEnrollmentTime: new Date(Date.now() + 5 * 60 * 1000).toISOString()
				}
			}
		});
	});

	await page.route(matchesPath('/identitytoolkit.googleapis.com/v2/accounts/mfaEnrollment:finalize'), async (route) => {
		const body = route.request().postDataJSON();
		if (!body?.totpVerificationInfo) {
			await route.fallback();
			return;
		}
		if (!isWellFormedCode(body.totpVerificationInfo.verificationCode)) {
			await route.fulfill({ status: 400, json: serverError('INVALID_CODE : Invalid TOTP verification code.') });
			return;
		}
		const enrolled = await enrollPhoneFactor(request, body.idToken);
		await route.fulfill({
			json: { idToken: relabelSecondFactor(enrolled.idToken), refreshToken: enrolled.refreshToken }
		});
	});

	await page.route(matchesPath('/identitytoolkit.googleapis.com/v2/accounts/mfaSignIn:finalize'), async (route) => {
		const body = route.request().postDataJSON();
		if (!body?.totpVerificationInfo) {
			await route.fallback();
			return;
		}
		if (!isWellFormedCode(body.totpVerificationInfo.verificationCode)) {
			await route.fulfill({ status: 400, json: serverError('INVALID_CODE : Invalid TOTP verification code.') });
			return;
		}
		const { idToken, refreshToken } = await resolvePhoneChallenge(request, body.mfaPendingCredential, body.mfaEnrollmentId);
		await route.fulfill({ json: { idToken: relabelSecondFactor(idToken), refreshToken } });
	});

	/*
	 * A refreshed token has to keep telling the same story. The screens
	 * force a refresh right after enrollment (the just-added claim is not
	 * on the cached token), and the emulator reissues from the stored
	 * refresh-token record, which still says `phone`. Only a token that
	 * already carries a second factor is relabeled here -- an ordinary
	 * refresh for an identity with no second factor passes through
	 * untouched, so nothing is ever told about a factor that does not
	 * exist.
	 */
	await page.route(matchesPath('/securetoken.googleapis.com/v1/token'), async (route) => {
		const response = await route.fetch();
		const body = await response.json();
		if (typeof body.id_token === 'string' && hasSecondFactor(body.id_token)) {
			body.id_token = relabelSecondFactor(body.id_token);
			body.access_token = body.id_token;
		}
		await route.fulfill({ response, json: body });
	});
}

/**
 * Whether an emulator ID token already names a second factor of any
 * provider.
 */
function hasSecondFactor(idToken: string): boolean {
	const payload = idToken.split('.', 3)[1];
	if (payload === undefined) return false;
	const claims = JSON.parse(Buffer.from(payload, 'base64url').toString('utf8'));
	return Boolean(claims?.firebase?.sign_in_second_factor);
}

