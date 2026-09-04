import { page } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { toApiResponder, toPageState } from '../../../../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * The `page` this route reads comes from its own fixture (#596), so what
 * this spec renders and what the continuum sweep measures are one
 * description. `vi.mock` is hoisted above every import, so `pageState` is
 * declared empty and filled from the fixture once the imports have run --
 * the route reads `page.params.engagementId` inside its own functions
 * rather than at module scope, so the later write is seen. Same
 * installation, through the same `toPageState`, as
 * `route-continuum.svelte.spec.ts`.
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

// registerPushSubscription/unregisterPushSubscription touch the real
// Service Worker/PushManager browser APIs (both are `v8 ignore`d in
// pushRegistration.ts for exactly that reason) -- mocked here so this
// spec proves what the route does with the durable preference, not
// whether a headless browser happens to grant a subscription.
const registerPushSubscription = vi.hoisted(() => vi.fn(() => Promise.resolve()));
const unregisterPushSubscription = vi.hoisted(() => vi.fn(() => Promise.resolve()));
vi.mock('#lib/pushRegistration.js', () => ({
	registerPushSubscription,
	unregisterPushSubscription,
	portalPushSubscriptionsPath: (engagementId: string) =>
		`/api/portal/engagements/${engagementId}/push-subscriptions`,
	portalNotificationPreferencePath: (engagementId: string) =>
		`/api/portal/engagements/${engagementId}/notification-preference`
}));

function jsonResponse(body: unknown, status = 200): Response {
	return { ok: status < 400, status, json: () => Promise.resolve(body), text: () => Promise.resolve(JSON.stringify(body)) } as Response;
}

function refusal(status: number, message: string): Response {
	return { ok: false, status, text: () => Promise.resolve(message) } as Response;
}

interface MockOptions {
	loadResponse?: Response;
	putResponse?: Response;
}

function mockApi({ loadResponse = jsonResponse({ enabled: false }), putResponse }: MockOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (init?.method === 'PUT') return Promise.resolve(putResponse ?? loadResponse);
		return Promise.resolve(loadResponse);
	});
}

const toggleButton = (label: string) => page.getByRole('button', { name: label });

const { engagementId } = fixture.params;

beforeEach(() => {
	apiFetchWithSession.mockReset();
	registerPushSubscription.mockClear();
	unregisterPushSubscription.mockClear();
});

describe('the Client portal Notifications settings screen', () => {
	it('explains what a notification is and is not, before any action', async () => {
		// This screen's happy path (GET only, no toggle clicked) is exactly
		// the fixture's own response.
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
		await render(Page, {});

		await expect
			.element(page.getByText(/never shows who sent it or what the message says/))
			.toBeVisible();
		await expect.element(page.getByText(/not a phone call/)).toBeVisible();
	});

	it('reports notifications as off when the Client has never decided', async () => {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
		await render(Page, {});

		await expect.element(page.getByText('Notifications are currently off.')).toBeVisible();
		await expect.element(toggleButton('Turn on notifications')).toBeVisible();
	});

	it('reports notifications as on when she has already turned them on', async () => {
		mockApi({ loadResponse: jsonResponse({ enabled: true }) });
		await render(Page, {});

		await expect
			.element(page.getByText('Notifications are currently on for this device.'))
			.toBeVisible();
		await expect.element(toggleButton('Turn off notifications')).toBeVisible();
	});

	// #303 AC3/AC4: turning it on persists the choice (so the next mount
	// registers) and only then asks the browser to subscribe -- the
	// explanation above has already been read by the time this fires.
	it('turns notifications on: persists the choice, then registers this device', async () => {
		mockApi({ loadResponse: jsonResponse({ enabled: false }), putResponse: jsonResponse({ enabled: true }) });
		await render(Page, {});

		await toggleButton('Turn on notifications').click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(`/api/portal/engagements/${engagementId}/notification-preference`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ enabled: true })
		});
		expect(registerPushSubscription).toHaveBeenCalledWith(
			`/api/portal/engagements/${engagementId}/push-subscriptions`,
			apiFetchWithSession
		);
		expect(unregisterPushSubscription).not.toHaveBeenCalled();
		await expect
			.element(page.getByRole('status'))
			.toHaveTextContent('Notifications are on for this device.');
	});

	// #303 AC3/AC4: turning it off persists the mute first (the send-path
	// filter reads that row regardless of any subscription's fate), then
	// takes this device off push.
	it('turns notifications off: persists the choice, then unregisters this device', async () => {
		mockApi({ loadResponse: jsonResponse({ enabled: true }), putResponse: jsonResponse({ enabled: false }) });
		await render(Page, {});

		await toggleButton('Turn off notifications').click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(`/api/portal/engagements/${engagementId}/notification-preference`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ enabled: false })
		});
		expect(unregisterPushSubscription).toHaveBeenCalledWith(
			`/api/portal/engagements/${engagementId}/push-subscriptions`,
			apiFetchWithSession
		);
		expect(registerPushSubscription).not.toHaveBeenCalled();
		await expect.element(page.getByRole('status')).toHaveTextContent('Notifications are off.');
	});

	it("shows the server's own words when the toggle is refused", async () => {
		mockApi({ loadResponse: jsonResponse({ enabled: false }), putResponse: refusal(429, 'too many requests -- try again later') });
		await render(Page, {});

		await toggleButton('Turn on notifications').click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('too many requests -- try again later');
	});
});
