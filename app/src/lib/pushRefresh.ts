import { asPushMessage } from '#lib/push.js';

/**
 * Wires a `navigator.serviceWorker` "message" listener that calls
 * onMatch() whenever an open-client push message (push.ts's
 * PUSH_MESSAGE_TYPE) arrives for engagementId -- the "already-open tab"
 * half of #61's "push wakes the client, which fetches the real content"
 * delivery; a closed/backgrounded tab instead relies on the
 * Notification's tap (service-worker.ts's notificationclick handler).
 * Shared by both thread pages (Staff and Client-portal) rather than each
 * defining its own identical listener. Returns a cleanup function callers
 * must invoke in onDestroy.
 */
/* v8 ignore start -- wires the real navigator.serviceWorker API; the payload-matching logic it delegates to (asPushMessage) is unit-tested in push.spec.ts */
export function subscribeToThreadPushMessages(engagementId: string, onMatch: () => void): () => void {
	if (!('serviceWorker' in navigator)) return () => {};

	const handler = (event: MessageEvent) => {
		const message = asPushMessage(event.data);
		if (message?.payload.engagementId === engagementId) onMatch();
	};
	navigator.serviceWorker.addEventListener('message', handler);
	return () => navigator.serviceWorker.removeEventListener('message', handler);
}
/* v8 ignore stop */
