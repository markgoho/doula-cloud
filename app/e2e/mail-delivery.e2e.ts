import { expect, test } from '@playwright/test';
import { MAILBOX_DOMAIN, MAILBOX_URL } from './stack';
import { drainOutbox, drainUntilMailArrives, readMailbox } from './outboxMail';
import { signInEnrolled, enterPracticeAsEnrolled } from './mfa';
import { seedFoundingOwner, uniqueEmail } from './staffSignup';

const STAFF_INVITE_OUTBOX = 'process-staff-invite-outbox';
const INVITE_SUBJECT = "You've been invited to join a practice on Doula Cloud";

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
	const doulaEmail = uniqueEmail('doula', MAILBOX_DOMAIN);
	const complainerEmail = uniqueEmail('complainer', MAILBOX_DOMAIN);
	const password = 'password123';

	// Fixture setup, not the seam under test (#207).
	const { idToken: ownerIdToken, localId: ownerUID, practiceId } = await seedFoundingOwner(request, {
		practiceName: 'Rooted Birth Collective',
		staffName: 'Renata Vela'
	});

	const ownerHeaders = await signInEnrolled(request, ownerIdToken, ownerUID);
	await enterPracticeAsEnrolled(context, page, ownerHeaders, practiceId);

	await page.goto(`/practices/${practiceId}/invite`);
	await page.getByLabel('Their email').fill(doulaEmail);
	await page.getByRole('button', { name: 'Send invite' }).click();
	await expect(page.getByText(`is on its way to ${doulaEmail}`)).toBeVisible();

	// Nothing fires by itself locally (#762): this POSTs the one
	// `process-*` endpoint under test, which deployed is reached by
	// ADR-0013's nudge and by `process-outbox-drain` (#481).
	//
	// It drains the whole table, not just this spec's row (staffinvite's
	// claimQuery filters on status and next_attempt_at and nothing else),
	// and Playwright runs spec files in parallel against one shared stack
	// -- so this call mails, and clears the token from, whatever other
	// spec's invitation happens to be pending at this instant. That is
	// deliberate here and handled there: stack.ts's readStaffInviteToken
	// falls back to this same mailbox when its outbox row has gone (#827).
	//
	// It runs the other way too, which is why this is a wait rather than
	// one call (#1141): another spec's drain can be holding *this* row
	// locked, and outboxMail.ts writes out what that costs a single read.
	//
	// The harness's read: JSON, for assertions. Never an observed act.
	const message = await drainUntilMailArrives(
		request,
		STAFF_INVITE_OUTBOX,
		doulaEmail,
		INVITE_SUBJECT
	);
	expect(message.from).toBe(`Doula Cloud <notifications@${MAILBOX_DOMAIN}>`);

	// The persona's read: the inbox, in a browser, clicking the link out
	// of the message body rather than lifting a token from a table.
	await page.goto(`${MAILBOX_URL}/inbox/${encodeURIComponent(doulaEmail)}`);
	await expect(page.getByRole('link', { name: INVITE_SUBJECT })).toBeVisible();
	await page.getByRole('link', { name: INVITE_SUBJECT }).click();
	await page.getByRole('link', { name: /\/accept-invite\?token=/ }).click();

	await expect(page.getByLabel('Email')).toBeVisible();
	await page.getByLabel('Email').fill(doulaEmail);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect(page.getByRole('heading', { name: 'Tell us about yourself' })).toBeVisible();

	// Mailgun's event side. A complaint for an address suppresses it
	// account-wide (ADR-0029), so an invitation to it after the complaint
	// is refused at the endpoint (#861) and no mail is ever queued. The
	// send-time guard in mailsuppress.Sender still stands behind that for
	// an address suppressed after its mail was already queued.
	const complaint = await request.post(`${MAILBOX_URL}/api/delivery-event`, {
		data: { to: complainerEmail, event: 'complained', reason: 'abuse' }
	});
	const complaintResult = await complaint.json();
	expect(complaintResult.status, 'the BFF rejected the signed Mailgun webhook').toBe(200);

	await enterPracticeAsEnrolled(context, page, ownerHeaders, practiceId);
	await page.goto(`/practices/${practiceId}/invite`);
	await page.getByLabel('Their email').fill(complainerEmail);
	await page.getByRole('button', { name: 'Send invite' }).click();
	// #861: the invitation is refused at the endpoint now, before any row
	// is written, so the Owner is told at the field rather than reading a
	// success message for mail that will only ever be dead-lettered.
	await expect(
		page.getByText('This email address is blocked. Blocked email addresses shows why and what can be done.').first()
	).toBeVisible();
	// One drain, not a wait: this claim is that nothing was ever queued
	// for that address, so there is no message to wait for and any wait
	// would only be a wait for the deadline.
	await drainOutbox(request, STAFF_INVITE_OUTBOX);

	const suppressed = await readMailbox(request, complainerEmail);
	expect(suppressed, 'a suppressed address still received mail').toEqual([]);
});
