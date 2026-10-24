import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PageInLayout from '../PageInLayout.svelte';
import { expectTheSiteFooter } from '../siteFooter.expect.js';
import Page from './+page.svelte';

/*
 * Where Buttondown sends a subscriber after she clicks "Confirm my email"
 * (#1799), in place of its own page. Confirming starts the Welcome email,
 * which carries the pilot offer (#361), so the page says it is coming.
 */
describe('/on-the-list', () => {
	it('is unlisted: noindex, with its canonical address', async () => {
		await render(Page);
		expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe(
			'noindex, nofollow'
		);
		expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
			'https://doula.cloud/on-the-list'
		);
		expect(document.title).toBe("You're on the list | DoulaCloud");
	});

	it('says she is on the list, and that the welcome email with the pilot is on its way', async () => {
		await render(Page);
		await expect.element(testPage.getByRole('heading', { level: 1, name: "You're on the list" })).toBeVisible();
		await expect
			.element(
				testPage.getByText(
					'Thanks for confirming. A welcome email from me is on its way, with a small pilot you can join before DoulaCloud opens in January 2027.'
				)
			)
			.toBeVisible();
		await expect
			.element(testPage.getByText('If it lands in your Promotions tab, drag it to Primary so you see the next one.'))
			.toBeVisible();
	});

	it('is signed by the founder', async () => {
		await render(Page);
		await expect.element(testPage.getByText('Building DoulaCloud')).toBeVisible();
	});

	it('carries the site footer, and one contentinfo landmark', async () => {
		await render(PageInLayout, { page: Page });
		await expectTheSiteFooter();
	});

	it('does not link the unlisted pilot terms (#444)', async () => {
		await render(PageInLayout, { page: Page });
		expect(document.querySelector('a[href*="pilot-terms"]')).toBeNull();
	});
});
