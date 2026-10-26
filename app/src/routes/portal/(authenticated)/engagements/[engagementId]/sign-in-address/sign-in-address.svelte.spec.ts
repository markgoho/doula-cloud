import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { gotoWithOutcome } from '#lib/outcome.js';
import { jsonResponse } from '#lib/testResponse.js';
import { toApiResponder, toPageState } from '../../../../../routeFixture.js';
import { fixture, session } from './page.fixture.js';

/*
 * #619's address, and #618's sign out everywhere (#2045). The `page` this route reads and the answer its
 * own fetch gets both come from its fixture (#596), so what this spec
 * renders and what the continuum sweep measures are one description.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const goto = vi.hoisted(() => vi.fn());
const refreshAll = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto, refreshAll }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiFetch: vi.fn()
}));

// Push unregister is #618's own borrowing of signOut.ts's bounded
// best-effort pattern -- mocked rather than exercised, the same as the
// portal layout's own sign-out spec: it is fire-and-forget by design,
// and the Service Worker/Push APIs it drives have nothing to do with
// this screen's own behavior.
const unregisterPushSubscription = vi.hoisted(() => vi.fn().mockResolvedValue(undefined));
vi.mock('#lib/pushRegistration.js', () => ({
	unregisterPushSubscription,
	portalPushSubscriptionsPath: (engagementId: string) =>
		`/api/portal/engagements/${engagementId}/push-subscriptions`
}));
vi.mock('#lib/signOut.js', () => ({
	bestEffort: (work: () => Promise<void>) => work(),
	UNREGISTER_TIMEOUT_MS: 3000
}));

// The address the fixture says signs her in today, so the spec asserts
// on the same screen the sweep measures.
const currentAddress = session.signInAddress;
const happyPath = toApiResponder(fixture);

interface SetupOptions {
	/**
	What GET /api/portal/session answers -- the fixture's own by default.
	*/
	load?: Response | Error;
	/**
	What DELETE /api/portal/sessions (#618's sign-out-everywhere) answers.
	*/
	signOutEverywhere?: Response | Error;
}

async function setup({
	load,
	signOutEverywhere = jsonResponse('', 204)
}: SetupOptions = {}) {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	refreshAll.mockReset();
	unregisterPushSubscription.mockClear();
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (init?.method === 'DELETE') {
			return signOutEverywhere instanceof Error
				? Promise.reject(signOutEverywhere)
				: Promise.resolve(signOutEverywhere);
		}
		if (load === undefined) return happyPath(path);
		return load instanceof Error ? Promise.reject(load) : Promise.resolve(load);
	});
	await render(Page);
}

const signOutEverywhereButton = () => page.getByRole('button', { name: 'Sign out of every device' });
const signOutEverywhereDialog = () =>
	page.getByRole('dialog', { name: 'Sign out of every device' });

// #2045: a summary, not a form. The address is a row whose Change opens a
// question page; nothing on this page is typed into.
describe('the Client-portal sign-in address screen', () => {
	it('shows the address that signs her in today, with a Change to its question page', async () => {
		await setup();

		await expect.element(page.getByText(currentAddress)).toBeVisible();
		const change = page.getByRole('link', { name: 'Change' });
		await expect
			.element(change)
			.toHaveAttribute('href', '/portal/engagements/engagement-1/sign-in-address/change');
		await expect.element(change).toHaveAccessibleDescription('sign-in address');
		expect(page.getByRole('textbox').elements()).toHaveLength(0);
	});

	it('says the address is her own, apart from what her Practice holds', async () => {
		await setup();

		await expect.element(page.getByText(/separate from the contact details your Practice holds/)).toBeVisible();
	});

	it('reports the page failing to load without pretending there is an address', async () => {
		await setup({ load: jsonResponse('', 500) });

		await expect.element(page.getByText(/problem with the service/)).toBeVisible();
		expect(page.getByRole('link', { name: 'Change' }).elements()).toHaveLength(0);
	});

	it('reports a thrown load the same way', async () => {
		await setup({ load: new Error('offline') });

		await expect.element(page.getByText(/problem with the service/)).toBeVisible();
	});

	// #1710: the question page sends the link and comes back here, and this
	// page says what it did, before its title and focused.
	it('says the link was sent when the question page sends her back', async () => {
		goto.mockImplementationOnce(async () => {
			await setup();
		});

		await gotoWithOutcome(fixture.url, 'Check your email.');

		const message = page.getByRole('status').filter({ hasText: 'Check your email.' });
		await expect.element(message).toBeVisible();
		await expect.element(message).toHaveFocus();
	});
});

// #618, ADR-0026: her own "sign out everywhere", its own last section on
// the same screen, apart from the address row (#2045).
describe('the Client-portal sign-out-everywhere control', () => {
	it('warns that it ends every session, this one included', async () => {
		await setup();

		await expect
			.element(page.getByRole('heading', { level: 2, name: 'Sign out of every device' }))
			.toBeVisible();
		await expect.element(page.getByText(/ends every session on every device, including this one/)).toBeVisible();
	});

	it('asks for confirmation before signing out of every device', async () => {
		await setup();

		await signOutEverywhereButton().click();

		await expect.element(signOutEverywhereDialog()).toBeVisible();
		expect(apiFetchWithSession).not.toHaveBeenCalledWith(
			'/api/portal/sessions',
			expect.anything()
		);
	});

	it('takes this device off push, signs out of every device, and returns to the login screen', async () => {
		await setup();

		await signOutEverywhereButton().click();
		await signOutEverywhereDialog().getByRole('button', { name: 'Sign out of every device' }).click();

		expect(unregisterPushSubscription).toHaveBeenCalledWith(
			'/api/portal/engagements/engagement-1/push-subscriptions',
			expect.anything()
		);
		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/portal/sessions', { method: 'DELETE' });
		// #487: a Back press to this exact URL must not reuse the
		// still-signed-in load result instead of re-checking the session.
		expect(refreshAll).toHaveBeenCalled();
		expect(goto).toHaveBeenCalledWith('/portal/login');
	});

	it('reports a failed sign-out-everywhere in place, without navigating away', async () => {
		await setup({ signOutEverywhere: jsonResponse('Sign-out failed', 500) });

		await signOutEverywhereButton().click();
		await signOutEverywhereDialog().getByRole('button', { name: 'Sign out of every device' }).click();

		await expect.element(page.getByText('Sign-out failed')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('reports a thrown sign-out-everywhere in place', async () => {
		await setup({ signOutEverywhere: new Error('offline') });

		await signOutEverywhereButton().click();
		await signOutEverywhereDialog().getByRole('button', { name: 'Sign out of every device' }).click();

		await expect.element(page.getByText('offline')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
});
