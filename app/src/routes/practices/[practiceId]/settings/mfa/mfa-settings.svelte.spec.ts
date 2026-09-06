import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { fixture, impact as fixtureImpact } from './page.fixture.js';

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

interface SetupOptions {
	roles?: string[];
	impact?: { required: boolean; withoutSecondFactor: number };
	impactOk?: boolean;
	putResponse?: () => Response;
}

/*
 * One setup per test. It returns `puts` -- every body/headers pair the
 * screen sent to the PUT -- because "did she confirm, and what did she
 * ask for?" is the behaviour this screen exists to produce. The Membership
 * (roles) comes off page.data.session (#835), set here rather than fetched.
 */
async function setup({ roles = ['owner'], impact = fixtureImpact, impactOk = true, putResponse }: SetupOptions = {}) {
	pageState.data = {
		session: { practiceId: 'practice-1', practiceName: 'Riverside Doula Collective', roles, isContractor: false }
	};
	const puts: { body: Record<string, unknown>; headers: Record<string, string> }[] = [];
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (init?.method === 'PUT') {
			puts.push({
				body: JSON.parse(String(init.body)) as Record<string, unknown>,
				headers: init.headers as Record<string, string>
			});
			return Promise.resolve(putResponse ? putResponse() : jsonResponse(undefined, 204));
		}
		return Promise.resolve(jsonResponse(impact, impactOk ? 200 : 500));
	});
	await render(Page, {});
	return { puts };
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

describe('MFA settings screen', () => {
	it("shows the switch's current state, the affected count, and a button to require it for an Owner", async () => {
		await setup();

		await expect.element(testPage.getByText('Mandatory for Owners, optional for other Staff')).toBeVisible();
		await expect
			.element(testPage.getByText('6 Staff members currently have no second factor set up.'))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Require MFA for all Staff' })).toBeVisible();
	});

	it('hides the setting entirely from a non-Owner, and never asks the Owner-only endpoint for it', async () => {
		await setup({ roles: ['doula'] });

		await expect
			.element(testPage.getByText('Only a Practice Owner can view or change this setting.'))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Require MFA for all Staff' })).not.toBeInTheDocument();
		expect(apiFetchWithSession).not.toHaveBeenCalledWith(expect.stringContaining('/mfa-required/impact'));
	});

	it('shows the count in the confirmation before requiring MFA for all Staff, then confirms with X-Confirmed', async () => {
		const { puts } = await setup();

		await testPage.getByRole('button', { name: 'Require MFA for all Staff' }).click();
		const dialog = testPage.getByRole('dialog', { name: 'Require a second factor for every Staff member' });
		await expect
			.element(
				dialog.getByText(
					'6 Staff members have no second factor, and will not be able to sign in to this Practice until they set one up.'
				)
			)
			.toBeVisible();

		await dialog.getByRole('button', { name: 'Require MFA for all Staff' }).click();

		expect(puts).toEqual([
			{ body: { required: true }, headers: { 'Content-Type': 'application/json', 'X-Confirmed': 'true' } }
		]);
		await expect
			.element(testPage.getByText('Every Staff member must now sign in with a second factor.'))
			.toBeVisible();
	});

	it('tells her nobody will be signed out when every Staff member already has a second factor', async () => {
		await setup({ impact: { required: false, withoutSecondFactor: 0 } });

		await testPage.getByRole('button', { name: 'Require MFA for all Staff' }).click();
		const dialog = testPage.getByRole('dialog', { name: 'Require a second factor for every Staff member' });
		await expect
			.element(dialog.getByText('Every Staff member already has a second factor set up, so nobody will be signed out.'))
			.toBeVisible();
	});

	it('uses singular wording for one affected Staff member', async () => {
		await setup({ impact: { required: false, withoutSecondFactor: 1 } });

		await expect.element(testPage.getByText('1 Staff member currently has no second factor set up.')).toBeVisible();

		await testPage.getByRole('button', { name: 'Require MFA for all Staff' }).click();
		const dialog = testPage.getByRole('dialog', { name: 'Require a second factor for every Staff member' });
		await expect
			.element(
				dialog.getByText(
					'1 Staff member has no second factor, and will not be able to sign in to this Practice until they set one up.'
				)
			)
			.toBeVisible();
	});

	it('shows an error and leaves the switch off when the confirmed request is refused', async () => {
		const { puts } = await setup({
			putResponse: () => jsonResponse({ message: 'this action requires confirmation' }, 400)
		});

		await testPage.getByRole('button', { name: 'Require MFA for all Staff' }).click();
		const dialog = testPage.getByRole('dialog', { name: 'Require a second factor for every Staff member' });
		await dialog.getByRole('button', { name: 'Require MFA for all Staff' }).click();

		expect(puts).toHaveLength(1);
		await expect.element(testPage.getByText('this action requires confirmation')).toBeVisible();
		await expect.element(testPage.getByText('Mandatory for Owners, optional for other Staff')).toBeVisible();
	});

	it('stops requiring MFA for all Staff immediately, with no confirmation step', async () => {
		const { puts } = await setup({ impact: { required: true, withoutSecondFactor: 2 } });

		await expect.element(testPage.getByText('Mandatory for every Staff member')).toBeVisible();
		await testPage.getByRole('button', { name: 'Stop requiring MFA for all Staff' }).click();

		expect(puts).toEqual([
			{ body: { required: false }, headers: { 'Content-Type': 'application/json', 'X-Confirmed': 'true' } }
		]);
		await expect
			.element(testPage.getByText('Staff without a second factor can sign in without one again.'))
			.toBeVisible();
	});

	it('shows a load error when the impact endpoint cannot be read', async () => {
		await setup({ impactOk: false });

		await expect.element(testPage.getByText('There is a problem with the service. Try again in a few minutes.')).toBeVisible();
	});
});
