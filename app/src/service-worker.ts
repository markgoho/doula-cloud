/// <reference types="@sveltejs/kit" />
/// <reference no-default-lib="true"/>
/// <reference lib="esnext" />
/// <reference lib="webworker" />

// #61's minimal PWA layer: a service worker scoped only to push events --
// no offline caching, no asset precaching, per the ticket's explicit
// scope. SvelteKit auto-detects and registers this file (see
// https://svelte.dev/docs/kit/service-workers), so no manual
// navigator.serviceWorker.register call is needed anywhere in the app;
// only the push subscription itself (pushRegistration.ts) needs an
// explicit, per-page call after login.
//
// The payload-parsing/notification-building logic lives in #lib/push.ts,
// not here, so it's unit-testable via Vitest without a browser (per #61's
// AC) -- this file only wires that logic to the two ServiceWorker events.

import { notificationFor, parsePushPayload, pushMessageFor, threadURLFor } from '#lib/push.js';

const worker = /** @type {ServiceWorkerGlobalScope} */ (
	/** @type {unknown} */ (globalThis.self)
);

// A page open before this service worker activates is otherwise never
// "controlled" by it (the browser only starts controlling a client on
// its *next* navigation) -- without claiming it here, clients.matchAll()
// below would never see that tab, so notifyOpenClients silently reaches
// no one for the entire session that installed the worker.
worker.addEventListener('activate', (event: ExtendableEvent) => {
	event.waitUntil(worker.clients.claim());
});

// notifyOpenClients posts pushMessageFor(payload) to every open window
// client -- see push.ts's PUSH_MESSAGE_TYPE doc comment for why this,
// not a fetch from the service worker itself, is how an already-open
// page refetches on push.
async function notifyOpenClients(payload: Parameters<typeof pushMessageFor>[0]) {
	const clients = await worker.clients.matchAll({ type: 'window' });
	for (const client of clients) {
		client.postMessage(pushMessageFor(payload));
	}
}

worker.addEventListener('push', (event: PushEvent) => {
	if (!event.data) return;
	const payload = parsePushPayload(event.data.text());
	if (!payload) return;

	const { title, options } = notificationFor(payload);
	event.waitUntil(
		Promise.all([worker.registration.showNotification(title, options), notifyOpenClients(payload)])
	);
});

worker.addEventListener('notificationclick', (event: NotificationEvent) => {
	event.notification.close();
	// event.notification.data is exactly the PushPayload notificationFor
	// attached above -- no re-parsing needed, just a runtime guard against
	// a stale/malformed notification somehow surviving without one.
	const data = event.notification.data as { engagementId?: unknown; practiceId?: unknown } | undefined;
	if (typeof data?.engagementId !== 'string' || data.engagementId === '') return;
	const practiceId = typeof data.practiceId === 'string' && data.practiceId !== '' ? data.practiceId : undefined;

	event.waitUntil(worker.clients.openWindow(threadURLFor({ engagementId: data.engagementId, practiceId })));
});
