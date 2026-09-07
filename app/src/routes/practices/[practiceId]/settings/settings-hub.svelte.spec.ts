import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * The `page` this route reads comes from its own fixture (#596), so the
 * params this spec installs and the params the continuum sweep installs
 * are one description.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

async function setup(roles: string[] = []) {
	pageState.data = {
		session: { practiceId: 'practice-1', practiceName: 'Riverside Doula Collective', roles, isContractor: false }
	};
	await render(Page, {});
}

describe('the Settings hub', () => {
	it('lists every settings screen but MFA for a non-Owner', async () => {
		await setup(['doula']);

		await expect.element(testPage.getByRole('link', { name: 'Payments' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Website' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Client Fields' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Plan Templates' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Contract Template' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Multi-factor authentication' }))
			.not.toBeInTheDocument();
		await expect
			.element(testPage.getByRole('link', { name: 'Blocked email addresses' }))
			.not.toBeInTheDocument();
		await expect
			.element(testPage.getByRole('link', { name: "Export this Practice's data" }))
			.not.toBeInTheDocument();
	});

	/*
	 * #744: the two gated entries are gated differently. An Admin runs the
	 * work -- an address that bounced can receive neither the invite, the
	 * Contract nor the payment notice until somebody lifts it -- while who
	 * is at the Practice at all stays the Owner's.
	 */
	it('gives an Admin the blocked-addresses entry but not the Owner-only MFA one', async () => {
		await setup(['admin']);

		const link = testPage.getByRole('link', { name: 'Blocked email addresses' });
		await expect.element(link).toBeVisible();
		await expect
			.element(link)
			.toHaveAttribute('href', '/practices/practice-1/settings/blocked-addresses');
		await expect
			.element(testPage.getByRole('link', { name: 'Multi-factor authentication' }))
			.not.toBeInTheDocument();
	});

	it("adds the Owner-only MFA entry, linking to the switch's own screen", async () => {
		await setup(['owner']);

		const link = testPage.getByRole('link', { name: 'Multi-factor authentication' });
		await expect.element(link).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Blocked email addresses' }))
			.toBeVisible();
		await expect
			.element(link)
			.toHaveAttribute('href', '/practices/practice-1/settings/mfa');
	});

	// #288: the export control is Owner-only, the same seat as erasure --
	// a whole-Practice export crosses every attachment and role boundary
	// ADR-0008 draws, so it does not widen to Admin the way blocked
	// addresses does.
	it('adds the Owner-only export entry, linking to the BFF download route', async () => {
		await setup(['owner']);

		const link = testPage.getByRole('link', { name: "Export this Practice's data" });
		await expect.element(link).toBeVisible();
		await expect.element(link).toHaveAttribute('href', '/api/practices/practice-1/export');
	});

	it('withholds the export entry from an Admin', async () => {
		await setup(['admin']);

		await expect
			.element(testPage.getByRole('link', { name: "Export this Practice's data" }))
			.not.toBeInTheDocument();
	});
});
