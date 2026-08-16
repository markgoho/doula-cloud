/**
 * Shape of the content-free push payload the Go BFF sends on Message
 * creation (api/internal/message/push.go's pushPayload) -- per #56's
 * no-PHI-to-non-BAA-vendor rule, this never carries sender name or
 * message text, only enough to route a notification tap to the right
 * thread. PracticeID is present only for a Staff recipient (the
 * Practice-scoped thread route); absent for a Client recipient (the
 * Client-portal route needs only the Engagement id).
 */
export type PushPayload = {
	engagementId: string;
	practiceId?: string;
};

/**
 * Parses a push event's JSON payload, rejecting anything that isn't a
 * well-formed PushPayload rather than letting a malformed or unexpected
 * payload propagate into notificationFor/threadURLFor.
 */
export function parsePushPayload(raw: string): PushPayload | null {
	let data: unknown;
	try {
		data = JSON.parse(raw);
	} catch {
		return null;
	}
	if (typeof data !== 'object' || data === null) return null;
	const { engagementId, practiceId } = data as Record<string, unknown>;
	if (typeof engagementId !== 'string' || engagementId === '') return null;
	if (practiceId !== undefined && (typeof practiceId !== 'string' || practiceId === '')) return null;
	return practiceId ? { engagementId, practiceId } : { engagementId };
}

/**
 * Builds the system notification to show for payload. Content-free per
 * #56: no sender name, no message text -- the tap itself is what fetches
 * the real content.
 */
export function notificationFor(payload: PushPayload): { title: string; options: NotificationOptions } {
	return {
		title: 'New message',
		options: {
			body: 'You have a new message.',
			data: payload
		}
	};
}

/** Resolves the thread URL a notification tap should open. */
export function threadURLFor(payload: PushPayload): string {
	return payload.practiceId
		? `/practices/${payload.practiceId}/engagements/${payload.engagementId}`
		: `/portal/engagements/${payload.engagementId}`;
}

/**
 * The postMessage type the service worker sends to every open window
 * client on a push -- distinct from a browser-shown Notification. The
 * service worker itself has no Identity Platform ID token, so it can
 * never call the BFF directly (per ADR-0002, only the page can do an
 * authenticated fetch); this message is how it wakes an *already-open*
 * page to refetch. A closed/backgrounded app instead relies on the
 * Notification's tap (threadURLFor) to open/focus a page, which then
 * fetches on its own mount -- the same "push wakes the client, which
 * fetches the real content" outcome, reached through whichever of the two
 * paths applies.
 */
export const PUSH_MESSAGE_TYPE = 'doula-cloud/push-message';

export type PushMessage = { type: typeof PUSH_MESSAGE_TYPE; payload: PushPayload };

/** Builds the message the service worker posts to open window clients. */
export function pushMessageFor(payload: PushPayload): PushMessage {
	return { type: PUSH_MESSAGE_TYPE, payload };
}

/**
 * Narrows an incoming `message` event's data to a PushMessage carrying
 * engagementId, or null if it isn't one -- used by a page to tell a
 * same-origin `message` event apart from this one before deciding to
 * refetch.
 */
export function asPushMessage(data: unknown): PushMessage | null {
	if (typeof data !== 'object' || data === null) return null;
	const { type, payload } = data as Record<string, unknown>;
	if (type !== PUSH_MESSAGE_TYPE) return null;
	if (typeof payload !== 'object' || payload === null) return null;
	const { engagementId } = payload as Record<string, unknown>;
	if (typeof engagementId !== 'string' || engagementId === '') return null;
	return { type: PUSH_MESSAGE_TYPE, payload: payload as PushPayload };
}
