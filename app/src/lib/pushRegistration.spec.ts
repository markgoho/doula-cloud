import { describe, expect, it, vi } from 'vitest';
import {
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
