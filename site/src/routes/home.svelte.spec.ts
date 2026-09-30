import { describe, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PageInLayout from './PageInLayout.svelte';
import Page from './+page.svelte';
import { expectTheSiteFooter } from './siteFooter.expect.js';

// The home page is not a ReadingPage, which is why the site footer lives
// in the layout (#1556): this is the page that proves it.
describe('the home page', () => {
	it('carries the site footer, and one contentinfo landmark (#1556)', async () => {
		await render(PageInLayout, { page: Page });
		await expectTheSiteFooter();
	});
});
