// `bun run seed:founder-session` -- mints the founder's session against the
// local stack (`bun run dev:full`) and prints what a browser needs to open
// the founder read page (#1526).
//
// The stack starts the BFF with FOUNDER_STAFF_ID set to a fixed id
// (e2e/stack.ts), and no signup can produce a Staff row with that id. This
// seeds one, pointed at a fresh Identity Platform emulator account, and
// signs it in with a second factor -- `staffauth.FounderOnly` refuses a
// session that has none. It is seedFounderSession (e2e/founder.ts), the
// e2e suite's own fixture, called outside the Playwright runner the way
// seed-staff-session.ts calls its own; that file's header says why the
// emulator needs a phone factor rather than TOTP.
//
// Running it again takes the founder over: the row is re-pointed at the
// new account, and the session printed before stops being the founder's.
import { request } from '@playwright/test';
import { seedFounderSession } from '../e2e/founder';
import { DEV_SERVER_ORIGIN } from '../e2e/ports';

const context = await request.newContext();
try {
	const headers = await seedFounderSession(context);

	console.log(
		JSON.stringify(
			{
				cookieName: '__session',
				cookieValue: headers.Cookie.replace('__session=', ''),
				origin: DEV_SERVER_ORIGIN,
				feedbackUrl: `${DEV_SERVER_ORIGIN}/feedback`
			},
			undefined,
			2
		)
	);
} finally {
	await context.dispose();
}
