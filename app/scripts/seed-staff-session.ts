// `bun run seed:staff-session` -- mints a Staff session against the local
// stack (`bun run dev:full`) without walking the login form or the MFA
// enrollment screen, and prints the pieces an automated agent needs to drop
// straight into a real browser.
//
// Why this exists (#900): the Firebase Auth emulator vendored in
// app/node_modules has no TOTP enrollment path at all -- only PHONE_SMS --
// so the product's real /mfa/enroll screen (TotpMultiFactorGenerator,
// app/src/routes/mfa/enroll/+page.svelte) 400s against it with
// `INVALID_ARGUMENT : ((Missing phoneEnrollmentInfo.))` the instant it
// calls generateSecret(). That is an emulator capability gap
// (firebase/firebase-tools#6224), not a bug in this app's client code, the
// BFF, seed data, or local config -- confirmed by reproducing the 400
// directly against a standalone emulator instance and cross-checked
// against mfa.ts's own doc comment, which independently verified the same
// thing while building #606's e2e fixtures. An Owner always needs a second
// factor (staffauth.Middleware), so this wall blocks reaching *any*
// Practice screen as Staff locally -- not just the enrollment screen itself.
//
// The fix is not new product code: mfa.ts already has a full, audited
// workaround for e2e fixtures -- enroll the emulator's one working
// provider (PHONE_SMS) instead of TOTP, and mint a session off the
// resulting token. api/internal/staffauth/middleware.go's gate reads only
// the `firebase.sign_in_second_factor` claim; it does not care which
// provider produced it. This script is that same workaround, called
// outside the Playwright test runner via `request.newContext()` (which
// works standalone -- see the doc comment on why this is safe) so an
// agent can run it against a long-lived `dev:full` session rather than an
// ephemeral e2e one. It reuses seedFoundingOwner/signInEnrolled verbatim;
// nothing here re-derives the request shape #825 already consolidated.
//
// What this does NOT exercise: the login form, the TOTP QR/secret screen,
// or code entry -- those still need a real Identity Platform project (see
// docs/testing.md's "Logging in as Staff locally" section) or the
// deployed app. Everything downstream of authentication -- every
// Practice-scoped screen and API route -- runs for real.
import { request } from '@playwright/test';
import { seedFoundingOwner } from '../e2e/staffSignup';
import { signInEnrolled } from '../e2e/mfa';
import { DEV_SERVER_ORIGIN } from '../e2e/ports';

const practiceName = process.env.SEED_PRACTICE_NAME;
const staffName = process.env.SEED_STAFF_NAME;
const workState = process.env.SEED_WORK_STATE;

const context = await request.newContext();
try {
	const owner = await seedFoundingOwner(context, { practiceName, staffName, workState });
	const headers = await signInEnrolled(context, owner.idToken, owner.localId);
	const cookieValue = headers.Cookie.replace('__session=', '');

	console.log(
		JSON.stringify(
			{
				cookieName: '__session',
				cookieValue,
				origin: DEV_SERVER_ORIGIN,
				practiceUrl: `${DEV_SERVER_ORIGIN}/practices/${owner.practiceId}`,
				practiceId: owner.practiceId,
				staffId: owner.staffId,
				email: owner.email
			},
			undefined,
			2
		)
	);
} finally {
	await context.dispose();
}
