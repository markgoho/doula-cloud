import { expect, type APIRequestContext } from '@playwright/test';
import { signInEnrolled } from './mfa';
import { E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { seedFounder } from './stack';
import { uniqueEmail } from './staffSignup';

const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;

/**
 * Signs the founder in (#1526): a fresh Identity Platform account, the
 * `staff` row whose id is the stack's FOUNDER_STAFF_ID pointed at it
 * (stack.ts's seedFounder), and a session that carries a second factor
 * -- `staffauth.FounderOnly` refuses one that does not.
 *
 * No `POST /api/staff/signup` in between, unlike seedFoundingOwner: a
 * signup mints a `staff.id` of its own, and the founder's is fixed by
 * configuration. So he holds no Membership and no Practice, which is the
 * case the read page is built for.
 *
 * Returns the `__session` cookie header, the shape signInEnrolled hands
 * back for every other fixture.
 */
export async function seedFounderSession(request: APIRequestContext): Promise<{ Cookie: string }> {
	const email = uniqueEmail('founder');
	const signUp = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`, {
		data: { email, password: 'password123', returnSecureToken: true }
	});
	expect(signUp.ok(), `founder signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken, localId } = await signUp.json();

	seedFounder(localId, email);

	return signInEnrolled(request, idToken, localId);
}
