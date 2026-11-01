import { expect, test, type APIRequestContext } from '../../app/e2e/playwrightTest';
import { EMULATOR_URL, enrollPhoneFactor, randomPhoneNumber } from '../../app/e2e/mfa';
import { API_URL, OPS_URL } from '../../app/e2e/ports';
import { OPS_TENANT_ID, SEEDED_OPERATOR } from '../../app/e2e/stack';

/*
 * #2469: the vendored Auth emulator signs in to a tenant, which local ops
 * stands on (ADR-0062). What it showed, so nobody derives it again:
 *
 * - A tenant written to by an id it does not yet have is made with that
 *   id, so the stack fixes the id (stack.ts's OPS_TENANT_ID).
 * - A tenant made over REST starts with MFA off, and phone enrollment in
 *   it is refused until its mfaConfig names PHONE_SMS. The emulator has
 *   no TOTP, in a tenant as outside one.
 * - mfaConfig MANDATORY does not refuse an unenrolled sign-in: the token
 *   is issued with no second factor. Refusing it is doula-ops's job.
 * - A tenant's verification codes are listed under the tenant only.
 * - A tenant login cannot sign in without the tenant id.
 */

type Tokens = { idToken: string };

function claimsOf(idToken: string): { firebase: Record<string, unknown> } {
	return JSON.parse(Buffer.from(idToken.split('.', 3)[1], 'base64url').toString('utf8'));
}

async function signInWithPassword(
	request: APIRequestContext,
	body: Record<string, unknown>
): Promise<Tokens & { error?: { message: string } }> {
	const response = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=fake-key`,
		{ data: { returnSecureToken: true, ...body } }
	);
	return await response.json();
}

test('The seeded operator login signs in to the operator tenant, and the token names the tenant', async ({
	request
}) => {
	const signedIn = await signInWithPassword(request, {
		email: SEEDED_OPERATOR.email,
		password: SEEDED_OPERATOR.password,
		tenantId: OPS_TENANT_ID
	});

	const { firebase } = claimsOf(signedIn.idToken);
	expect(firebase.tenant).toBe(OPS_TENANT_ID);
	expect(firebase.sign_in_second_factor).toBeUndefined();
});

test('An operator login cannot sign in outside its tenant', async ({ request }) => {
	const signedIn = await signInWithPassword(request, {
		email: SEEDED_OPERATOR.email,
		password: SEEDED_OPERATOR.password
	});

	expect(signedIn.error?.message).toBe('EMAIL_NOT_FOUND');
});

async function newOperatorLogin(request: APIRequestContext): Promise<Tokens> {
	const email = `operator-${crypto.randomUUID()}@doula-cloud.invalid`;
	const created = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/projects/doula-cloud/tenants/${OPS_TENANT_ID}/accounts`,
		{
			headers: { Authorization: 'Bearer owner' },
			data: { email, password: 'operator-spec-password', displayName: 'Spec Operator', emailVerified: true }
		}
	);
	expect(created.ok()).toBe(true);
	const signedIn = await signInWithPassword(request, {
		email,
		password: 'operator-spec-password',
		tenantId: OPS_TENANT_ID
	});
	return signedIn;
}

function exchange(request: APIRequestContext, idToken: string) {
	return request.post(`${OPS_URL}/api/session`, { headers: { Authorization: `Bearer ${idToken}` } });
}

test('A login in the operator tenant enrolls the emulator factor, and its token carries both', async ({
	request
}) => {
	const signedIn = await newOperatorLogin(request);

	const enrolled = await enrollPhoneFactor(request, signedIn.idToken, randomPhoneNumber(), OPS_TENANT_ID);

	const { firebase } = claimsOf(enrolled.idToken);
	expect(firebase.tenant).toBe(OPS_TENANT_ID);
	expect(firebase.sign_in_second_factor).toBe('phone');
});

test('doula-ops mints a session from an operator token that passed the emulator factor', async ({ request }) => {
	const signedIn = await newOperatorLogin(request);
	const enrolled = await enrollPhoneFactor(request, signedIn.idToken, randomPhoneNumber(), OPS_TENANT_ID);

	const response = await exchange(request, enrolled.idToken);

	expect(response.status()).toBe(200);
	expect(await response.json()).toEqual({ name: 'Spec Operator', email: expect.stringContaining('@doula-cloud.invalid') });
	// The cookie is Secure, which an API request context over http keeps
	// to itself, so it is sent back by hand.
	const cookie = (response.headers()['set-cookie'] ?? '').split(';', 1)[0];
	const session = await request.get(`${OPS_URL}/api/session`, { headers: { Cookie: cookie } });
	expect(session.status()).toBe(200);
});

test('doula-ops sends an operator token with no second factor to enroll', async ({ request }) => {
	const signedIn = await newOperatorLogin(request);

	const response = await exchange(request, signedIn.idToken);

	expect(response.status()).toBe(403);
	const refusal = await response.json();
	expect(refusal.code).toBe('MFA_REQUIRED');
});

test('doula-ops refuses a Staff token, second factor and all', async ({ request }) => {
	const email = `staff-${crypto.randomUUID()}@doula-cloud.invalid`;
	const signUp = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`, {
		data: { email, password: 'staff-spec-password', returnSecureToken: true }
	});
	const { idToken, localId } = await signUp.json();
	await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:update`, {
		headers: { Authorization: 'Bearer owner' },
		data: { localId, emailVerified: true }
	});
	const enrolled = await enrollPhoneFactor(request, idToken);

	const response = await exchange(request, enrolled.idToken);

	expect(response.status()).toBe(401);
	const refusal = await response.json();
	expect(refusal.code).toBe('UNAUTHORIZED');
});

test('#2470: doula-api refuses an operator token, second factor and all', async ({ request }) => {
	const signedIn = await newOperatorLogin(request);
	const enrolled = await enrollPhoneFactor(request, signedIn.idToken, randomPhoneNumber(), OPS_TENANT_ID);

	const response = await request.post(`${API_URL}/api/session`, {
		headers: { Authorization: `Bearer ${enrolled.idToken}` }
	});

	// The emulator refuses this at verify, before the tenant check runs, so
	// this proves the outcome; api/'s operatortoken specs prove the check.
	expect(response.status()).toBe(401);
	const refusal = await response.json();
	expect(refusal.code).toBe('UNAUTHORIZED');
	expect(response.headers()['set-cookie']).toBeUndefined();
});

async function operatorSession(request: APIRequestContext): Promise<{ idToken: string; cookie: string }> {
	const signedIn = await newOperatorLogin(request);
	const enrolled = await enrollPhoneFactor(request, signedIn.idToken, randomPhoneNumber(), OPS_TENANT_ID);
	const response = await exchange(request, enrolled.idToken);
	expect(response.status()).toBe(200);
	return { idToken: enrolled.idToken, cookie: (response.headers()['set-cookie'] ?? '').split(';', 1)[0] };
}

test("#2471: doula-ops steps a session up on a sign-in just now by the session's own operator", async ({
	request
}) => {
	const { idToken, cookie } = await operatorSession(request);

	const response = await request.post(`${OPS_URL}/api/session/step-up`, {
		headers: { Cookie: cookie, Authorization: `Bearer ${idToken}` }
	});

	expect(response.status()).toBe(204);
});

test('#2471: doula-ops refuses a step-up with no fresh sign-in, or with another operator', async ({ request }) => {
	const mine = await operatorSession(request);
	const theirs = await operatorSession(request);

	const none = await request.post(`${OPS_URL}/api/session/step-up`, { headers: { Cookie: mine.cookie } });
	const other = await request.post(`${OPS_URL}/api/session/step-up`, {
		headers: { Cookie: mine.cookie, Authorization: `Bearer ${theirs.idToken}` }
	});

	expect(none.status()).toBe(401);
	const refusal = await none.json();
	expect(refusal.code).toBe('UNAUTHORIZED');
	expect(other.status()).toBe(401);
});
