import { page as testPage } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { RootLanding } from './+page.js';
import Page from './+page.svelte';
import { data as staffPickerData, fixture } from './page.fixture.js';

describe('/+page.svelte', () => {
	it('offers a signed-out visitor the three real entry points, none implied as the main one', async () => {
		const data: RootLanding = { type: 'signed-out' };
		await render(Page, { params: fixture.params, data });

		await expect.element(testPage.getByRole('link', { name: 'Staff log in' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Set up a Practice' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Client portal log in' })).toBeVisible();
		await expect.element(testPage.getByText('your session ended', { exact: false })).not.toBeInTheDocument();
	});

	it("lists a signed-in Staff visitor's several Practices when there is no single or last-used one", async () => {
		// The fixture's own Membership carries #530's URL; a second one is
		// added here rather than invented from scratch, since "several" is
		// content the fixture -- one Membership -- does not itself hold.
		const data: RootLanding = {
			...staffPickerData,
			memberships: [
				...staffPickerData.memberships,
				{ practiceId: 'practice-2', practiceName: 'Hilltop Doulas', roles: ['doula'] }
			]
		};
		await render(Page, { params: fixture.params, data });

		const [firstMembership] = staffPickerData.memberships;
		const link = testPage.getByRole('link', { name: firstMembership.practiceName });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', `/practices/${firstMembership.practiceId}`);
		await expect.element(testPage.getByRole('link', { name: 'Hilltop Doulas' })).toBeVisible();
	});

	// #745 removed this screen's own empty-picker branch: a Staff visitor
	// with no Membership never reaches it, because `+page.ts` redirects
	// her to `/no-practice` first. `root-load.spec.ts` covers that.

	// #312: the portal root lists every Engagement her Portal Account
	// reaches, across every Practice, past and present -- so this fixture
	// deliberately carries one `completed` Engagement, at a different
	// Practice from the `active` one, rather than two Engagements in the
	// same status.
	it("lists a signed-in Client-portal visitor's several Engagements, across Practices and statuses", async () => {
		const data: RootLanding = {
			type: 'portal-picker',
			engagements: [
				{
					engagementId: 'engagement-1',
					practiceName: 'Riverside Doulas',
					status: 'active',
					createdAt: '2026-01-15T20:00:00Z'
				},
				{
					engagementId: 'engagement-2',
					practiceName: 'Hilltop Doulas',
					status: 'completed',
					createdAt: '2026-03-12T20:00:00Z'
				}
			]
		};
		await render(Page, { params: fixture.params, data });

		// engagementLabel (#310): the Practice name plus when the Engagement
		// began, which is also what tells two Engagements at one Practice
		// apart -- see clientRegister.spec.ts for that case directly.
		const link = testPage.getByRole('link', { name: 'Riverside Doulas, started Jan 15, 2026' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/portal/engagements/engagement-1');
		await expect
			.element(testPage.getByRole('link', { name: 'Hilltop Doulas, started Mar 12, 2026' }))
			.toBeVisible();

		// The Client register's fixed labels (ADR-0015), not the raw
		// `active`/`completed` enum values -- a `completed` Engagement
		// stays listed and reads honestly as "Care ended" rather than
		// dropping off the list or reading as a raw status a Client has no
		// register entry for.
		await expect.element(testPage.getByText('Ongoing')).toBeVisible();
		await expect.element(testPage.getByText('Care ended')).toBeVisible();
	});

	it('tells a Client-portal visitor with no Engagement yet to ask her Practice, rather than showing an empty list', async () => {
		const data: RootLanding = { type: 'portal-picker', engagements: [] };
		await render(Page, { params: fixture.params, data });

		await expect
			.element(testPage.getByText("You don't have care set up yet. Ask your Practice to set it up."))
			.toBeVisible();
	});
});
