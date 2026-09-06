/**
 * A path-and-init fetch function, already carrying whatever credential
 * the caller authenticates with -- mirrors billing.ts's Fetcher so this
 * module stays agnostic to Bearer-token vs. session-cookie callers.
 */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

/**
 * Converts a URL-safe base64 VAPID public key (the shape
 * webpush.GenerateVAPIDKeys and the api/internal/push package produce)
 * into the Uint8Array PushManager.subscribe's applicationServerKey
 * expects.
 */
export function urlBase64ToUint8Array(base64String: string): Uint8Array {
	const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
	const base64 = (base64String + padding).replaceAll('-', '+').replaceAll('_', '/');
	const rawData = atob(base64);
	return Uint8Array.from([...rawData].map((c) => c.codePointAt(0)!));
}

/**
 * The VAPID public key the BFF signs pushes with -- public by design
 * (the private half never leaves the server), so safe to inline at build
 * time the same way firebase.ts inlines the browser API key.
 */
export function vapidPublicKey(): string {
	return (import.meta.env.VITE_VAPID_PUBLIC_KEY as string | undefined) ?? '';
}

/**
 * Where this device's push subscription lives for a Practice -- the one
 * endpoint both Staff call sites use: the landing page registers against
 * it (#61) and the authenticated layout unregisters against it on
 * sign-out (#152).
 */
export function practicePushSubscriptionsPath(practiceId: string): string {
	return `/api/practices/${practiceId}/push-subscriptions`;
}

/**
 * The same endpoint for an Engagement, which is what the Client portal is
 * scoped by -- register on the thread screen (#61), unregister on
 * sign-out (#153).
 */
export function portalPushSubscriptionsPath(engagementId: string): string {
	return `/api/portal/engagements/${engagementId}/push-subscriptions`;
}

/**
 * Where an Engagement's durable push preference lives (#303) -- read with
 * GET, written with PUT, both behind clientauth.Middleware the same as
 * portalPushSubscriptionsPath.
 */
export function portalNotificationPreferencePath(engagementId: string): string {
	return `/api/portal/engagements/${engagementId}/notification-preference`;
}

/**
 * Registers this device's push subscription with subscribeURL, via the
 * already-registered service worker's PushManager -- "once per device
 * after login" per #61's AC; safe to call on every landing-page mount
 * since the BFF's register endpoint upserts by endpoint. Best-effort and
 * non-blocking: PushManager.subscribe() commonly fails or is unsupported
 * (no permission granted, headless/incognito browsers, no VAPID key
 * configured), and push notifications are an enhancement, not a
 * precondition for using the app -- any failure here is swallowed, never
 * thrown to the caller. The boolean return reports the outcome (#715) for
 * a caller that needs to react to it -- the Notifications settings
 * screen, which must not claim success when this resolves false;
 * registerPushSubscriptionIfEnabled's own silent-by-design callers stay
 * silent by simply not looking at it.
 */
/* v8 ignore start -- requires the Service Worker/PushManager browser APIs, exercised by Playwright e2e not Vitest */
// eslint-disable-next-line unicorn/consistent-boolean-name -- an action verb, not a predicate; renaming would break every established call site for a name no clearer than this one
export async function registerPushSubscription(subscribeURL: string, fetcher: Fetcher): Promise<boolean> {
	try {
		if (!('serviceWorker' in navigator) || !('PushManager' in globalThis)) return false;
		const publicKey = vapidPublicKey();
		if (!publicKey) return false;

		const registration = await navigator.serviceWorker.ready;
		const subscription =
			(await registration.pushManager.getSubscription()) ??
			(await registration.pushManager.subscribe({
				userVisibleOnly: true,
				// TS's DOM lib types applicationServerKey as BufferSource, which
				// (in this TS version) distinguishes ArrayBuffer- from
				// SharedArrayBuffer-backed views more strictly than
				// Uint8Array.from's inferred return type satisfies -- the
				// runtime value is a plain ArrayBuffer-backed Uint8Array either
				// way, so this cast is safe.
				applicationServerKey: urlBase64ToUint8Array(publicKey) as BufferSource
			}));

		const json = subscription.toJSON();
		const response = await fetcher(subscribeURL, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ endpoint: json.endpoint, keys: json.keys })
		});
		return response.ok;
	} catch {
		// Best-effort registration -- see the doc comment above.
		return false;
	}
}
/* v8 ignore stop */

/**
 * Whether this device can receive a push right now, independently of the
 * durable per-Engagement preference (#715): that preference carries no
 * device dimension, so a Client who has switched browsers, denied the
 * permission prompt, or revoked it since last granting it needs a
 * device-local answer instead. A mere read of `Notification.permission` --
 * never a subscribe attempt -- so calling this can never itself trigger
 * the browser's own permission prompt.
 */
export function isPushPermissionGranted(): boolean {
	return (
		'Notification' in globalThis &&
		'serviceWorker' in navigator &&
		'PushManager' in globalThis &&
		Notification.permission === 'granted'
	);
}

/**
 * Consults preferenceURL before ever registering for push (#303 AC1/AC4):
 * a Client who has never turned notifications on, or who has turned them
 * off, must never see the browser's permission prompt fire on a mere page
 * load -- only the notification-settings screen's own explicit action does
 * that. `register` is registerPushSubscription by default, injected so
 * this gate is provable without the Service Worker/PushManager browser
 * APIs that function itself needs (the same injection shape
 * signOutOfSession's `unregisterPush` parameter already uses).
 *
 * A failed or malformed preference read is treated the same as
 * "disabled": this call sits on every authenticated portal mount, and a
 * transient failure here must never turn into a silent, unexplained
 * permission prompt.
 */
export async function registerPushSubscriptionIfEnabled(
	preferenceURL: string,
	subscribeURL: string,
	fetcher: Fetcher,
	// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors registerPushSubscription's own name, its default value
	register: (subscribeURL: string, fetcher: Fetcher) => Promise<boolean> = registerPushSubscription
): Promise<void> {
	let isEnabled = false;
	try {
		const response = await fetcher(preferenceURL);
		if (response.ok) {
			const data: unknown = await response.json();
			isEnabled = typeof data === 'object' && data !== null && (data as { enabled?: unknown }).enabled === true;
		}
	} catch {
		// Treated as disabled -- see the doc comment above.
	}

	if (isEnabled) {
		await register(subscribeURL, fetcher);
	}
}

/**
 * Takes this device off push for unsubscribeURL's owner, by deleting the
 * subscription the BFF holds for this browser's push endpoint. Called on
 * sign-out -- Staff (#152) and the Client portal (#153) alike -- so the
 * next person on a shared laptop or a borrowed phone does not see the
 * previous person's pushes on the lock screen.
 *
 * The browser's own PushSubscription is left in place: the server row is
 * gone, so nothing can push to it, and the register endpoint upserts by
 * endpoint, so the next person to sign in on this device takes the same
 * endpoint over. Best-effort and never fatal, the same as its register
 * sibling -- a device that cannot be unregistered must not keep someone
 * signed in.
 */
/* v8 ignore start -- requires the Service Worker/PushManager browser APIs, which headless Vitest/Chromium does not give a subscription to drive */
export async function unregisterPushSubscription(
	unsubscribeURL: string,
	fetcher: Fetcher
): Promise<void> {
	try {
		if (!('serviceWorker' in navigator) || !('PushManager' in globalThis)) return;

		// getRegistration(), not the `ready` its register sibling awaits:
		// `ready` never settles when no worker ever activates -- it does not
		// reject, so no catch can rescue it -- and this call runs *before*
		// the end-session request, so hanging here would leave someone stuck
		// on a live session with a spinning button and no message.
		const registration = await navigator.serviceWorker.getRegistration();
		const subscription = await registration?.pushManager.getSubscription();
		if (!subscription) return;

		await fetcher(`${unsubscribeURL}?endpoint=${encodeURIComponent(subscription.endpoint)}`, {
			method: 'DELETE'
		});
	} catch {
		// Best-effort unregistration -- see the doc comment above.
	}
}
/* v8 ignore stop */
