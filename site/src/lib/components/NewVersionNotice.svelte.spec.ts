import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { LegalDocument } from '#lib/legal.js';
import NewVersionNotice from './NewVersionNotice.svelte';

const HISTORY: LegalDocument = {
	name: 'Terms of Service',
	path: '/terms',
	versions: [
		{ effective: '2026-10-03', material: false, change: 'Writes the name of the product as one word.' },
		{ effective: '2026-11-01', material: true, change: 'The price of a Credit changes.' }
	]
};

describe('NewVersionNotice (#1816)', () => {
	it('names the newer version, when it takes effect, what changes, and links its text', async () => {
		await render(NewVersionNotice, { legalDocument: HISTORY, version: HISTORY.versions[0] });

		const region = testPage.getByRole('region', { name: 'A new version takes effect on November 1, 2026' });
		await expect.element(region).toBeVisible();
		await expect.element(region.getByText('November 1, 2026', { exact: true })).toHaveAttribute('datetime', '2026-11-01');
		await expect.element(region.getByText('A material change. The price of a Credit changes.')).toBeVisible();
		await expect
			.element(region.getByRole('link', { name: 'Read the version that takes effect on November 1, 2026' }))
			.toHaveAttribute('href', '/terms/2026-11-01');
	});

	it('says nothing on the newest version', async () => {
		await render(NewVersionNotice, { legalDocument: HISTORY, version: HISTORY.versions[1] });
		expect(testPage.getByRole('region').elements()).toEqual([]);
	});

	it('says which change is not material', async () => {
		const history: LegalDocument = {
			...HISTORY,
			versions: [HISTORY.versions[0], { effective: '2026-11-01', material: false, change: 'Fixes a typing error.' }]
		};
		await render(NewVersionNotice, { legalDocument: history, version: history.versions[0] });
		await expect.element(testPage.getByText('Not a material change. Fixes a typing error.')).toBeVisible();
	});
});
