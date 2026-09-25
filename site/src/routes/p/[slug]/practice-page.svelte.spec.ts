import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { PracticePage } from '#lib/practicePage.js';
import Page from './+page.svelte';

const hostile: PracticePage = {
	slug: 'river-birth',
	name: 'River Birth <b>& Co</b>',
	serviceDescription: 'Birth support.\nPostpartum <script>window.pwned = true</script> *care*.',
	cancellationPolicy: 'Line one.\nLine two with <img src=x onerror="window.pwned = true"> [a](b).',
	supportName: 'Ada <i>Owner</i>',
	supportEmail: 'ada@example.com',
	publishedAt: '2026-09-01T12:00:00.000Z'
};

function renderPage(page: PracticePage = hostile) {
	return render(Page, { data: { page }, params: { slug: page.slug } } as never);
}

describe('a Practice page at /p/<slug>/', () => {
	it('prints everything she typed as text, never as markup (#441)', async () => {
		const { container } = await renderPage();

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: 'River Birth <b>& Co</b>' }))
			.toBeVisible();
		await expect
			.element(testPage.getByText('Postpartum <script>window.pwned = true</script> *care*.', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByText('Ada <i>Owner</i>', { exact: false })).toBeVisible();
		expect(container.querySelector(':scope main :is(script, img, b, i)')).toBeNull();
		expect((globalThis as unknown as { pwned?: boolean }).pwned).toBeUndefined();
	});

	it('keeps the line breaks she typed', async () => {
		await renderPage();
		const policy = testPage.getByText('Line one.', { exact: false }).element();
		expect(getComputedStyle(policy).whiteSpace).toBe('pre-line');
	});

	it('carries the marker the #443 probe reads, on its main landmark', async () => {
		await renderPage();
		await expect.element(testPage.getByRole('main')).toHaveAttribute('data-practice-page', '');
	});

	it('gives the support contact as an address with a mailto link', async () => {
		await renderPage();
		await expect
			.element(testPage.getByRole('link', { name: 'ada@example.com' }))
			.toHaveAttribute('href', 'mailto:ada@example.com');
	});

	it('says when it was last published, and who published it', async () => {
		await renderPage();
		await expect.element(testPage.getByText('September 1, 2026')).toHaveAttribute('datetime', '2026-09-01');
		await expect
			.element(testPage.getByRole('link', { name: 'Doula Cloud' }))
			.toHaveAttribute('href', 'https://doula.cloud/');
	});

	it('is indexable, with a canonical address and a description from her own words', async () => {
		await renderPage();
		expect(document.head.querySelector('meta[name="robots"]')).toBeNull();
		expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
			'https://doula.cloud/p/river-birth/'
		);
		expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toBe(
			'Birth support. Postpartum <script>window.pwned = true</script> *care*.'
		);
	});
});
