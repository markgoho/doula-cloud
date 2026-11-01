import { beforeEach, describe, expect, it } from 'vitest';
import {
	didSessionEnd,
	forgetSessionHeld,
	noteSessionHeld,
	portalLoginAfterRefusal,
	staffLoginAfterRefusal,
	staffLoginAfterSessionEnded
} from './sessionEnded';

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
	beforeEach(() => {
		forgetSessionHeld();
	});

	/*
	 * Pinned literally, not built from the module's own constants: these
	 * strings are what `app/e2e/sign-out.e2e.ts` and
	 * `portal-sign-out.e2e.ts` wait for, and what every route-load spec
	 * asserts a refusal redirects to. A spec that spelled them the way the
	 * module does would agree with a rename instead of catching it.
	 */
	it('sends a Staff session that ended to the Staff login screen', () => {
		expect(staffLoginAfterSessionEnded()).toBe('/login?sessionEnded=true');
	});

	// #2259: a device that never signed in was not signed out.
	it('sends a refusal in a document that held no session to the plain Staff login, carrying returnTo', () => {
		expect(staffLoginAfterRefusal('/practices/p-1/birth-plan')).toBe(
			'/login?returnTo=%2Fpractices%2Fp-1%2Fbirth-plan'
		);
	});

	it('says the session ended when this document held one, and still carries returnTo', () => {
		noteSessionHeld();

		expect(staffLoginAfterRefusal('/practices/p-1')).toBe('/login?sessionEnded=true&returnTo=%2Fpractices%2Fp-1');
	});

	it('reads a refusal after sign-out as no session having ended', () => {
		noteSessionHeld();
		forgetSessionHeld();

		expect(staffLoginAfterRefusal('/practices/p-1')).toBe('/login?returnTo=%2Fpractices%2Fp-1');
	});

	it('sends a portal refusal to the portal login, flagged only when this document held a session', () => {
		expect(portalLoginAfterRefusal('/portal/engagements/e-1/birth-plan')).toBe(
			'/portal/login?returnTo=%2Fportal%2Fengagements%2Fe-1%2Fbirth-plan'
		);

		noteSessionHeld();

		expect(portalLoginAfterRefusal('/portal/engagements/e-1')).toBe(
			'/portal/login?sessionEnded=true&returnTo=%2Fportal%2Fengagements%2Fe-1'
		);
	});

	/*
	 * The property the convention actually needs, and the one nothing
	 * enforced before: what a writer emits is what a reader recognizes.
	 */
	it('produces addresses the reader recognizes', () => {
		noteSessionHeld();
		const staff = new URL(staffLoginAfterRefusal('/practices/p-1'), 'https://example.test');
		const portal = new URL(portalLoginAfterRefusal('/portal/engagements/e-1'), 'https://example.test');

		const ended = new URL(staffLoginAfterSessionEnded(), 'https://example.test');

		expect(didSessionEnd(ended)).toBe(true);
		expect(didSessionEnd(staff)).toBe(true);
		expect(didSessionEnd(portal)).toBe(true);
	});
});
