import { expect, type APIRequestContext } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { signIn } from './auth';

const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

const FOUNDING_OWNER_PASSWORD = 'password123';

export interface FoundingOwnerFields {
	practiceName?: string;
	staffName?: string;
	workState?: string;
}

export interface SeededFoundingOwner {
	email: string;
	password: string;
	idToken: string;
	localId: string;
	practiceId: string;
	staffId: string;
}

/**
 * Provisions a Practice's founding Owner: an Identity Platform account,
 * then POST /api/staff/signup with it. Returns the raw (unenrolled)
 * idToken -- #606's MFA gate is a separate concern a caller opts into
 * via mfa.ts's signInEnrolled, since one spec (mfa-required.e2e.ts's
 * split-Owner case) deliberately needs this Owner to stay unenrolled.
 */
export async function seedFoundingOwner(
	request: APIRequestContext,
	fields: FoundingOwnerFields = {}
): Promise<SeededFoundingOwner> {
	const { practiceName = 'Riverside Doulas', staffName = 'Jamie Owner', workState = 'NY' } = fields;
	// Random suffix, not just Date.now(): millisecond-only uniqueness
	// collides across parallel Playwright workers -- confirmed as a real,
	// intermittent failure across this suite, not a theoretical one.
	const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	const email = `staff-${unique}@example.com`;

	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password: FOUNDING_OWNER_PASSWORD, returnSecureToken: true } }
	);
	expect(signUp.ok(), `owner signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken, localId } = await signUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${idToken}` },
		data: { practiceName, staffName, workState }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `staff signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId, staffId } = JSON.parse(signupBody);

	return { email, password: FOUNDING_OWNER_PASSWORD, idToken, localId, practiceId, staffId };
}

export interface SeededNoPracticeAccount {
	email: string;
	headers: { Cookie: string };
}

/**
 * Provisions an Identity Platform account and exchanges its ID token for
 * a session via auth.ts's signIn -- without ever calling
 * POST /api/staff/signup in between, unlike seedFoundingOwner above. The
 * resulting session is authenticated but resolves to no staff row: the
 * state `/no-practice` (#745) exists for, and which #749's accessibility
 * scan needs a fixture for, distinct from holding no session at all.
 */
export async function seedAccountWithNoPractice(request: APIRequestContext): Promise<SeededNoPracticeAccount> {
	// Same random-suffix reasoning as seedFoundingOwner above.
	const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	const email = `no-practice-${unique}@example.com`;

	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password: FOUNDING_OWNER_PASSWORD, returnSecureToken: true } }
	);
	expect(signUp.ok(), `no-practice account signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken } = await signUp.json();

	const headers = await signIn(request, API_URL, idToken);

	return { email, headers };
}
