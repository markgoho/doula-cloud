import { page as testPage } from 'vitest/browser';
import { expect } from 'vitest';

/**
 * What every page of the site owes a reader at its foot (#1556): one
 * `contentinfo` landmark, and in it a link to the Terms of Service and a
 * link to the Privacy Policy. Two `contentinfo` landmarks on one page
 * make each of them ambiguous to a screen reader, so the count is part
 * of the assertion. Call it after rendering the page through
 * PageInLayout.svelte, since the footer is the layout's.
 */
export async function expectTheSiteFooter(): Promise<void> {
	const footer = testPage.getByRole('contentinfo');
	expect(footer.elements()).toHaveLength(1);
	await expect
		.element(footer.getByRole('link', { name: 'Terms of Service' }))
		.toHaveAttribute('href', '/terms');
	await expect
		.element(footer.getByRole('link', { name: 'Privacy Policy' }))
		.toHaveAttribute('href', '/privacy');
}
