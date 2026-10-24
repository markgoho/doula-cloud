import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import VersionHistory from './VersionHistory.svelte';

describe('VersionHistory', () => {
	it('lists every version newest first, each with its material flag and what changed', async () => {
		await render(VersionHistory, {
			legalDocument: {
				name: 'Terms of Service',
				path: '/terms',
				versions: [
					{ effective: '2026-09-29', material: false, change: 'First version.' },
					{ effective: '2026-11-01', material: true, change: 'The price of a Credit changes.' }
				]
			}
		});

		await expect.element(testPage.getByRole('heading', { level: 2, name: 'Version history' })).toBeVisible();
		const items = testPage.getByRole('listitem').elements();
		expect(items.map((item) => item.textContent?.replaceAll(/\s+/g, ' ').trim())).toEqual([
			'November 1, 2026. A material change. The price of a Credit changes.',
			'September 29, 2026. Not a material change. First version.'
		]);
		await expect.element(testPage.getByText('November 1, 2026')).toHaveAttribute('datetime', '2026-11-01');
		expect(testPage.getByRole('link').elements()).toEqual([]);
	});

	// #1816: the version an Owner agreed to stays one press away after a
	// newer one starts.
	it('links the date of each version whose text is kept to its own page', async () => {
		await render(VersionHistory, {
			legalDocument: {
				name: 'Terms of Service',
				path: '/terms',
				versions: [
					{ effective: '2026-09-29', material: false, change: 'First version.' },
					{ effective: '2026-10-03', material: false, change: 'Writes the name of the product as one word.' }
				]
			}
		});

		await expect.element(testPage.getByRole('link', { name: 'October 3, 2026' })).toHaveAttribute('href', '/terms/2026-10-03');
		expect(testPage.getByRole('link').elements()).toHaveLength(1);
	});
});
