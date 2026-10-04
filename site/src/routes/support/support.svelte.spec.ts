import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PageInLayout from '../PageInLayout.svelte';
import { expectTheSiteFooter } from '../siteFooter.expect.js';
import Page from './+page.svelte';

/*
 * What Stripe reads at /support (#390, #358): a description of the
 * service, the refund position, and a contact. Stripe re-checks this URL
 * for as long as the platform account is live, so each of the three is
 * asserted by its words.
 */
describe('/support', () => {
	it('is indexable, with no canonical tag, as the copy document sets it', async () => {
		await render(Page);
		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Support and billing' })).toBeVisible();
		expect(document.head.querySelector('meta[name="robots"]')).toBeNull();
		expect(document.head.querySelector('link[rel="canonical"]')).toBeNull();
	});

	it('says what the product is, and that it is not a payment service', async () => {
		await render(Page);
		await expect
			.element(testPage.getByText('DoulaCloud is practice-management software for doulas and doula agencies.', { exact: false }))
			.toBeVisible();
		await expect
			.element(testPage.getByText('never receives or holds a practice', { exact: false }))
			.toBeVisible();
	});

	it('states the refund position the Terms of Service bind', async () => {
		await render(Page);
		await expect.element(testPage.getByRole('heading', { level: 2, name: 'Refunds and cancellation' })).toBeVisible();
		await expect
			.element(testPage.getByText('can be refunded within three years of the date they were bought', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByText('To ask for a refund, email us. We do not need a reason.')).toBeVisible();
	});

	it('gives a contact that reaches a person', async () => {
		await render(Page);
		await expect.element(testPage.getByText('hello@doula.cloud')).toBeVisible();
	});

	it('carries the site footer, and one contentinfo landmark', async () => {
		await render(PageInLayout, { page: Page });
		await expectTheSiteFooter();
	});
});
