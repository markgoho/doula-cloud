import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toApiResponder } from '../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * #745: the screen for a live session that resolves to no Practice --
 * a signup that half-landed, or a Staff member whose last Membership was
 * removed. It replaces `no matching staff account`, which named an
 * internal lookup and offered nothing to do about it.
 */

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetch, apiBaseURL: () => '' }));
vi.mock('#lib/pushRegistration.js', () => ({ unregisterPushSubscription: vi.fn() }));

const signOutOfSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/signOut.js', () => ({ signOutOfSession }));

beforeEach(() => {
	for (const mock of [goto, apiFetch, signOutOfSession]) mock.mockReset();
	apiFetch.mockImplementation(toApiResponder(fixture));
});

describe('the no-Practice landing screen', () => {
	it('names the state and offers both ways on from it', async () => {
		await render(Page, {});

		await expect
			.element(testPage.getByRole('heading', { name: 'Your account is not part of a Practice' }))
			.toBeVisible();
		const setUp = testPage.getByRole('link', { name: 'Set up a Practice' });
		await expect.element(setUp).toBeVisible();
		expect(setUp.element()).toHaveAttribute('href', '/signup');
		await expect.element(testPage.getByText('Wait for an invitation', { exact: false })).toBeVisible();
	});

	// She holds a live session that leads nowhere, so the door out of it
	// has to be on the screen that says so.
	it('lets her sign out from here', async () => {
		signOutOfSession.mockResolvedValue({ ok: true });
		await render(Page, {});

		await testPage.getByRole('button', { name: 'Sign out' }).click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/login'));
	});

	it('sends a visitor with no session at all to the login screen instead', async () => {
		apiFetch.mockResolvedValue(jsonResponse('missing credential', 401));

		await render(Page, {});

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/login'));
	});

	// A stale tab, or a bookmark kept after an invitation was accepted.
	// The claim this screen makes is no longer true of her, so it does not
	// make it: `/` owns where a person with a Practice lands.
	it('sends a visitor who does have a Practice back to the landing decision', async () => {
		apiFetch.mockResolvedValue(
			jsonResponse({
				memberships: [{ practiceId: 'practice-1', practiceName: 'Riverside Doulas', roles: ['owner'] }]
			})
		);

		await render(Page, {});

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/'));
	});

	it('stays put when the probe cannot be made at all', async () => {
		apiFetch.mockRejectedValue(new Error('offline'));

		await render(Page, {});

		await expect
			.element(testPage.getByRole('heading', { name: 'Your account is not part of a Practice' }))
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
});
