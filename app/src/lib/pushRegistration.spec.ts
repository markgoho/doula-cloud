import { describe, expect, it } from 'vitest';
import { urlBase64ToUint8Array, vapidPublicKey } from './pushRegistration.js';

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
		expect(Array.from(bytes)).toEqual([0xfb, 0xff]);
	});
});
