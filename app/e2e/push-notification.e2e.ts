import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { seedClientPortalUser } from './stack';

// The Firebase Auth emulator and the Go BFF container -- see
// e2e/global-setup.ts and compose.e2e.yaml for how these get started.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

// #61's e2e AC: simulate a push event end to end (push received -> service
// worker fetches -> thread UI updates) without a real VAPID round-trip --
// this self-contained podman-compose stack has no route to the public
// internet's push services anyway. Rather than a CDP session, this
// dispatches a real `PushEvent` directly inside the registered service
// worker's own execution context via Playwright's Worker.evaluate --
// PushEvent is constructable per spec, so no CDP is needed. The service
// worker (src/service-worker.ts) reacts by posting a content-free message
// to every open window client, which is what the Client-portal thread
// page (src/routes/portal/engagements/[engagementId]/+page.svelte) uses
// to trigger its own authenticated refetch -- the service worker has no
// Identity Platform ID token, so it can never call the BFF itself
// (#lib/push.ts's PUSH_MESSAGE_TYPE doc comment).
test('a synthetic push event wakes the open thread tab and it refetches', async ({
	page,
	request,
	context
}) => {
	const staffEmail = `staff-${Date.now()}@example.com`;
	const clientEmail = `client-${Date.now()}@example.com`;
	const password = 'password123';

	// Provision a Practice + Staff (owner), a Client + Engagement at that
	// Practice, and a Client-portal login -- the same setup
	// client-portal-login.e2e.ts uses, since this test needs a real
	// Engagement thread to push a Message into.
	const staffSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: staffEmail, password, returnSecureToken: true } }
	);
	expect(staffSignUp.ok()).toBe(true);
	const { idToken: staffIdToken } = await staffSignUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${staffIdToken}` },
		data: { practiceName: 'Riverside Doulas', staffName: 'Jamie Owner', staffEmail }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `staff signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	const createClient = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
		headers: { Authorization: `Bearer ${staffIdToken}` },
		data: { name: 'Pat Client', email: clientEmail }
	});
	const createClientBody = await createClient.text();
	expect(createClient.ok(), `create client failed: ${createClient.status()} ${createClientBody}`).toBe(
		true
	);
	const { clientId, engagementId } = JSON.parse(createClientBody);

	const clientSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: clientEmail, password, returnSecureToken: true } }
	);
	expect(clientSignUp.ok()).toBe(true);
	const { localId: clientUID } = await clientSignUp.json();
	seedClientPortalUser(clientUID, clientId);

	await page.goto('/portal/login');
	await page.getByLabel('Email').fill(clientEmail);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
	await expect(page.getByText('No messages yet.')).toBeVisible();

	// Wait for the service worker to install and start controlling this
	// tab (see service-worker.ts's activate handler -- without
	// clients.claim() there, a tab opened before the worker activates is
	// never controlled, so it would never receive the push handler's
	// postMessage below).
	await page.evaluate(() => navigator.serviceWorker.ready);

	// A Staff member sends a Message through the real create-Message
	// endpoint while the Client's thread tab sits open, exactly as #58's
	// Staff-side flow would.
	const messageBody = `push-triggered message ${Date.now()}`;
	const createMessage = await request.post(
		`${API_URL}/api/practices/${practiceId}/engagements/${engagementId}/messages`,
		{ headers: { Authorization: `Bearer ${staffIdToken}` }, data: { body: messageBody } }
	);
	expect(createMessage.ok()).toBe(true);

	const sw = context.serviceWorkers()[0] ?? (await context.waitForEvent('serviceworker'));
	await sw.evaluate((payload) => {
		dispatchEvent(new PushEvent('push', { data: JSON.stringify(payload) }));
	}, { engagementId });

	// Proves delivery end to end: no manual reload, just the push ->
	// postMessage -> refetch chain updating the already-open thread.
	await expect(page.getByText(messageBody)).toBeVisible();
});
