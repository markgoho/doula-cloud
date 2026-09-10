import { expect, type APIRequestContext } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT } from './ports';
import { MAILBOX_URL, WORKER_SECRET } from './stack';

const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

/**
 * One message as e2e/mailbox.ts serves it back over `/api/messages`.
 */
export interface MailboxMessage {
	id: string;
	seq: number;
	to: string;
	from: string;
	replyTo: string;
	subject: string;
	text: string;
	label: string;
}

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
// again. Comfortably under Playwright's own per-test timeout, so a
// genuine failure to deliver reports as the sentence below -- naming the
// address and the outbox -- rather than as a bare test timeout that says
// only that something took too long.
const DELIVERY_TIMEOUT_MS = 20_000;
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
 * `isWanted` rather than "the inbox is no longer empty", because an
 * address usually holds mail this caller is not waiting for -- a Staff
 * invitation to the same person, an earlier verification -- and a
 * message already sitting there would end the wait before the message
 * under test had been sent at all. A caller that requests two of the
 * same kind of mail for one address would need more than a subject to
 * tell them apart; no spec does that today.
 */
export async function drainUntilMailArrives(
	request: APIRequestContext,
	outbox: string,
	address: string,
	isWanted: (message: MailboxMessage) => boolean
): Promise<MailboxMessage> {
	const deadline = Date.now() + DELIVERY_TIMEOUT_MS;
	for (;;) {
		await drainOutbox(request, outbox);
		const inbox = await readMailbox(request, address);
		const found = inbox.find((message) => isWanted(message));
		if (found) {
			return found;
		}
		if (Date.now() >= deadline) {
			throw new Error(
				`e2e: no matching mail reached ${address} within ${DELIVERY_TIMEOUT_MS}ms of draining ${outbox} repeatedly`
			);
		}
		await new Promise((resolve) => setTimeout(resolve, DELIVERY_POLL_MS));
	}
}

/**
 * Matches a message by its exact subject line.
 */
export function withSubject(subject: string): (message: MailboxMessage) => boolean {
	return (message) => message.subject === subject;
}
