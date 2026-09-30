import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PageInLayout from '../PageInLayout.svelte';
import PilotTerms from '../pilot-terms/+page.svelte';
import { expectTheSiteFooter } from '../siteFooter.expect.js';
import Page from './+page.svelte';

/*
 * What a reader of /terms sees, and what a crawler is given (#1556). The
 * sentences asserted here are the ones that do legal work, by their
 * words, so an edit that drops one fails the build rather than going
 * unnoticed; docs/copy/terms-of-service-page.md says why each is there.
 */
describe('the Terms of Service at /terms (#1556)', () => {
	it('is indexed, with its canonical address', async () => {
		await render(Page);
		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Terms of Service' })).toBeVisible();
		expect(document.head.querySelector('meta[name="robots"]')).toBeNull();
		expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
			'https://doula.cloud/terms'
		);
	});

	it('names the parties, and the date this version takes effect', async () => {
		await render(Page);
		await expect
			.element(testPage.getByText('Elephantine LLC, a New York limited liability company', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByText('Last updated September 29, 2026.')).toBeVisible();
		await expect.element(testPage.getByText('This version takes effect on', { exact: false })).toBeVisible();
	});

	it('shows the version history, with the material flag and what changed', async () => {
		await render(Page);
		await expect.element(testPage.getByRole('heading', { level: 2, name: 'Version history' })).toBeVisible();
		expect(normalized(section('Version history').querySelector('li')?.textContent ?? undefined)).toBe(
			'September 29, 2026. Not a material change. First version.'
		);
	});

	it('states the price of a Credit, and that there is nothing else to pay', async () => {
		await render(Page);
		await expect.element(testPage.getByText('$20.00')).toBeVisible();
		await expect
			.element(testPage.getByText('There is no subscription, no minimum and no recurring charge.', { exact: false }))
			.toBeVisible();
		await expect
			.element(testPage.getByText('A rise never changes a Credit you already bought', { exact: false }))
			.toBeVisible();
	});

	it('gives refunds a heading of their own, with the same words as the pilot terms', async () => {
		await render(Page);
		await expect.element(testPage.getByRole('heading', { level: 2, name: 'Refunds' })).toBeVisible();
		const termsText = section('Refunds').textContent;
		document.body.replaceChildren();

		await render(PilotTerms);
		const pilot = section('Refunds').textContent;
		expect(normalized(termsText)).toBe(normalized(pilot));
		expect(normalized(termsText)).toContain('We do not need a reason.');
	});

	it('promises 30 days of notice, and no retroactive change', async () => {
		await render(Page);
		await expect
			.element(testPage.getByText('we email each Owner at least 30 days before the new version takes effect', { exact: false }))
			.toBeVisible();
		await expect
			.element(testPage.getByText('A change never applies to a dispute, a claim or a purchase that came before it.'))
			.toBeVisible();
	});

	it("links the Stripe Connected Account Agreement and Stripe's Privacy Policy", async () => {
		await render(Page);
		await expect
			.element(testPage.getByRole('link', { name: 'Stripe Connected Account Agreement' }))
			.toHaveAttribute('href', 'https://stripe.com/connect-account/legal/full');
		await expect
			.element(testPage.getByRole('link', { name: "Stripe's Privacy Policy" }))
			.toHaveAttribute('href', 'https://stripe.com/privacy');
	});

	it('says the practice is the merchant, and that no business associate agreement is signed', async () => {
		await render(Page);
		await expect
			.element(testPage.getByText('your practice is the merchant: it holds its own agreement with Stripe', { exact: false }))
			.toBeVisible();
		await expect
			.element(testPage.getByText('it signs no business associate agreement today', { exact: false }))
			.toBeVisible();
	});

	it('puts a dispute before the courts for Monroe County, under New York law, with nothing in their place', async () => {
		await render(Page);
		await expect
			.element(testPage.getByText('These terms are governed by the law of the State of New York.'))
			.toBeVisible();
		await expect
			.element(testPage.getByText('the state or federal courts for Monroe County, New York', { exact: false }))
			.toBeVisible();
		await expect
			.element(testPage.getByText('on your own or together with others', { exact: false }))
			.toBeVisible();
		expect(document.body.textContent?.toLowerCase()).not.toContain('arbitration');
	});

	it('carries the site footer, and one contentinfo landmark', async () => {
		await render(PageInLayout, { page: Page });
		await expectTheSiteFooter();
	});
});

// The section a level-two heading opens, whole: its heading and its text.
function section(heading: string): Element {
	const found = testPage.getByRole('heading', { level: 2, name: heading }).element().closest('section');
	if (!found) throw new Error(`no section under the heading ${heading}`);
	return found;
}

function normalized(text: string | null | undefined): string {
	return (text ?? '').replaceAll(/\s+/g, ' ').trim();
}
