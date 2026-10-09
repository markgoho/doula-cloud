import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { expectOneFramedHeading } from '#lib/components/templates/pageFrame.testing.js';
import Page from './+page.svelte';
import { toPageState } from '../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * The screen reads its token off `page.url` once, as it mounts, so each
 * test installs its own address before `render()`. The fixture's has no
 * token, which is the "missing code" branch.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

vi.mock('#lib/firebase.js', () => ({ getFirebaseAuth: () => ({ currentUser: undefined }) }));
const probeSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiBaseURL: () => '', probeSession }));

beforeEach(() => {
	probeSession.mockReset();
	probeSession.mockResolvedValue(undefined);
});

afterEach(() => {
	vi.unstubAllGlobals();
});

async function setup({ token, respond }: { token?: string; respond?: () => Promise<Response> } = {}) {
	Object.assign(pageState, toPageState(fixture));
	if (token) pageState.url = new URL(`https://example.test/verify-email?token=${token}`);
	if (respond) vi.stubGlobal('fetch', vi.fn(respond));
	await render(Page);
}

// #1576: this screen sat straight in the signed-out layout, with no
// gutter, until it moved onto EntryPage.

describe('verifying an email address', () => {
	it('says it is checking the link while the check is in flight', async () => {
		await setup({ token: 'token-1', respond: () => new Promise<Response>(() => {}) });

		await expect.element(testPage.getByText('Checking your link…')).toBeVisible();
		await expectOneFramedHeading(fixture.readyText);
	});

	it('confirms the address and offers the way to log in', async () => {
		await setup({ token: 'token-1', respond: () => Promise.resolve(new Response(undefined, { status: 204 })) });

		await expect.element(testPage.getByText('Your email address is verified.')).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Continue to log in' })).toBeVisible();
		await expectOneFramedHeading(fixture.readyText);
	});

	// #1504: the Owner who signed up in this browser is still signed in, so
	// the link goes to her Practice and says so, not to a login screen.
	it('names where the link goes for a person who is signed in', async () => {
		probeSession.mockResolvedValue({ staffId: 'staff-1' });
		await setup({ token: 'token-1', respond: () => Promise.resolve(new Response(undefined, { status: 204 })) });

		await expect.element(testPage.getByRole('link', { name: 'Go to your Practice' })).toHaveAttribute('href', '/');
		expect(testPage.getByRole('link', { name: 'Continue to log in' }).elements()).toHaveLength(0);
	});

	// A failed link is not a refused form, so the tab title carries no
	// "Error: " prefix: govuk-alignment.md keeps that for refused forms.
	it('reports a link with no code in words, without an error title', async () => {
		await setup();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('This link is missing its verification code.');
		await expectOneFramedHeading(fixture.readyText);
		expect(document.title.startsWith('Error:')).toBe(false);
	});
});
