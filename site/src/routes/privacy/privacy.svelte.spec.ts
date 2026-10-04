import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PageInLayout from '../PageInLayout.svelte';
import { expectTheSiteFooter } from '../siteFooter.expect.js';
import Page from './+page.svelte';

/*
 * What a reader of /privacy sees, and what a crawler is given (#1556).
 * docs/copy/privacy-policy-page.md names the code or record each claim
 * was checked against; this spec holds the sentences that do legal work
 * by their words.
 */
describe('the Privacy Policy at /privacy (#1556)', () => {
	it('is indexed, with its canonical address', async () => {
		await render(Page);
		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Privacy Policy' })).toBeVisible();
		expect(document.head.querySelector('meta[name="robots"]')).toBeNull();
		expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
			'https://doula.cloud/privacy'
		);
	});

	it('says when this version takes effect, and shows the version history', async () => {
		await render(Page);
		await expect.element(testPage.getByText('Last updated October 3, 2026.')).toBeVisible();
		await expect.element(testPage.getByRole('heading', { level: 2, name: 'Version history' })).toBeVisible();
		await expect
			.element(testPage.getByRole('listitem').filter({ hasText: 'Not a material change. First version.' }))
			.toBeVisible();
		await expect
			.element(
				testPage.getByRole('listitem').filter({
					hasText:
						'Not a material change. Says how long Feedback is kept, what erases it, and that GitHub keeps a note of each piece.'
				})
			)
			.toBeVisible();
	});

	it('says how long Feedback is kept, what erases it, and that GitHub never receives the words typed', async () => {
		await render(Page);
		await expect
			.element(testPage.getByText('Feedback is deleted 24 months after it was sent', { exact: false }))
			.toBeVisible();
		await expect
			.element(testPage.getByText('erase it when your practice erases your record or is deleted', { exact: false }))
			.toBeVisible();
		await expect
			.element(testPage.getByText('Once your login is deleted, your Feedback no longer names you.', { exact: false }))
			.toBeVisible();
		await expect
			.element(testPage.getByText('It never holds the words a person typed', { exact: false }))
			.toBeVisible();
	});

	it.each(['If you own a practice', 'If you work at a practice', 'If you are a client of a practice', 'If you visit this site'])(
		'has a section for each person it holds data about: %s',
		async (heading) => {
			await render(Page);
			await expect.element(testPage.getByRole('heading', { level: 2, name: heading })).toBeVisible();
		}
	);

	it('tells a client to ask their practice, and gives our address if it does not answer', async () => {
		await render(Page);
		await expect
			.element(testPage.getByText('To see, correct or erase your record, ask your practice.'))
			.toBeVisible();
		await expect
			.element(testPage.getByText('If your practice does not answer you, write to us at', { exact: false }))
			.toBeVisible();
	});

	it('names each company that receives data', async () => {
		await render(Page);
		for (const company of [
			'Google Cloud',
			'Google Identity Platform',
			'Firebase Hosting',
			'Mailgun',
			'Stripe',
			"Your browser's push service",
			'GitHub'
		]) {
			await expect.element(testPage.getByText(company, { exact: true })).toBeVisible();
		}
	});

	it("links Stripe's Privacy Policy", async () => {
		await render(Page);
		await expect
			.element(testPage.getByRole('link', { name: "Stripe's Privacy Policy" }))
			.toHaveAttribute('href', 'https://stripe.com/privacy');
	});

	it('names the one cookie, and says the site sets none', async () => {
		await render(Page);
		await expect.element(testPage.getByText('__session')).toBeVisible();
		await expect.element(testPage.getByText('This site sets no cookies.')).toBeVisible();
	});

	it('refuses to sell data, to advertise, or to track email', async () => {
		await render(Page);
		await expect.element(testPage.getByText("We do not sell anyone's data.")).toBeVisible();
		await expect
			.element(testPage.getByText('No email we send is tracked for opens or clicks.', { exact: false }))
			.toBeVisible();
	});

	it('applies a material change to data already held only after an Owner agrees', async () => {
		await render(Page);
		await expect
			.element(
				testPage.getByText(
					'A material change to how we use data we already hold applies to that data only after an Owner of the practice has agreed to it.',
					{ exact: false }
				)
			)
			.toBeVisible();
	});

	it('carries the site footer, and one contentinfo landmark', async () => {
		await render(PageInLayout, { page: Page });
		await expectTheSiteFooter();
	});
});
