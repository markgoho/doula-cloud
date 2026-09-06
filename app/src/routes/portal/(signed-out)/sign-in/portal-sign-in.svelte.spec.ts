import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * #610: a browser holds exactly one Doula Cloud session, so redeeming a
 * sign-in link on the laptop where her Practice session is live ends
 * that one. The BFF refuses the first Continue and says what it costs;
 * this page shows it on the button rather than on a screen of its own.
 */

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiBaseURL: () => '', apiFetchWithSession }));

/*
 * The token arrives in the link, so this route reads it off `page.url` --
 * installed from the fixture's own URL rather than a second one written
 * here, so the spec and the continuum sweep describe one screen.
 */
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
 * refuses without `X-Confirmed` and redeems with it, exactly as
 * `clientauth.RedeemMagicLinkHandler` does. So "she lands" is only
 * reachable if the page actually sent the confirmation, and no test has
 * to reach into the mock's call arguments to prove it.
 */
async function reachWarning() {
	const redeem = vi.fn(async (_url: string, init: RequestInit) =>
		(init.headers as Record<string, string>)['X-Confirmed'] === 'true'
			? jsonResponse({ ok: true })
			: jsonResponse({ code: 'SESSION_EVICTION_UNCONFIRMED', message: WARNING }, 409)
	);
	vi.stubGlobal('fetch', redeem);

	await render(Page, {});
	await testPage.getByRole('button', { name: 'Continue' }).click();

	await expect.element(testPage.getByText(WARNING)).toBeVisible();
	return redeem;
}

describe('Redeeming a sign-in link over a live Staff session (#610)', () => {
	it('warns on the button instead of signing her in', async () => {
		const redeem = await reachWarning();

		await expect
			.element(testPage.getByRole('button', { name: 'Continue and sign out' }))
			.toBeVisible();
		// Refused, not failed: nothing read a session the refusal never
		// minted, and the link was not spent.
		expect(apiFetchWithSession).not.toHaveBeenCalled();
		expect(redeem).toHaveBeenCalledTimes(1);
	});

	it('sends the same redeem again, confirmed, when she presses through', async () => {
		const redeem = await reachWarning();
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({
				engagements: [{ engagementId: 'engagement-1', practiceName: 'Bright Beginnings' }]
			})
		);

		await testPage.getByRole('button', { name: 'Continue and sign out' }).click();

		// The stand-in BFF redeems only for a confirmed press, so landing
		// here is itself the proof that the second press carried it.
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/portal/engagements/engagement-1'));
		expect(redeem).toHaveBeenCalledTimes(2);
	});

	it('reads an ordinary refusal as an error, not as something to press through', async () => {
		vi.stubGlobal(
			'fetch',
			vi
				.fn()
				.mockResolvedValue(
					jsonResponse(
						{ code: 'INVALID_ARGUMENT', message: 'this link is invalid or has expired -- ask for a new one' },
						400
					)
				)
		);

		await render(Page, {});
		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect
			.element(testPage.getByText('this link is invalid or has expired -- ask for a new one'))
			.toBeVisible();
		expect(testPage.getByRole('button', { name: 'Continue and sign out' }).elements()).toHaveLength(
			0
		);
	});
});
