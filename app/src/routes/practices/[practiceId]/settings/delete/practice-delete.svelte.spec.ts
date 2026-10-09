import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { toApiResponder, toPageState, type RouteFixture } from '../../../../routeFixture.js';
import { fixture, ownerWithoutSecondFactor } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

async function setup(subject: RouteFixture = fixture) {
	Object.assign(pageState, toPageState(subject));
	apiFetchWithSession.mockImplementation(toApiResponder(subject));
	await render(Page, {});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

describe('Delete-this-Practice settings screen', () => {
	it('offers an Owner with a second factor the Delete button', async () => {
		await setup();

		await expect.element(testPage.getByRole('button', { name: 'Delete this Practice' })).toBeVisible();
	});

	// #1532: starting deletion needs a second factor. She is told so before
	// she tries, with the way to set one up, and enrollment brings her back
	// here.
	it('tells an Owner with no second factor that deleting needs one, in place of the button', async () => {
		await setup({ ...fixture, ...ownerWithoutSecondFactor });

		await expect
			.element(testPage.getByText('You need two-factor authentication before you can delete this Practice.'))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Set up two-factor authentication' }))
			.toHaveAttribute('href', `/mfa/enroll?returnTo=${encodeURIComponent('/practices/practice-1/settings/delete')}`);
		await expect.element(testPage.getByRole('button', { name: 'Delete this Practice' })).not.toBeInTheDocument();
	});
});
