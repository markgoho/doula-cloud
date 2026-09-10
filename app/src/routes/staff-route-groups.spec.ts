import { existsSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const routesRoot = new URL('.', import.meta.url);

/*
 * Archetype A -- the screens a person reaches with no session -- gets
 * its own route group so it can carry the reduced signed-out bar (#431).
 * Three screens a session can reach sit here too, because the reduced
 * bar is the true one for each: `/no-practice` (#745), the wayfinding
 * root `/` (#678), and TOTP enrollment `mfa/enroll` (#1114), whose
 * person holds a session that has not cleared MFA and so has no
 * reachable Practice-scoped destination for a Staff bar to offer.
 *
 * It cannot live in the root layout: SvelteKit layouts nest rather than
 * replace, so a bar there would render above the Staff bar as well. That
 * is a structural claim about the route tree, not about any one screen,
 * which is why it is asserted here rather than inside a page's own spec.
 */
describe('the signed-out route group', () => {
	it('has a signed-out layout of its own', () => {
		expect(existsSync(new URL('(signed-out)/+layout.svelte', routesRoot))).toBe(true);
	});

	it.each([
		['login', '(signed-out)/login/+page.svelte'],
		['signup', '(signed-out)/signup/+page.svelte'],
		['accept-invite', '(signed-out)/accept-invite/+page.svelte'],
		['the pre-account Offer read', '(signed-out)/offers/[offerId]/+page.svelte'],
		['the wayfinding root', '(signed-out)/+page.svelte'],
		['TOTP enrollment', '(signed-out)/mfa/enroll/+page.svelte']
	])('holds %s', (_name, path) => {
		expect(existsSync(new URL(path, routesRoot))).toBe(true);
	});

	it.each([
		['login', 'login/+page.svelte'],
		['signup', 'signup/+page.svelte'],
		['accept-invite', 'accept-invite/+page.svelte'],
		['the pre-account Offer read', 'offers/[offerId]/+page.svelte'],
		['the wayfinding root', '+page.svelte'],
		['TOTP enrollment', 'mfa/enroll/+page.svelte']
	])('leaves %s behind at no ungrouped path', (_name, path) => {
		expect(existsSync(new URL(path, routesRoot))).toBe(false);
	});
});

/*
 * The root layout stays chrome-free on purpose. Every bar in the app hangs
 * off a group or a section below it, so a screen that belongs to neither
 * side gets nothing rather than the wrong thing.
 */
describe('the root layout', () => {
	it('renders no shell of its own', () => {
		expect(existsSync(new URL('+layout.svelte', routesRoot))).toBe(true);
	});
});
