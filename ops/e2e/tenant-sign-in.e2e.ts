import { expect, test, type APIRequestContext } from '../../app/e2e/playwrightTest';
import { EMULATOR_URL, enrollPhoneFactor, randomPhoneNumber } from '../../app/e2e/mfa';
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

test('A login in the operator tenant enrolls the emulator factor, and its token carries both', async ({
	request
}) => {
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

	const enrolled = await enrollPhoneFactor(request, signedIn.idToken, randomPhoneNumber(), OPS_TENANT_ID);

	const { firebase } = claimsOf(enrolled.idToken);
	expect(firebase.tenant).toBe(OPS_TENANT_ID);
	expect(firebase.sign_in_second_factor).toBe('phone');
});
