import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PageInLayout from './PageInLayout.svelte';
import Page from './+page.svelte';
import { expectTheSiteFooter } from './siteFooter.expect.js';

describe('the teaser at / (#358)', () => {
	it('is the letter, word for word from #362, with January 2027 as a date', async () => {
		await render(Page);
		await expect.element(testPage.getByText('Coming January 2027')).toBeVisible();
		await expect
			.element(
				testPage.getByRole('heading', {
					level: 1,
					name: "I'm building the thing you keep rebuilding in a spreadsheet."
				})
			)
			.toBeVisible();
		await expect
			.element(testPage.getByText("It opens in January 2027. It's not ready yet", { exact: false }))
			.toBeVisible();
		await expect
			.element(testPage.getByText("Put your name down and I'll write to you when it opens."))
			.toBeVisible();
		await expect.element(testPage.getByText('Building DoulaCloud')).toBeVisible();
	});

	it('puts the letter before the card, so a screen reader meets the note first', async () => {
		await render(Page);
		const heading = testPage.getByRole('heading', { level: 1 }).element();
		const card = testPage.getByRole('heading', { level: 2, name: 'Join the waitlist' }).element();
		expect(heading.compareDocumentPosition(card)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
	});

	it('is found and shared: a title, a description and the social card, with no canonical tag (#368)', async () => {
		await render(Page);
		expect(document.title).toBe('DoulaCloud: coming January 2027');
		expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toContain(
			'opens in January 2027'
		);
		expect(document.head.querySelector('link[rel="canonical"]')).toBeNull();
		expect(document.head.querySelector('meta[property="og:image"]')?.getAttribute('content')).toBe(
			'https://doula.cloud/social-card.png'
		);
		expect(document.head.querySelector('meta[name="robots"]')).toBeNull();
	});

	it('posts the waitlist form to Buttondown with all six field names exactly (#358)', async () => {
		const { container } = await render(Page);
		const form = container.querySelector('form');
		expect(form?.getAttribute('method')).toBe('post');
		expect(form?.getAttribute('action')).toBe(
			'https://buttondown.com/api/emails/embed-subscribe/doulacloud'
		);
		expect(
			[...(form?.elements ?? [])].map((element) => (element as HTMLInputElement).name).filter(Boolean)
		).toEqual([
			'metadata__first_name',
			'email',
			'metadata__pilot_interest',
			'metadata__utm_source',
			'metadata__utm_medium',
			'metadata__utm_campaign',
			'metadata__utm_content'
		]);
	});

	it('loads Pirsch, verbatim from #392, outside the form (#358)', async () => {
		await render(Page);
		const tag = document.head.querySelector('script#pianjs');
		expect(tag?.getAttribute('src')).toBe('https://api.pirsch.io/pa.js');
		expect((tag as HTMLElement | null)?.dataset.code).toBe('MbvnhdqLauluFOdAKiUtNuHisLThCYam');
		expect(tag?.hasAttribute('defer')).toBe(true);
	});

	it('carries the site footer, and one contentinfo landmark', async () => {
		await render(PageInLayout, { page: Page });
		await expectTheSiteFooter();
	});
});
