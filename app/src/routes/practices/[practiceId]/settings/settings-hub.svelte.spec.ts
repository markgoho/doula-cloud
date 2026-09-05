import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
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

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

async function setup(roles: string[] = []) {
	apiFetchWithSession.mockResolvedValue(jsonResponse({ roles }));
	await render(Page, {});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

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
});
