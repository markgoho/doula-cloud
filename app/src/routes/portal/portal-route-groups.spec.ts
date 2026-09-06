import { existsSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const portalRoot = new URL('.', import.meta.url);

describe('portal authenticated route group', () => {
	it('has an authenticated layout covering the engagement screens', () => {
		expect(existsSync(new URL('(authenticated)/+layout.svelte', portalRoot))).toBe(true);
		expect(existsSync(new URL('(authenticated)/engagements/[engagementId]/+page.svelte', portalRoot))).toBe(true);
	});

	it('does not put the signed-out login screen under the authenticated group', () => {
		expect(existsSync(new URL('(authenticated)/login/+page.svelte', portalRoot))).toBe(false);
	});

	it('does not put the signed-out accept-invite screen under the authenticated group', () => {
		expect(existsSync(new URL('(authenticated)/accept-invite/+page.svelte', portalRoot))).toBe(false);
	});
});

/*
 * The signed-out group exists because SvelteKit layouts nest rather than
 * replace (#431): a reduced bar in a shared parent would render above the
 * portal's own bar too. So the two portal screens a person can reach with
 * no session live in their own group, with their own layout, and this
 * asserts the arrangement rather than trusting whoever moves a route next.
 */
describe('portal signed-out route group', () => {
	it('has a signed-out layout of its own', () => {
		expect(existsSync(new URL('(signed-out)/+layout.svelte', portalRoot))).toBe(true);
	});

	it('holds every screen a Client can reach with no session', () => {
		expect(existsSync(new URL('(signed-out)/login/+page.svelte', portalRoot))).toBe(true);
		expect(existsSync(new URL('(signed-out)/accept-invite/+page.svelte', portalRoot))).toBe(true);
		/*
		 * #619's confirmation screen belongs here rather than under the
		 * authenticated group: the link arrives in the *new* mailbox, which
		 * may be read on a device that has never signed in, and the token is
		 * the whole credential. Putting it behind a session would make the
		 * link unusable exactly where it is meant to be opened.
		 */
		expect(existsSync(new URL('(signed-out)/confirm-sign-in-address/+page.svelte', portalRoot))).toBe(
			true
		);
	});

	it('leaves neither screen behind at the old ungrouped path', () => {
		expect(existsSync(new URL('login/+page.svelte', portalRoot))).toBe(false);
		expect(existsSync(new URL('accept-invite/+page.svelte', portalRoot))).toBe(false);
	});
});
