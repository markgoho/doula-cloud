import { expect, type APIRequestContext } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { signIn } from './auth';
import { seedClientPortalUser, seedEngagement } from './stack';

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
	clientId: string;
	engagementId: string;
	staffEmail: string;
	staffIdToken: string;
	staffHeaders: { Cookie: string };
	clientEmail: string;
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
	const { practiceId } = JSON.parse(signupBody);

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
		clientId,
		engagementId,
		staffEmail,
		staffIdToken,
		staffHeaders,
		clientEmail
	};
}
