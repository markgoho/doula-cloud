import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';
import { MAILBOX_DOMAIN, MAILBOX_URL, WORKER_SECRET } from './stack';
import { signInEnrolled, enterPracticeAsEnrolled } from './mfa';

const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

// The one spec that walks mail as mail (#764, map #759). Every other
// spec that needs an invite token reads it off the pending outbox row
// (stack.ts's readStaffInviteToken) -- fine for proving a code path, and
// useless to a simulation run, which has to observe the act a person
// actually performs: opening the message and clicking what is in it.
//
// So this walks the whole path end to end: send the invitation through
// the screen, drain the outbox the way Cloud Scheduler would, and then
// arrive at the sandbox mailbox (e2e/mailbox.ts) as a browser, read the
// subject line, and follow the link out of the message body. It also
// exercises Mailgun's event side, which no Mailgun CLI forwards to
// localhost: a spam complaint writes an email_suppressions row, after
// which mailsuppress.Sender refuses the next send to that address.
test('An invitation arrives as readable mail, and a complaint stops the next one', async ({
	page,
	request,
	context
}) => {
	const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	const ownerEmail = `owner-${unique}@${MAILBOX_DOMAIN}`;
	const doulaEmail = `doula-${unique}@${MAILBOX_DOMAIN}`;
	const complainerEmail = `complainer-${unique}@${MAILBOX_DOMAIN}`;
	const password = 'password123';

	// Fixture setup, not the seam under test (#207).
	const ownerSignUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email: ownerEmail, password, returnSecureToken: true } }
	);
	expect(ownerSignUp.ok(), `ownerSignUp failed: ${ownerSignUp.status()}`).toBe(true);
	const { idToken: ownerIdToken, localId: ownerUID } = await ownerSignUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${ownerIdToken}` },
		data: { practiceName: 'Rooted Birth Collective', staffName: 'Renata Vela', staffEmail: ownerEmail, workState: 'NY' }
	});
	expect(signup.ok(), `signup failed: ${signup.status()} ${await signup.text()}`).toBe(true);
	const { practiceId } = await signup.json();

	const ownerHeaders = await signInEnrolled(request, ownerIdToken, ownerUID);
	await enterPracticeAsEnrolled(context, page, ownerHeaders, practiceId);

	await page.goto(`/practices/${practiceId}/invite`);
	await page.getByLabel('Their email').fill(doulaEmail);
	await page.getByRole('button', { name: 'Send invite' }).click();
	await expect(page.getByText(`is on its way to ${doulaEmail}`)).toBeVisible();

	// Nothing fires by itself locally (#762): the harness POSTs the
	// `process-*` endpoint Cloud Scheduler would have called.
	const drain = async () =>
		request.post(`${API_URL}/api/internal/notifications/process-staff-invite-outbox`, {
			headers: { 'X-Internal-Secret': WORKER_SECRET }
		});
	const drained = await drain();
	expect(drained.ok(), 'draining the staff-invite outbox failed').toBe(true);

	// The harness's read: JSON, for assertions. Never an observed act.
	const inbox = await request.get(`${MAILBOX_URL}/api/messages?to=${encodeURIComponent(doulaEmail)}`);
	const [message] = await inbox.json();
	expect(message, `no mail reached ${doulaEmail}`).toBeTruthy();
	expect(message.subject).toBe("You've been invited to join a practice on Doula Cloud");
	expect(message.from).toBe(`Doula Cloud <notifications@${MAILBOX_DOMAIN}>`);

	// The persona's read: the inbox, in a browser, clicking the link out
	// of the message body rather than lifting a token from a table.
	await page.goto(`${MAILBOX_URL}/inbox/${encodeURIComponent(doulaEmail)}`);
	await expect(page.getByRole('link', { name: "You've been invited to join a practice on Doula Cloud" })).toBeVisible();
	await page.getByRole('link', { name: "You've been invited to join a practice on Doula Cloud" }).click();
	await page.getByRole('link', { name: /\/accept-invite\?token=/ }).click();

	await expect(page.getByLabel('Email')).toBeVisible();
	await page.getByLabel('Email').fill(doulaEmail);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect(page.getByRole('heading', { name: 'Tell us about yourself' })).toBeVisible();

	// Mailgun's event side. A complaint for an address suppresses it
	// account-wide (ADR-0029), so the invitation queued for it after the
	// complaint is dead-lettered by mailsuppress.Sender and never
	// reaches the mailbox at all.
	const complaint = await request.post(`${MAILBOX_URL}/api/delivery-event`, {
		data: { to: complainerEmail, event: 'complained', reason: 'abuse' }
	});
	const complaintResult = await complaint.json();
	expect(complaintResult.status, 'the BFF rejected the signed Mailgun webhook').toBe(200);

	await enterPracticeAsEnrolled(context, page, ownerHeaders, practiceId);
	await page.goto(`/practices/${practiceId}/invite`);
	await page.getByLabel('Their email').fill(complainerEmail);
	await page.getByRole('button', { name: 'Send invite' }).click();
	await expect(page.getByText(`is on its way to ${complainerEmail}`)).toBeVisible();
	const drainedAgain = await drain();
	expect(drainedAgain.ok(), 'draining after the complaint failed').toBe(true);

	const suppressed = await request.get(`${MAILBOX_URL}/api/messages?to=${encodeURIComponent(complainerEmail)}`);
	expect(await suppressed.json(), 'a suppressed address still received mail').toEqual([]);
});
