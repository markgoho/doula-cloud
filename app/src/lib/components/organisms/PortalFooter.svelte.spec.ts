import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import '#lib/styles/app.css';
import PortalFooter from './PortalFooter.svelte';

async function setup() {
	await render(PortalFooter);
}

describe('PortalFooter.svelte', () => {
	it('links to the live Privacy Policy, in a new tab, and says so on screen', async () => {
		await setup();

		const link = page.getByRole('link', { name: 'Privacy Policy (opens in new tab)' });
		await expect.element(link).toBeVisible();
		await expect.element(link).toHaveAttribute('href', 'https://doula.cloud/privacy');
		await expect.element(link).toHaveAttribute('target', '_blank');
		await expect
			.element(page.getByText('Privacy Policy (opens in new tab)', { exact: true }))
			.toBeVisible();
	});

	it('sits in the page footer landmark', async () => {
		await setup();

		await expect
			.element(page.getByRole('contentinfo').getByRole('link', { name: /Privacy Policy/ }))
			.toBeVisible();
	});

	// The brief's Density and Fitts's Law rule: no hit target under 44px.
	// The default width runs first, before any test pins the viewport.
	it.each([
		['the default width', undefined],
		['320px', 320]
	] as const)('gives the link a hit target of 44px or more at %s', async (_, width) => {
		if (width) await page.viewport(width, 640);
		await setup();

		const link = page.getByRole('link', { name: 'Privacy Policy (opens in new tab)' });
		await expect.element(link).toBeVisible();
		expect(link.element().getBoundingClientRect().height).toBeGreaterThanOrEqual(44);
	});

	// ADR-0053: the link is a notice. Nothing here asks her to agree.
	it('asks for no agreement', async () => {
		await setup();

		await expect.element(page.getByRole('checkbox')).not.toBeInTheDocument();
		await expect.element(page.getByRole('button')).not.toBeInTheDocument();
		await expect.element(page.getByText(/agree/i)).not.toBeInTheDocument();
	});
});
