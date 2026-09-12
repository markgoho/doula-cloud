import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toApiResponder, toPageState } from '../../../routeFixture.js';
import { afterSessionEnded, fixture, session } from './page.fixture.js';

/*
 * The screen reads its own URL for #757's `sessionEnded` flag, through
 * `#lib/appState.svelte.js` -- which reads `$app/state` rather than
 * replacing it, so mocking the source here is what reaches it. Declared
 * empty because `vi.mock` is hoisted above every import, then filled from
 * the fixture. A test that needs the ended-session address installs the
 * fixture's own variant over it, before `render()`.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

// #283: on load, this screen probes for a live Client-portal session of
// its own and, if one exists, sends the visitor on exactly the way a
// fresh sign-in would -- reusing decidePortalLanding rather than a
// second copy of that decision. These tests cover only that on-load
// probe; the sign-in form's own submit flow is unchanged and untouched
// here.

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

/*
 * This screen calls `probeSession` directly rather than
 * `apiFetchWithSession` (#lib/api.js's own doc comment on why), so the
 * mock is one level lower, on `apiFetch`, with `probeSession` mirrored
 * here to close over it -- the same mirroring `route-continuum.svelte.
 * spec.ts` does for this route. That is what lets `toApiResponder(fixture)`
 * answer this route's fetch the same way it answers every other route's.
 */
const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiBaseURL: () => '',
	apiFetchWithSession: vi.fn(),
	probeSession: async <Session,>(path: string): Promise<Session | undefined> => {
		try {
			const response = await apiFetch(path);
			if (!response.ok) return undefined;
			return (await response.json()) as Session;
		} catch {
			return undefined;
		}
	}
}));

beforeEach(() => {
	goto.mockReset();
	apiFetch.mockReset();
	// Back to the base fixture's own address, so a test that installed
	// #757's ended-session variant does not leave it on the next one.
	Object.assign(pageState, toPageState(fixture));
});

afterEach(() => {
	vi.unstubAllGlobals();
});

const [firstEngagement, secondEngagement] = session.engagements;

async function setup() {
	await render(Page, {});
}

describe('Client-portal login -- on-load session probe (#283)', () => {
	it('redirects a signed-in visitor to her only Engagement, without showing the form', async () => {
		// One Engagement rather than the fixture's two, so this is a
		// departure from it -- written as a spread rather than a fresh
		// object that re-states the Engagement fields it shares.
		apiFetch.mockResolvedValue(jsonResponse({ ...session, engagements: [firstEngagement] }));

		await setup();

		await vi.waitFor(() =>
			expect(goto).toHaveBeenCalledWith(`/portal/engagements/${firstEngagement.engagementId}`)
		);
		expect(apiFetch).toHaveBeenCalledWith('/api/portal/session');
	});

	// #312: the picker used to render inline, here, on the sign-in screen --
	// not an address she could return to. It now lives at the app root,
	// which already probes both populations and renders the same list, so
	// a signed-in visitor with several Engagements is sent there instead.
	it("sends a signed-in visitor with several Engagements to the portal root, without rendering a picker of its own", async () => {
		// The fixture's own session already has two Engagements -- it is
		// this test's happy path, not a reason to invent a second one.
		apiFetch.mockImplementation(toApiResponder(fixture));

		await setup();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/'));
		expect(testPage.getByRole('link', { name: secondEngagement.practiceName }).elements()).toHaveLength(0);
	});

	it('renders the ordinary login form for a signed-out visitor, with no session-ended messaging', async () => {
		apiFetch.mockResolvedValue(jsonResponse('no matching portal session', 404));

		await setup();

		await expect.element(testPage.getByLabelText('Email')).toBeVisible();
		expect(testPage.getByText(/session/i).elements()).toHaveLength(0);
		expect(goto).not.toHaveBeenCalled();
	});

	it('falls back to the ordinary form when the probe reports no session, network failure included', async () => {
		// probeSession itself swallows a thrown fetch and every non-OK
		// response (see its own tests in api.spec.ts); from this screen's
		// side, that failure is indistinguishable from "not signed in".
		apiFetch.mockResolvedValue(jsonResponse('no matching portal session', 404));

		await setup();

		await expect
			.element(testPage.getByRole('button', { name: 'Send me a sign-in link' }))
			.toBeVisible();
	});

	it('never probes the Staff session', async () => {
		apiFetch.mockResolvedValue(jsonResponse('no matching portal session', 404));

		await setup();

		await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(1));
		expect(apiFetch).not.toHaveBeenCalledWith('/api/staff/session');
	});
});

/*
 * #757: `handleExpiredSession` (#lib/api.js) sends an ended Client session
 * to this screen carrying `sessionEnded=true`, exactly as it does the
 * Staff one, so this screen answers "why am I here?" for the same reason.
 * The wording differs because what she does next does: a Client has no
 * password to re-enter (#617), only a link to ask for.
 *
 * The flag is the whole variable, so it is `setup`'s one parameter, and
 * the fixture's own ended-session variant is where its URL is written
 * rather than a second copy of that address in this file. At module scope
 * because it closes over nothing the describe owns.
 */
async function setupSessionEnded({ hasSessionEnded = true } = {}) {
	apiFetch.mockResolvedValue(jsonResponse('no matching portal session', 404));
	if (hasSessionEnded) Object.assign(pageState, toPageState({ ...fixture, ...afterSessionEnded }));

	await render(Page, {});
}

describe('Client-portal login -- the session-ended notice (#757)', () => {
	const NOTICE = 'For your security, we signed you out. Ask for a new sign-in link to continue.';

	it('says why she is back here when the URL carries the flag', async () => {
		await setupSessionEnded();

		await expect.element(testPage.getByText(NOTICE)).toBeVisible();
		await expect.element(testPage.getByLabelText('Email')).toBeVisible();
	});

	it('says nothing on an ordinary visit', async () => {
		await setupSessionEnded({ hasSessionEnded: false });

		await expect
			.element(testPage.getByRole('button', { name: 'Send me a sign-in link' }))
			.toBeVisible();
		expect(testPage.getByText(NOTICE).elements()).toHaveLength(0);
	});

	it('steps aside once she has asked for a link, rather than stacking two notices', async () => {
		vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({}, 200)));

		await setupSessionEnded();
		await testPage.getByLabelText('Email').fill('priya@example.com');
		await testPage.getByRole('button', { name: 'Send me a sign-in link' }).click();

		await expect.element(testPage.getByText(/we have sent a sign-in link/i)).toBeVisible();
		expect(testPage.getByText(NOTICE).elements()).toHaveLength(0);
	});
});
