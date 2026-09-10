import { describe, expect, it } from 'vitest';
import { didSessionEnd, portalLoginAfterSessionEnded, staffLoginAfterSessionEnded } from './sessionEnded';

describe('didSessionEnd', () => {
	it('recognizes a login address carrying the flag', () => {
		expect(didSessionEnd(new URL('https://example.test/login?sessionEnded=true'))).toBe(true);
	});

	it('reads an ordinary visit to the same screen as no session having ended', () => {
		expect(didSessionEnd(new URL('https://example.test/login'))).toBe(false);
	});

	it('reads any other value as an ordinary visit rather than a half-recognized one', () => {
		expect(didSessionEnd(new URL('https://example.test/login?sessionEnded=false'))).toBe(false);
		expect(didSessionEnd(new URL('https://example.test/login?sessionEnded'))).toBe(false);
	});

	it('is unmoved by another flag the same screen reads (#694)', () => {
		expect(didSessionEnd(new URL('https://example.test/login?codeSpent=true'))).toBe(false);
	});
});

describe('the login addresses', () => {
	/*
	 * Pinned literally, not built from the module's own constants: these
	 * two strings are what `app/e2e/sign-out.e2e.ts` and
	 * `portal-sign-out.e2e.ts` wait for, and what every route-load spec
	 * asserts a refusal redirects to. A spec that spelled them the way the
	 * module does would agree with a rename instead of catching it.
	 */
	it('sends a Staff session that ended to the Staff login screen', () => {
		expect(staffLoginAfterSessionEnded()).toBe('/login?sessionEnded=true');
	});

	it('sends a Client-portal session that ended to the portal login screen', () => {
		expect(portalLoginAfterSessionEnded()).toBe('/portal/login?sessionEnded=true');
	});

	/*
	 * The property the convention actually needs, and the one nothing
	 * enforced before: what a writer emits is what a reader recognizes.
	 */
	it('produces addresses the reader recognizes', () => {
		const staff = new URL(staffLoginAfterSessionEnded(), 'https://example.test');
		const portal = new URL(portalLoginAfterSessionEnded(), 'https://example.test');

		expect(didSessionEnd(staff)).toBe(true);
		expect(didSessionEnd(portal)).toBe(true);
	});
});
