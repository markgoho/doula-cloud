import { existsSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const portalRoot = new URL('.', import.meta.url);

describe('portal authenticated route group', () => {
	it('has an authenticated layout covering the engagement screens', () => {
		expect(existsSync(new URL('(authenticated)/+layout.svelte', portalRoot))).toBe(true);
		expect(existsSync(new URL('(authenticated)/engagements/[engagementId]/+page.svelte', portalRoot))).toBe(true);
	});

	it('does not put the signed-out login screen under the authenticated group', () => {
		expect(existsSync(new URL('login/+page.svelte', portalRoot))).toBe(true);
		expect(existsSync(new URL('(authenticated)/login/+page.svelte', portalRoot))).toBe(false);
	});

	it('does not put the signed-out accept-invite screen under the authenticated group', () => {
		expect(existsSync(new URL('accept-invite/+page.svelte', portalRoot))).toBe(true);
		expect(existsSync(new URL('(authenticated)/accept-invite/+page.svelte', portalRoot))).toBe(false);
	});
});
