import { describe, expect, it } from 'vitest';
import { portalLoginAfterSessionEnded, sessionEndedFrom, staffLoginAfterSessionEnded } from './sessionEnded';

describe('sessionEndedFrom', () => {
	it('recognizes a login address carrying the flag', () => {
		expect(sessionEndedFrom(new URL('https://example.test/login?sessionEnded=true'))).toBe(true);
	});

	it('reads an ordinary visit to the same screen as no session having ended', () => {
		expect(sessionEndedFrom(new URL('https://example.test/login'))).toBe(false);
	});

	it('reads any other value as an ordinary visit rather than a half-recognized one', () => {
		expect(sessionEndedFrom(new URL('https://example.test/login?sessionEnded=false'))).toBe(false);
		expect(sessionEndedFrom(new URL('https://example.test/login?sessionEnded'))).toBe(false);
	});

	it('is unmoved by another flag the same screen reads (#694)', () => {
		expect(sessionEndedFrom(new URL('https://example.test/login?codeSpent=true'))).toBe(false);
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
		expect(sessionEndedFrom(new URL(staffLoginAfterSessionEnded(), 'https://example.test'))).toBe(true);
		expect(sessionEndedFrom(new URL(portalLoginAfterSessionEnded(), 'https://example.test'))).toBe(true);
	});
});
