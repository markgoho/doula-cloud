import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { RootLanding } from './+page.js';
import Page from './+page.svelte';

describe('/+page.svelte', () => {
	it('offers a signed-out visitor the three real entry points, none implied as the main one', async () => {
		const data: RootLanding = { type: 'signed-out' };
		await render(Page, { params: {}, data });

		await expect.element(testPage.getByRole('link', { name: 'Staff log in' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Set up a Practice' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Client portal log in' })).toBeVisible();
		await expect.element(testPage.getByText('your session ended', { exact: false })).not.toBeInTheDocument();
	});

	it("lists a signed-in Staff visitor's several Practices when there is no single or last-used one", async () => {
		const data: RootLanding = {
			type: 'staff-picker',
			memberships: [
				{ practiceId: 'practice-1', practiceName: 'Riverside Doulas', roles: ['owner'] },
				{ practiceId: 'practice-2', practiceName: 'Hilltop Doulas', roles: ['doula'] }
			]
		};
		await render(Page, { params: {}, data });

		const link = testPage.getByRole('link', { name: 'Riverside Doulas' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/practices/practice-1');
		await expect.element(testPage.getByRole('link', { name: 'Hilltop Doulas' })).toBeVisible();
	});

	it('tells a Staff visitor with no Practice yet to ask an Owner, rather than showing an empty list', async () => {
		const data: RootLanding = { type: 'staff-picker', memberships: [] };
		await render(Page, { params: {}, data });

		await expect
			.element(testPage.getByText("You don't belong to any Practice yet. Ask an Owner to invite you."))
			.toBeVisible();
	});

	it("lists a signed-in Client-portal visitor's several Engagements", async () => {
		const data: RootLanding = {
			type: 'portal-picker',
			engagements: [
				{ engagementId: 'engagement-1', practiceName: 'Riverside Doulas', status: 'active' },
				{ engagementId: 'engagement-2', practiceName: 'Hilltop Doulas', status: 'active' }
			]
		};
		await render(Page, { params: {}, data });

		const link = testPage.getByRole('link', { name: 'Riverside Doulas' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/portal/engagements/engagement-1');
		await expect.element(testPage.getByRole('link', { name: 'Hilltop Doulas' })).toBeVisible();
	});

	it('tells a Client-portal visitor with no Engagement yet to ask her Practice, rather than showing an empty list', async () => {
		const data: RootLanding = { type: 'portal-picker', engagements: [] };
		await render(Page, { params: {}, data });

		await expect
			.element(testPage.getByText("You don't have an Engagement yet. Ask your Practice to set one up."))
			.toBeVisible();
	});
});
