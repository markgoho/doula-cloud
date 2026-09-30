import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PageInLayout from '../PageInLayout.svelte';
import { expectTheSiteFooter } from '../siteFooter.expect.js';
import Page from './+page.svelte';

describe('the pilot terms at /pilot-terms (#444)', () => {
	it('is unlisted: noindex, with its canonical address', async () => {
		await render(Page);
		expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe(
			'noindex, nofollow'
		);
		expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
			'https://doula.cloud/pilot-terms'
		);
	});

	it('states the price and the grant', async () => {
		await render(Page);
		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Pilot terms' })).toBeVisible();
		await expect.element(testPage.getByText('three free Credits for each person on its staff')).toBeVisible();
		await expect.element(testPage.getByRole('heading', { level: 2, name: 'Refunds' })).toBeVisible();
	});

	it('says when the terms last changed, under its main content', async () => {
		await render(Page);
		await expect.element(testPage.getByText('Last updated August 29, 2026.')).toBeVisible();
		await expect.element(testPage.getByText('August 29, 2026')).toHaveAttribute('datetime', '2026-08-29');
	});

	it('carries the site footer, and no second contentinfo of its own (#1556)', async () => {
		await render(PageInLayout, { page: Page });
		await expectTheSiteFooter();
	});

	it('is linked from nothing on its own page, the site footer included (#444)', async () => {
		await render(PageInLayout, { page: Page });
		expect(document.querySelector('a[href*="pilot-terms"]')).toBeNull();
	});
});
