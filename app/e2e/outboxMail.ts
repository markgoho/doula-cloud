import { expect, type APIRequestContext } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT } from './ports';
import { MAILBOX_URL, WORKER_SECRET } from './stack';
// One message as the sandbox mailbox serves it back over
// `/api/messages`: the catcher's own row shape, not a second copy of it
// that nothing keeps in step. Type-only, so a spec importing this file
// never loads mailbox.ts's `Bun.serve` half.
import type { Captured as MailboxMessage } from './mailbox';

const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

/**
 * Reads one address's sandbox mailbox (e2e/mailbox.ts) as JSON.
 */
export async function readMailbox(
	request: APIRequestContext,
	address: string
): Promise<MailboxMessage[]> {
	const inbox = await request.get(`${MAILBOX_URL}/api/messages?to=${encodeURIComponent(address)}`);
	expect(inbox.ok(), `reading ${address}'s sandbox mailbox failed: ${inbox.status()}`).toBe(true);
	return (await inbox.json()) as MailboxMessage[];
}

/**
 * POSTs one `process-*` outbox endpoint once. Nothing fires by itself
 * locally (#762): deployed these are reached by ADR-0013's nudge and by
 * `process-outbox-drain` (#481).
 */
export async function drainOutbox(request: APIRequestContext, outbox: string): Promise<void> {
	const drained = await request.post(`${API_URL}/api/internal/notifications/${outbox}`, {
		headers: { 'X-Internal-Secret': WORKER_SECRET }
	});
	expect(drained.ok(), `draining ${outbox} failed: ${drained.status()}`).toBe(true);
}

// How long a spec waits for its own message, and how often it looks
// again. A third of Playwright's 30s per-test default (this repo's
// playwright.config.ts sets no timeout of its own), because the wait is
// never the first thing a spec does: the walk that queues the mail comes
// first, and a budget near the whole test's would be spent past the
// test's own deadline, reporting as a bare timeout instead of the
// sentence below -- which names the address and the outbox. It is not a
// failure this can *cause*: mail that never arrives fails the spec at
// whichever deadline comes first. Delivery here is sub-second and a
// neighbor's transaction is milliseconds, so ten seconds is already
// orders of magnitude of headroom.
const DELIVERY_TIMEOUT_MS = 10_000;
const DELIVERY_POLL_MS = 250;

/**
 * Drains `outbox` until the message this caller is about has arrived at
 * `address`, and returns it (#1141).
 *
 * ## Why one drain and one read cannot be trusted
 *
 * Every `process-*` endpoint runs its whole claim-compose-send-mark loop
 * inside one transaction (outbox.ProcessPending), and every claim query
 * takes `LIMIT $1 FOR UPDATE SKIP LOCKED` over rows filtered on status
 * and `next_attempt_at` and nothing else -- so a drain is table-wide and
 * holds each row it claims until it commits.
 *
 * Playwright runs spec files in parallel against one shared stack, and a
 * repeated batch (`--repeat-each`, which is how every flake on this
 * tracker has been confirmed fixed) puts several copies of one spec in
 * flight at once. Two copies A and B each queue a row; B's drain claims
 * *both* and locks them; A's drain then skips its own row, because it is
 * locked, and answers 200 having sent nothing. A reads the mailbox
 * before B's loop has reached A's row, finds it empty, and fails --
 * blaming the mail path for a harness race. Widening the batch limit
 * cannot help: the rows are locked, not unseen.
 *
 * Re-draining rather than only re-reading, because a skipped row is not
 * always somebody else's to finish: if any row in B's batch errors,
 * `runOutbox` rolls the whole transaction back and A's row is pending
 * and unclaimed again with no drain left in flight to take it.
 *
 * A subject rather than "the inbox is no longer empty", because an
 * address usually holds mail this caller is not waiting for -- a Staff
 * invitation to the same person, an earlier verification -- and a
 * message already sitting there would end the wait before the message
 * under test had been sent at all. A caller that requested two of the
 * same kind of mail for one address would need more than a subject to
 * tell them apart; no spec does that, so this stays a string rather than
 * a predicate until one does.
 *
 * ## The one drain that does not come through here
 *
 * simulation/clock.ts's `jump` POSTs `/api/internal/outboxes/drain`,
 * which runs every registered outbox and so carries exactly this shape.
 * It is left alone because nothing is exposed to it: the runs that call
 * it assert that the clock moved, and none of them reads a mailbox in
 * the same breath. A simulation act that starts doing so belongs here.
 */
export async function drainUntilMailArrives(
	request: APIRequestContext,
	outbox: string,
	address: string,
	subject: string
): Promise<MailboxMessage> {
	const deadline = Date.now() + DELIVERY_TIMEOUT_MS;
	for (;;) {
		await drainOutbox(request, outbox);
		const inbox = await readMailbox(request, address);
		const found = inbox.find((message) => message.subject === subject);
		if (found) {
			return found;
		}
		if (Date.now() >= deadline) {
			throw new Error(
				`e2e: no mail titled ${JSON.stringify(subject)} reached ${address} within ${DELIVERY_TIMEOUT_MS}ms of draining ${outbox} repeatedly`
			);
		}
		await new Promise((resolve) => setTimeout(resolve, DELIVERY_POLL_MS));
	}
}
