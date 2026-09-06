import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * #610 on the invitation: ADR-0026 makes it the first magic link, so
 * this Continue is a Client sign-in and can evict a live Practice
 * session exactly as a later one can. The BFF refuses the first press
 * and says what it costs; the page shows it on the button.
 */

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiBaseURL: () => '', apiFetchWithSession }));

// The invitation token arrives in the link, so this route reads it off
// `page.url` -- installed from the fixture's own URL rather than a second
// one written here, so the spec and the continuum sweep describe one
// screen.
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const WARNING = 'Continuing signs you out of your Practice in this browser.';

beforeEach(() => {
	goto.mockReset();
	apiFetchWithSession.mockReset();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

/*
 * Presses Continue once and gets the refusal, handing back the fetch
 * mock so a test can count the presses.
 *
 * The mock stands in for the BFF rather than answering blindly: it
 * refuses without `X-Confirmed` and claims the invitation with it,
 * exactly as `portalinvite.AcceptInviteHandler` does. So "she lands" is
 * only reachable if the page actually sent the confirmation.
 */
async function reachWarning() {
	const accept = vi.fn(async (_url: string, init: RequestInit) =>
		(init.headers as Record<string, string>)['X-Confirmed'] === 'true'
			? jsonResponse({ clientId: 'client-1' })
			: jsonResponse({ code: 'SESSION_EVICTION_UNCONFIRMED', message: WARNING }, 409)
	);
	vi.stubGlobal('fetch', accept);

	await render(Page, {});
	await testPage.getByRole('button', { name: 'Continue' }).click();

	await expect.element(testPage.getByText(WARNING)).toBeVisible();
	return accept;
}

describe('Accepting a portal invitation over a live Staff session (#610)', () => {
	it('warns on the button instead of claiming the invitation', async () => {
		const accept = await reachWarning();

		await expect
			.element(testPage.getByRole('button', { name: 'Continue and sign out' }))
			.toBeVisible();
		// Refused, not failed: the refusal rolled back everything the
		// handler wrote, so nothing went on to read a session it never
		// minted.
		expect(apiFetchWithSession).not.toHaveBeenCalled();
		expect(accept).toHaveBeenCalledTimes(1);
	});

	it('sends the same accept again, confirmed, when she presses through', async () => {
		const accept = await reachWarning();
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({
				engagements: [{ engagementId: 'engagement-1', practiceName: 'Bright Beginnings' }]
			})
		);

		await testPage.getByRole('button', { name: 'Continue and sign out' }).click();

		// The stand-in BFF claims the invitation only for a confirmed
		// press, so landing here is itself the proof it carried one.
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/portal/engagements/engagement-1'));
		expect(accept).toHaveBeenCalledTimes(2);
	});

	it('reads an ordinary refusal as an error, not as something to press through', async () => {
		vi.stubGlobal(
			'fetch',
			vi
				.fn()
				.mockResolvedValue(
					jsonResponse({ code: 'CONFLICT', message: 'a portal account already exists for this address' }, 409)
				)
		);

		await render(Page, {});
		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect
			.element(testPage.getByText('a portal account already exists for this address'))
			.toBeVisible();
		expect(testPage.getByRole('button', { name: 'Continue and sign out' }).elements()).toHaveLength(
			0
		);
	});
});
