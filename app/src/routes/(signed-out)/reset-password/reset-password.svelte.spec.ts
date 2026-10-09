import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../routeFixture.js';
import { fixture } from './page.fixture.js';

// The `page` this screen reads comes from the route's own fixture, through
// the same `toPageState` the continuum sweep installs (#596).
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

vi.mock('#lib/api.js', () => ({ apiBaseURL: () => '' }));

const globalFetch = vi.hoisted(() => vi.fn());

beforeEach(() => {
	vi.stubGlobal('fetch', globalFetch);
	globalFetch.mockReset();
});

const passwordField = () => testPage.getByLabelText('New password');
const submit = () => testPage.getByRole('button', { name: 'Reset password' }).click();

describe('the password rule on the reset screen (#1538)', () => {
	it('states the rule before she types', async () => {
		await render(Page, {});

		await expect.element(testPage.getByText('Must be 15 characters or more')).toBeVisible();
	});

	it('refuses 14 characters before anything is sent', async () => {
		await render(Page, {});
		await passwordField().fill('a'.repeat(14));
		await submit();

		await expect
			.element(testPage.getByRole('link', { name: 'Password must be 15 characters or more' }))
			.toBeVisible();
		expect(globalFetch).not.toHaveBeenCalled();
	});

	it('sends 64 characters with spaces in them', async () => {
		const long = 'correct horse battery staple '.repeat(3).slice(0, 64);
		globalFetch.mockResolvedValueOnce(jsonResponse({}, 204));
		await render(Page, {});
		await passwordField().fill(long);
		await submit();

		await expect.element(testPage.getByText(/Your password has been reset/)).toBeVisible();
		const [, init] = globalFetch.mock.calls[0];
		expect(JSON.parse(init.body).newPassword).toBe(long);
	});
});
