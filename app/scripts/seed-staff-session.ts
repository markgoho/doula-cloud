// `bun run seed:staff-session` -- mints a Staff session against the local
// stack (`bun run dev:full`) without walking the login form, and prints the
// pieces an automated agent needs to drop straight into a real browser.
//
// It signs up a founding Owner and signs her in with no second factor, by
// default. That is the Owner a real signup makes: since #1531 she enters
// her Practice without one, and the screens she meets then -- the
// second-factor recommendation (#1533), and the notice on each of the five
// acts that need one (#1532) -- are drawn only for her.
//
// SEED_SECOND_FACTOR=1 signs her in with a second factor instead, for a
// walk of one of those five acts (api/internal/staffauth/secondfactor.go's
// SecondFactorActs) or the Practice's mfa-required switch. The Firebase
// Auth emulator vendored in app/node_modules has no TOTP enrollment path
// -- only PHONE_SMS (firebase/firebase-tools#6224) -- so the real
// /mfa/enroll screen 400s against it, and this uses mfa.ts's fixture
// workaround: enroll a phone factor and mint the session off the token it
// issues. The session records the claim, and nothing that reads it back
// cares which provider produced it.
//
// Both paths reuse the e2e suite's own helpers (seedFoundingOwner, auth.ts's
// signIn, mfa.ts's signInEnrolled), called outside the Playwright test
// runner via `request.newContext()`, so an agent can run this against a
// long-lived `dev:full` session rather than an ephemeral e2e one; nothing
// here re-derives the request shape #825 already consolidated.
//
// What this does NOT exercise: the login form, the TOTP QR/secret screen,
// or code entry -- those still need a real Identity Platform project (see
// docs/testing/e2e.md's "Logging in as Staff locally" section) or the
// deployed app. Everything downstream of authentication -- every
// Practice-scoped screen and API route -- runs for real.
//
// SEED_CLIENT=1 also gives the Practice one Client and one Engagement for
// her, through the same seedClient/seedEngagement helpers the e2e specs
// use, and prints their URLs -- the walk most screens past the Overview
// need, which agents used to write a throwaway script for (#1679).
import { request } from '@playwright/test';
import { seedFoundingOwner } from '../e2e/staffSignup';
import { signIn } from '../e2e/auth';
import { signInEnrolled } from '../e2e/mfa';
import { seedClient } from '../e2e/portalClient';
import { seedEngagement } from '../e2e/stack';
import { API_URL, DEV_SERVER_ORIGIN } from '../e2e/ports';

const practiceName = process.env.SEED_PRACTICE_NAME;
const staffName = process.env.SEED_STAFF_NAME;
const workState = process.env.SEED_WORK_STATE;
const hasSecondFactor = process.env.SEED_SECOND_FACTOR === '1';

const context = await request.newContext();
try {
	const owner = await seedFoundingOwner(context, { practiceName, staffName, workState });
	const headers = hasSecondFactor
		? await signInEnrolled(context, owner.idToken, owner.localId)
		: await signIn(context, API_URL, owner.idToken);
	const cookieValue = headers.Cookie.replace('__session=', '');
	const practiceUrl = `${DEV_SERVER_ORIGIN}/practices/${owner.practiceId}`;
	let walk = {};
	if (process.env.SEED_CLIENT === '1') {
		const clientId = await seedClient(context, owner.practiceId, headers, { givenName: 'Ada', familyName: 'Walker' });
		const engagementId = seedEngagement(clientId, owner.practiceId);
		walk = {
			clientId,
			clientUrl: `${practiceUrl}/clients/${clientId}`,
			engagementId,
			engagementUrl: `${practiceUrl}/engagements/${engagementId}`
		};
	}

	console.log(
		JSON.stringify(
			{
				cookieName: '__session',
				cookieValue,
				origin: DEV_SERVER_ORIGIN,
				practiceUrl,
				practiceId: owner.practiceId,
				staffId: owner.staffId,
				email: owner.email,
				secondFactor: hasSecondFactor,
				...walk
			},
			undefined,
			2
		)
	);
} finally {
	await context.dispose();
}
