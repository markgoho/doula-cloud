import { expect, type APIRequestContext } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { signIn, sessionCookieFrom } from './auth';
import { seedClientPortalUser, seedEngagement, readStaffInviteToken } from './stack';

// The Firebase Auth emulator and the Go BFF -- both host processes -- see
// e2e/global-setup.ts and e2e/stack.ts for how these get started.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

/**
 * The password every seeded account here is given -- these are emulator
 * fixtures, so it only has to satisfy Identity Platform's length rule.
 */
export const PORTAL_CLIENT_PASSWORD = 'password123';

export interface SeededPortalClient {
	practiceId: string;
	staffId: string;
	clientId: string;
	engagementId: string;
	staffEmail: string;
	staffIdToken: string;
	staffHeaders: { Cookie: string };
	clientEmail: string;
}

/** The fields a caller states about a seeded Client. Only the names,
 * because ADR-0017's match query is what these fixtures exist to feed and
 * the name columns are the only part of it a spec ever sets deliberately
 * -- everything else on the record is left at whatever the create
 * endpoint defaults to. */
export interface NewClientFields {
	givenName: string;
	familyName?: string;
}

/**
 * Provisions a plain Client at an existing Practice -- no portal account,
 * no Identity Platform user, nothing but the record itself. The sibling
 * of `seedPortalClient` for the specs that only need a Client to exist:
 * the Client detail hub, her edit screen, and the second record a
 * match-override prompt needs something to match against.
 *
 * Takes the Staff cookie header rather than minting its own session, so a
 * caller that already signed in does not pay for a second signup.
 *
 * The email is generated rather than asked for, and generated *unique*:
 * FindMatches matches an email exactly, so a fixture that shared one with
 * another Client would collide on the email as well as on the name, and a
 * spec about a name collision would no longer be testing what it says.
 */
export async function seedClient(
	request: APIRequestContext,
	practiceId: string,
	staffHeaders: { Cookie: string },
	fields: NewClientFields
): Promise<string> {
	const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	const created = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
		headers: staffHeaders,
		data: { ...fields, email: `client-${unique}@example.com` }
	});
	const body = await created.text();
	expect(created.ok(), `seedClient failed: ${created.status()} ${body}`).toBe(true);
	const { id } = JSON.parse(body);
	return id;
}

export interface SeededContractorDoula {
	staffId: string;
	email: string;
	headers: { Cookie: string };
}

/**
 * Provisions a second Staff member at an existing Practice, holding the
 * `doula` role and a contractor employment type -- neither `owner` nor
 * `admin`. Drives the real invitation and acceptance endpoints rather
 * than seeding a Membership directly, so this Doula's Membership is
 * exactly what any real accepted invite produces (roles, employment
 * type, and the `joined` membership event `AcceptInviteHandler` records).
 * The one piece no API response ever carries is the invitation's
 * plaintext token (#316) -- `readStaffInviteToken` (stack.ts) reads it
 * off the pending `staff_invite_outbox` row instead, which is where it
 * sits, unmailed, for the whole run: nothing in the e2e stack runs the
 * outbox worker.
 *
 * Takes the Owner's own cookie header, the same way `seedClient` takes
 * one, so a caller that already has an Owner session does not pay for a
 * second signup. Reusable by any spec that needs a session behind
 * ADR-0008's contractor gate, not just the one route #525 was filed for.
 */
export async function seedContractorDoula(
	request: APIRequestContext,
	practiceId: string,
	ownerHeaders: { Cookie: string }
): Promise<SeededContractorDoula> {
	const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	const email = `contractor-${unique}@example.com`;

	const invite = await request.post(`${API_URL}/api/practices/${practiceId}/staff/invitations`, {
		headers: ownerHeaders,
		data: { email, roles: ['doula'], employmentType: 'contractor' }
	});
	const inviteBody = await invite.text();
	expect(invite.ok(), `contractor invite failed: ${invite.status()} ${inviteBody}`).toBe(true);
	const { invitationId } = JSON.parse(inviteBody);

	const inviteToken = readStaffInviteToken(invitationId);
	expect(inviteToken, `no pending staff_invite_outbox row for invitation ${invitationId}`).toBeTruthy();

	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password: PORTAL_CLIENT_PASSWORD, returnSecureToken: true } }
	);
	expect(
		signUp.ok(),
		`contractor doula signUp failed: ${signUp.status()} ${await signUp.text()}`
	).toBe(true);
	const { idToken } = await signUp.json();

	// AcceptInviteHandler runs before any session exists (a bootstrap
	// Bearer token, not the __session cookie) and mints the session on
	// its own response -- the same shape staff/signup uses.
	const accept = await request.post(`${API_URL}/api/staff/accept-invite`, {
		headers: { Authorization: `Bearer ${idToken}` },
		data: { inviteToken, name: 'Casey Contractor', workState: 'NY' }
	});
	const acceptBody = await accept.text();
	expect(accept.ok(), `contractor accept-invite failed: ${accept.status()} ${acceptBody}`).toBe(true);
	const { staffId } = JSON.parse(acceptBody);

	const headers = sessionCookieFrom(accept, 'accept-invite');

	return { staffId, email, headers };
}

/**
 * Provisions everything a Client needs to log in to their portal: a
 * Practice with a Staff owner, a Client + Engagement at that Practice,
 * and an Identity Platform account linked to that Client.
 *
 * Done through the API and stack.ts's client_portal_users fixture rather
 * than through the UI, so that a spec about some *later* portal behaviour
 * is not also re-proving signup, Client creation and portal provisioning.
 * The Staff session and IDs come back too, for specs that go on to act as
 * the doula.
 */
export async function seedPortalClient(
	request: APIRequestContext,
	practiceName: string
): Promise<SeededPortalClient> {
	// The random suffix (not just Date.now(), millisecond-resolution)
	// avoids EMAIL_EXISTS collisions with other *.e2e.ts files' emails when
	// Playwright's parallel workers start within the same millisecond.
	const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	const staffEmail = `staff-${unique}@example.com`;
	const clientEmail = `client-${unique}@example.com`;

	const staffSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: staffEmail, password: PORTAL_CLIENT_PASSWORD, returnSecureToken: true } }
	);
	expect(staffSignUp.ok(), `staffSignUp failed: ${staffSignUp.status()} ${await staffSignUp.text()}`).toBe(true);
	const { idToken: staffIdToken } = await staffSignUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${staffIdToken}` },
		data: { practiceName, staffName: 'Alex Owner', staffEmail , workState: 'NY' }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `staff signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId, staffId } = JSON.parse(signupBody);

	// Everything after signup is cookie-authenticated (#151).
	const staffHeaders = await signIn(request, API_URL, staffIdToken);

	const createClient = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
		headers: staffHeaders,
		data: { givenName: 'Pat', familyName: 'Client', email: clientEmail }
	});
	const createClientBody = await createClient.text();
	expect(createClient.ok(), `create client failed: ${createClient.status()} ${createClientBody}`).toBe(true);
	const { id: clientId } = JSON.parse(createClientBody);
	const engagementId = seedEngagement(clientId, practiceId);

	// A separate Identity Platform account for the Client-portal login --
	// distinct from the Staff account above -- linked to that Client via
	// client_portal_users (see stack.ts for why this is seeded directly
	// rather than through a BFF endpoint).
	const clientSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: clientEmail, password: PORTAL_CLIENT_PASSWORD, returnSecureToken: true } }
	);
	expect(clientSignUp.ok(), `clientSignUp failed: ${clientSignUp.status()} ${await clientSignUp.text()}`).toBe(true);
	const { localId: clientUID } = await clientSignUp.json();
	seedClientPortalUser(clientUID, clientId);

	return {
		practiceId,
		staffId,
		clientId,
		engagementId,
		staffEmail,
		staffIdToken,
		staffHeaders,
		clientEmail
	};
}
