import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

// #283: on load, this screen probes for a live Client-portal session of
// its own and, if one exists, sends the visitor on exactly the way a
// fresh sign-in would -- reusing decidePortalLanding rather than a
// second copy of that decision. These tests cover only that on-load
// probe; the sign-in form's own submit flow is unchanged and untouched
// here.

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

vi.mock('firebase/auth', () => ({
	signInWithEmailAndPassword: vi.fn(),
	signOut: vi.fn()
}));
vi.mock('#lib/firebase.js', () => ({ getFirebaseAuth: () => ({}) }));

const probeSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiBaseURL: () => '',
	apiFetchWithSession: vi.fn(),
	probeSession
}));

beforeEach(() => {
	goto.mockReset();
	probeSession.mockReset();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('Client-portal login -- on-load session probe (#283)', () => {
	it('redirects a signed-in visitor to her only Engagement, without showing the form', async () => {
		probeSession.mockResolvedValue({
			engagements: [{ engagementId: 'engagement-1', practiceName: 'Riverside Doulas', status: 'active' }]
		});

		await render(Page, {});

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/portal/engagements/engagement-1'));
		expect(probeSession).toHaveBeenCalledWith('/api/portal/session');
	});

	it('shows the Engagement picker for a signed-in visitor with several Engagements', async () => {
		probeSession.mockResolvedValue({
			engagements: [
				{ engagementId: 'engagement-1', practiceName: 'Riverside Doulas', status: 'active' },
				{ engagementId: 'engagement-2', practiceName: 'Hilltop Doulas', status: 'active' }
			]
		});

		await render(Page, {});

		await expect
			.element(testPage.getByRole('heading', { name: 'Choose an Engagement' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Hilltop Doulas' })).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('renders the ordinary login form for a signed-out visitor, with no session-ended messaging', async () => {
		probeSession.mockResolvedValue(undefined);

		await render(Page, {});

		await expect.element(testPage.getByLabelText('Email')).toBeVisible();
		expect(testPage.getByText(/session/i).elements()).toHaveLength(0);
		expect(goto).not.toHaveBeenCalled();
	});

	it('falls back to the ordinary form when the probe reports no session, network failure included', async () => {
		// probeSession itself swallows a thrown fetch and every non-OK
		// response (see its own tests in api.spec.ts); from this screen's
		// side, that failure is indistinguishable from "not signed in".
		probeSession.mockResolvedValue(undefined);

		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Log in' })).toBeVisible();
	});

	it('never probes the Staff session', async () => {
		probeSession.mockResolvedValue(undefined);

		await render(Page, {});

		await vi.waitFor(() => expect(probeSession).toHaveBeenCalledTimes(1));
		expect(probeSession).not.toHaveBeenCalledWith('/api/staff/session');
	});
});
