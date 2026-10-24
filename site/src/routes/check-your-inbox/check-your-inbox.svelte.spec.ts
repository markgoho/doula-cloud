import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PageInLayout from '../PageInLayout.svelte';
import { expectTheSiteFooter } from '../siteFooter.expect.js';
import Page from './+page.svelte';

/*
 * Where Buttondown sends a visitor after the waitlist form (#1799), in
 * place of its own page. Sign-up is double opt-in (ADR-0014), so this
 * page must not say she is on the list: the copy is #362's confirmation.
 */
describe('/check-your-inbox', () => {
	it('is unlisted: noindex, with its canonical address', async () => {
		await render(Page);
		expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe(
			'noindex, nofollow'
		);
		expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
			'https://doula.cloud/check-your-inbox'
		);
		expect(document.title).toBe('Check your inbox | DoulaCloud');
	});

	it('asks her to confirm, and says she is not on the list until she does', async () => {
		await render(Page);
		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Check your inbox' })).toBeVisible();
		await expect
			.element(
				testPage.getByText(
					"I sent a confirmation link to your email. Click it and you're on the list. Until you do, you're not."
				)
			)
			.toBeVisible();
		expect(testPage.getByRole('heading', { name: "You're on the list" }).elements()).toHaveLength(0);
	});

	it('is signed by the founder', async () => {
		await render(Page);
		await expect.element(testPage.getByText('Building DoulaCloud')).toBeVisible();
	});

	it('carries the site footer, and one contentinfo landmark', async () => {
		await render(PageInLayout, { page: Page });
		await expectTheSiteFooter();
	});
});
