import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PageTitle from './PageTitle.svelte';

describe('PageTitle', () => {
	it('sets the browser tab title', async () => {
		await render(PageTitle, { page: 'Clients' });

		await expect.poll(() => document.title).toBe('Clients — Doula Cloud');
	});

	it('updates the title when the page prop changes', async () => {
		const { rerender } = await render(PageTitle, { page: 'Clients' });

		await rerender({ page: 'Billing' });

		await expect.poll(() => document.title).toBe('Billing — Doula Cloud');
	});

	it('prefixes Error: when a refused form is showing', async () => {
		await render(PageTitle, { page: 'Log in', isError: true });

		await expect.poll(() => document.title).toBe('Error: Log in — Doula Cloud');
	});

	it("uses the Practice's own name on the portal", async () => {
		await render(PageTitle, { page: 'Your care', serviceName: 'Riverside Doulas' });

		await expect.poll(() => document.title).toBe('Your care — Riverside Doulas');
	});
});
