import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { PRIVACY, TERMS } from '#lib/legal.js';
import { textedVersions } from '#lib/legalTexts.js';
import PrivacyVersion from './privacy/[version]/+page.svelte';
import * as privacyVersion from './privacy/[version]/+page.js';
import TermsVersion from './terms/[version]/+page.svelte';
import * as termsVersion from './terms/[version]/+page.js';

/*
 * Each version's own page, at /terms/<date> and /privacy/<date> (#1816):
 * the address the email before a material change links, built for every
 * version that has a text.
 */
describe.each([
	{ legalDocument: TERMS, Page: TermsVersion, route: termsVersion },
	{ legalDocument: PRIVACY, Page: PrivacyVersion, route: privacyVersion }
])('the $legalDocument.name, one version at a time', ({ legalDocument, Page, route }) => {
	it('builds a page for every version with a text, and none for another date', () => {
		expect(route.entries()).toEqual(textedVersions(legalDocument).map(({ effective }) => ({ version: effective })));
		const load = route.load as (event: { params: { version: string } }) => unknown;
		expect(() => load({ params: { version: '1999-01-01' } })).toThrow();
	});

	it('prints the version it names, at its own canonical address', async () => {
		const version = textedVersions(legalDocument).at(-1)!;
		const load = route.load as (event: { params: { version: string } }) => { version: typeof version };
		const data = load({ params: { version: version.effective } });
		await render(Page, { data, params: { version: version.effective } } as never);

		await expect.element(testPage.getByRole('heading', { level: 1, name: legalDocument.name })).toBeVisible();
		expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
			`https://doula.cloud${legalDocument.path}/${version.effective}`
		);
		expect(document.title).toContain(legalDocument.name);
	});
});
