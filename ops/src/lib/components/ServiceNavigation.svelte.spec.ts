import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import '../../../../app/src/lib/styles/app.css';
import ServiceNavigation from './ServiceNavigation.svelte';

const operator = { name: 'Mark Goho', email: 'mark@example.com' };

const SECTIONS = [
	['Practices', '/practices'],
	['Monthly Credits', '/credits'],
	['Feedback', '/feedback'],
	['Reset two-factor authentication', '/staff'],
	['Pilot allowlist', '/pilot-allowlist'],
	['Pilot gate', '/pilot-gate'],
	['Legal versions', '/legal-versions'],
	['Operator log', '/operator-log']
] as const;

describe('ServiceNavigation', () => {
	it('links the service name home and every section, in order', async () => {
		await render(ServiceNavigation, { operator, pathname: '/', onsignedout: vi.fn() });

		await expect.element(page.getByRole('link', { name: 'DoulaCloud operations' })).toHaveAttribute('href', '/');
		const links = page.getByRole('navigation', { name: 'Operations' }).getByRole('link');
		await expect.poll(() => links.elements().map((link) => link.textContent)).toEqual(SECTIONS.map(([label]) => label));
		for (const [label, href] of SECTIONS) {
			await expect.element(page.getByRole('link', { name: label })).toHaveAttribute('href', href);
		}
	});

	it('marks no section current on the home page', async () => {
		await render(ServiceNavigation, { operator, pathname: '/', onsignedout: vi.fn() });

		await expect.element(page.getByRole('link', { name: 'Practices' })).not.toHaveAttribute('aria-current');
	});

	it('marks the section the page is in, and only that one', async () => {
		await render(ServiceNavigation, { operator, pathname: '/credits', onsignedout: vi.fn() });

		await expect.element(page.getByRole('link', { name: 'Monthly Credits' })).toHaveAttribute('aria-current', 'page');
		await expect.element(page.getByRole('link', { name: 'Practices' })).not.toHaveAttribute('aria-current');
	});

	it('keeps the section marked on a page inside it', async () => {
		await render(ServiceNavigation, { operator, pathname: '/practices/p-1/refund', onsignedout: vi.fn() });

		await expect.element(page.getByRole('link', { name: 'Practices' })).toHaveAttribute('aria-current', 'page');
	});

	it('does not mistake a longer first segment for a section', async () => {
		await render(ServiceNavigation, { operator, pathname: '/pilot-gates', onsignedout: vi.fn() });

		await expect.element(page.getByRole('link', { name: 'Pilot gate' })).not.toHaveAttribute('aria-current');
	});

	it('shows no section and no sign out when nobody is signed in', async () => {
		await render(ServiceNavigation, { operator: undefined, pathname: '/sign-in', onsignedout: vi.fn() });

		await expect.element(page.getByText('DoulaCloud operations')).toBeVisible();
		await expect.element(page.getByRole('navigation')).not.toBeInTheDocument();
		await expect.element(page.getByRole('link')).not.toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Sign out' })).not.toBeInTheDocument();
	});

	it('ends the session and hands back when Sign out is pressed', async () => {
		const onsignedout = vi.fn();
		const signOut = vi.fn<() => Promise<void>>().mockResolvedValue();
		await render(ServiceNavigation, { operator, pathname: '/', onsignedout, signOut });

		await page.getByRole('button', { name: 'Sign out' }).click();

		await vi.waitFor(() => expect(onsignedout).toHaveBeenCalledOnce());
		expect(signOut).toHaveBeenCalledOnce();
	});

	it('says so, and stays signed in, when ending the session fails', async () => {
		const onsignedout = vi.fn();
		const signOut = vi.fn<() => Promise<void>>().mockRejectedValue(new Error('down'));
		await render(ServiceNavigation, { operator, pathname: '/', onsignedout, signOut });

		await page.getByRole('button', { name: 'Sign out' }).click();

		await expect.element(page.getByText('Sorry, there is a problem with the service. Try again later.')).toBeVisible();
		expect(onsignedout).not.toHaveBeenCalled();
	});
});
