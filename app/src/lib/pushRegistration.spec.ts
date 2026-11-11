import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	isHomeScreenRequiredForPush,
	isPushPermissionGranted,
	requiresHomeScreenForPush,
	portalNotificationPreferencePath,
	portalPushSubscriptionsPath,
	practicePushSubscriptionsPath,
	registerPushSubscriptionIfEnabled,
	urlBase64ToUint8Array,
	vapidPublicKey
} from './pushRegistration.js';

describe('vapidPublicKey', () => {
	it('defaults to empty string when unset', () => {
		expect(vapidPublicKey()).toBe('');
	});
});

describe('urlBase64ToUint8Array', () => {
	it('decodes a URL-safe base64 string into its raw bytes', () => {
		// "hello" base64-encoded, with a trailing "=" stripped and no
		// URL-unsafe characters -- exercises the padding restoration path.
		const bytes = urlBase64ToUint8Array('aGVsbG8');
		expect(new TextDecoder().decode(bytes)).toBe('hello');
	});

	it('restores URL-safe "-"/"_" substitutions back to "+"/"/"', () => {
		// Byte 0xFB 0xFF encodes to "-_8=" in URL-safe base64
		// ("+/8=" in standard base64).
		const bytes = urlBase64ToUint8Array('-_8=');
		expect([...bytes]).toEqual([0xFB, 0xFF]);
	});
});

describe('push-subscriptions paths', () => {
	it('scopes the Staff endpoint by Practice', () => {
		expect(practicePushSubscriptionsPath('practice-1')).toBe(
			'/api/practices/practice-1/push-subscriptions'
		);
	});

	it('scopes the Client portal endpoint by Engagement', () => {
		expect(portalPushSubscriptionsPath('engagement-1')).toBe(
			'/api/portal/engagements/engagement-1/push-subscriptions'
		);
	});

	it('scopes the notification-preference endpoint by Engagement', () => {
		expect(portalNotificationPreferencePath('engagement-1')).toBe(
			'/api/portal/engagements/engagement-1/notification-preference'
		);
	});
});

describe('registerPushSubscriptionIfEnabled', () => {
	it('does not register when the stored preference is off', async () => {
		const register = vi.fn();
		const fetcher = vi.fn(async () => Response.json({ enabled: false }));

		await registerPushSubscriptionIfEnabled('/preference', '/subscribe', fetcher, register);

		expect(register).not.toHaveBeenCalled();
	});

	it('registers when the stored preference is on', async () => {
		const register = vi.fn();
		const fetcher = vi.fn(async () => Response.json({ enabled: true }));

		await registerPushSubscriptionIfEnabled('/preference', '/subscribe', fetcher, register);

		expect(register).toHaveBeenCalledWith('/subscribe', fetcher);
	});

	it('treats a failed preference read as off, never registering', async () => {
		const register = vi.fn();
		const fetcher = vi.fn(async () => {
			throw new Error('network down');
		});

		await registerPushSubscriptionIfEnabled('/preference', '/subscribe', fetcher, register);

		expect(register).not.toHaveBeenCalled();
	});

	it('treats a non-OK response as off, never registering', async () => {
		const register = vi.fn();
		const fetcher = vi.fn(async () => new Response('', { status: 401 }));

		await registerPushSubscriptionIfEnabled('/preference', '/subscribe', fetcher, register);

		expect(register).not.toHaveBeenCalled();
	});
});

describe('isPushPermissionGranted', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('is false when the Push/Notification APIs are unsupported', () => {
		expect(isPushPermissionGranted()).toBe(false);
	});

	it('is false when this device has never been asked or has denied permission', () => {
		vi.stubGlobal('Notification', { permission: 'default' });
		vi.stubGlobal('navigator', { serviceWorker: {} });
		vi.stubGlobal('PushManager', class {});

		expect(isPushPermissionGranted()).toBe(false);

		vi.stubGlobal('Notification', { permission: 'denied' });
		expect(isPushPermissionGranted()).toBe(false);
	});

	it('is true only once permission is granted and Push is supported', () => {
		vi.stubGlobal('Notification', { permission: 'granted' });
		vi.stubGlobal('navigator', { serviceWorker: {} });
		vi.stubGlobal('PushManager', class {});

		expect(isPushPermissionGranted()).toBe(true);
	});
});

const IPHONE_SAFARI =
	'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1';
const IPAD_SAFARI =
	'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15';
const IPHONE_CHROME =
	'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/123.0 Mobile/15E148 Safari/604.1';

const inBrowserTab = {
	userAgent: IPHONE_SAFARI,
	maxTouchPoints: 5,
	isStandalone: false,
	hasPushManager: false
};

describe('requiresHomeScreenForPush', () => {
	it('is true on an iPhone in Safari, outside standalone mode, with no PushManager', () => {
		expect(requiresHomeScreenForPush(inBrowserTab)).toBe(true);
	});

	it('is true on an iPad that presents as a Mac', () => {
		expect(requiresHomeScreenForPush({ ...inBrowserTab, userAgent: IPAD_SAFARI })).toBe(true);
	});

	it('is false once the app is added to the Home Screen', () => {
		expect(requiresHomeScreenForPush({ ...inBrowserTab, isStandalone: true, hasPushManager: true })).toBe(false);
		expect(requiresHomeScreenForPush({ ...inBrowserTab, isStandalone: true })).toBe(false);
	});

	it('is false in any browser that has PushManager', () => {
		expect(requiresHomeScreenForPush({ ...inBrowserTab, hasPushManager: true })).toBe(false);
	});

	it('is false on a Mac with no touch screen and in another browser on iOS', () => {
		expect(requiresHomeScreenForPush({ ...inBrowserTab, userAgent: IPAD_SAFARI, maxTouchPoints: 0 })).toBe(false);
		expect(requiresHomeScreenForPush({ ...inBrowserTab, userAgent: IPHONE_CHROME })).toBe(false);
		expect(requiresHomeScreenForPush({ ...inBrowserTab, userAgent: 'Mozilla/5.0 (Linux; Android 14)' })).toBe(false);
	});
});

describe('isHomeScreenRequiredForPush', () => {
	it('reads the device: true for iPhone Safari in a tab, false once standalone', () => {
		vi.stubGlobal('navigator', { userAgent: IPHONE_SAFARI, maxTouchPoints: 5 });
		vi.stubGlobal('matchMedia', () => ({ matches: false }));
		vi.stubGlobal('PushManager', class {});
		expect(isHomeScreenRequiredForPush()).toBe(false);

		Reflect.deleteProperty(globalThis, 'PushManager');
		expect(isHomeScreenRequiredForPush()).toBe(true);

		vi.stubGlobal('matchMedia', () => ({ matches: true }));
		expect(isHomeScreenRequiredForPush()).toBe(false);

		vi.stubGlobal('matchMedia', () => ({ matches: false }));
		vi.stubGlobal('navigator', { userAgent: IPHONE_SAFARI, maxTouchPoints: 5, standalone: true });
		expect(isHomeScreenRequiredForPush()).toBe(false);
	});
});
