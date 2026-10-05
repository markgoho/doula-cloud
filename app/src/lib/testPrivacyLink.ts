import { page } from 'vitest/browser';
import { expect } from 'vitest';
import { privacyPolicyUrl } from '#lib/legal.js';

/**
The assertion every Portal shell spec makes (#1558): the Privacy Policy
link is in the page footer landmark, goes to the live policy, and comes
after `<main>` in the document, so it is in the same place on every
Portal screen.
*/
export async function expectPrivacyLinkAfterMain(): Promise<void> {
	const link = page
		.getByRole('contentinfo')
		.getByRole('link', { name: 'Privacy Policy (opens in new tab)' });
	await expect.element(link).toBeVisible();
	await expect.element(link).toHaveAttribute('href', privacyPolicyUrl);
	const main = page.getByRole('main').element();
	expect(main.compareDocumentPosition(link.element()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
}
