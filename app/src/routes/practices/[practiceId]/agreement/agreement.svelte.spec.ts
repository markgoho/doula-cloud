import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { toPageState, type RouteFixture } from '../../../routeFixture.js';
import { bothDue, fixture, privacyOnly, termsDue, withoutSecondFactor } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession, apiFetch, apiBaseURL: () => '' }));

const signOutOfSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/signOut.js', () => ({ signOutOfSession }));

async function setup(subject: RouteFixture | typeof bothDue = fixture) {
	Object.assign(pageState, toPageState({ ...fixture, ...subject }));
	await render(Page, {});
}

beforeEach(() => {
	goto.mockReset();
	apiFetchWithSession.mockReset();
	signOutOfSession.mockReset();
});

describe('The agreement screen (#1549)', () => {
	it('says what changed, from the version’s own record, with its date', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'The Terms of Service have changed' })).toBeVisible();
		await expect.element(testPage.getByText(`Terms of Service, from Nov 1, 2026: ${termsDue.change}`)).toBeVisible();
	});

	it('names both documents when both changed, and the Privacy Policy alone when only it did', async () => {
		await setup(bothDue);
		await expect
			.element(testPage.getByRole('heading', { level: 1, name: 'The Terms of Service and the Privacy Policy have changed' }))
			.toBeVisible();
		await expect.element(testPage.getByText(/^Privacy Policy, from/)).toBeVisible();
	});

	it('names the Privacy Policy alone when only it changed', async () => {
		await setup(privacyOnly);
		await expect.element(testPage.getByRole('heading', { level: 1, name: 'The Privacy Policy has changed' })).toBeVisible();
	});

	// GOV.UK's Links guidance: a link that opens a new tab says so in its
	// own text. Each links the version she agrees to, or the page for the
	// version in force when that document did not change.
	it('links each document in a new tab, saying so, at the version she agrees to', async () => {
		await setup();

		const terms = testPage.getByRole('link', { name: 'Terms of Service (opens in new tab)' });
		await expect.element(terms).toHaveAttribute('href', 'https://doula.cloud/terms/2026-11-01');
		await expect.element(terms).toHaveAttribute('target', '_blank');
		const privacy = testPage.getByRole('link', { name: 'Privacy Policy (opens in new tab)' });
		await expect.element(privacy).toHaveAttribute('href', 'https://doula.cloud/privacy');
		await expect.element(privacy).toHaveAttribute('target', '_blank');
	});

	it('joins the declaration to the button as its description, and offers no "Not now"', async () => {
		await setup();

		const button = testPage.getByRole('button', { name: 'Agree and continue' });
		await expect
			.element(button)
			.toHaveAccessibleDescription(
				'By continuing, you agree to the Terms of Service (opens in new tab) and the Privacy Policy (opens in new tab) for The Finger Lakes Birth and Postpartum Support Collective of Rochester.'
			);
		await expect.element(testPage.getByText(/not now/i)).not.toBeInTheDocument();
	});

	it('records the agreement and goes on to the Practice, reloading its session', async () => {
		apiFetchWithSession.mockResolvedValue(new Response(undefined, { status: 204 }));
		await setup();

		await testPage.getByRole('button', { name: 'Agree and continue' }).click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/practices/practice-1', { invalidateAll: true }));
		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/practices/practice-1/terms-agreement', { method: 'POST' });
	});

	it('says so in place when the press fails, and stays', async () => {
		apiFetchWithSession.mockResolvedValue(Response.json({ code: 'INTERNAL_ERROR', message: 'internal error' }, { status: 500 }));
		await setup();

		await testPage.getByRole('button', { name: 'Agree and continue' }).click();

		await expect.element(testPage.getByRole('alert')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('says so in place when the request does not reach the server', async () => {
		apiFetchWithSession.mockRejectedValue(new TypeError('offline'));
		await setup();

		await testPage.getByRole('button', { name: 'Agree and continue' }).click();

		await expect.element(testPage.getByText('Your agreement could not be saved. Try again.')).toBeVisible();
	});

	// ADR-0048: export, deletion and sign out are one press away.
	it('offers export, deletion and sign out', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: "Export your Practice's data" }))
			.toHaveAttribute('href', '/api/practices/practice-1/export');
		await expect
			.element(testPage.getByRole('link', { name: 'Delete your Practice' }))
			.toHaveAttribute('href', '/practices/practice-1/settings/delete');
		await expect.element(testPage.getByRole('button', { name: 'Sign out' })).toBeVisible();
	});

	it('tells an Owner with no second factor that export needs one, in place of the link', async () => {
		await setup(withoutSecondFactor);

		await expect
			.element(testPage.getByText("You need two-factor authentication before you can export your Practice's data."))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: "Export your Practice's data" })).not.toBeInTheDocument();
	});

	it('signs out to the login screen', async () => {
		signOutOfSession.mockResolvedValue({ ok: true });
		await setup();

		await testPage.getByRole('button', { name: 'Sign out' }).click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/login'));
		expect(signOutOfSession).toHaveBeenCalledWith(
			expect.objectContaining({ unsubscribeURL: '/api/practices/practice-1/push-subscriptions' })
		);
	});

	it('says so in place when sign-out fails', async () => {
		signOutOfSession.mockResolvedValue({ ok: false, message: 'Sign-out failed. You are still signed in on this device.' });
		await setup();

		await testPage.getByRole('button', { name: 'Sign out' }).click();

		await expect.element(testPage.getByText('Sign-out failed. You are still signed in on this device.')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
});
