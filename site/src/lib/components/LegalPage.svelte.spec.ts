import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { PRIVACY, TERMS, type LegalDocument } from '#lib/legal.js';
import LegalPage from './LegalPage.svelte';

describe('LegalPage (#1816)', () => {
	it.each([TERMS, PRIVACY])('prints the $name version it is given, with its date and the history', async (legalDocument) => {
		const version = legalDocument.versions.at(-1)!;
		await render(LegalPage, { legalDocument, version });

		await expect.element(testPage.getByRole('heading', { level: 1, name: legalDocument.name })).toBeVisible();
		await expect.element(testPage.getByText('This version takes effect on', { exact: false })).toBeVisible();
		await expect.element(testPage.getByRole('heading', { level: 2, name: 'Version history' })).toBeVisible();
	});

	// During the 30 days: the version in force, and above its text the one
	// that is coming.
	it('puts a newer version above the text of the one it is given', async () => {
		const withComing: LegalDocument = {
			...TERMS,
			versions: [...TERMS.versions, { effective: '2099-01-01', material: true, change: 'The price of a Credit changes.' }]
		};
		await render(LegalPage, { legalDocument: withComing, version: TERMS.versions.at(-1)! });

		const notice = testPage.getByRole('region', { name: 'A new version takes effect on January 1, 2099' });
		await expect.element(notice).toBeVisible();
		await expect
			.element(notice.getByRole('link', { name: 'Read the version that takes effect on January 1, 2099' }))
			.toHaveAttribute('href', '/terms/2099-01-01');
		const headings = testPage.getByRole('heading', { level: 2 }).elements().map((h) => h.textContent);
		expect(headings[0]).toBe('A new version takes effect on January 1, 2099');
	});
});
