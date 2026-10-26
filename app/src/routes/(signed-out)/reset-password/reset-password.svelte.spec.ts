import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../routeFixture.js';
import { fixture } from './page.fixture.js';

// The `page` this screen reads comes from the route's own fixture, through
// the same `toPageState` the continuum sweep installs (#596).
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

vi.mock('#lib/api.js', () => ({ apiBaseURL: () => '' }));

// SvelteKit's router is not running in a component spec, so the history
// push on success (#1813) is asserted as the call it is.
const pushState = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ pushState }));

const globalFetch = vi.hoisted(() => vi.fn());

beforeEach(() => {
	vi.stubGlobal('fetch', globalFetch);
	globalFetch.mockReset();
	pushState.mockReset();
});

const passwordField = () => testPage.getByLabelText('New password');
const submit = () => testPage.getByRole('button', { name: 'Reset password' }).click();


/**
Renders the screen as opened from `url` (the fixture's own link by
default). The screen reads the link once, as it mounts, so the fixture's
URL goes back in place as soon as it has rendered.
*/
async function setupFromLink(url = new URL(fixture.url)) {
	pageState.url = url;
	try {
		await render(Page, {});
		await expect.element(passwordField()).toBeVisible();
	} finally {
		pageState.url = new URL(fixture.url);
	}
}

/*
 * The form's username field is `hidden`, so it has no role and no name for
 * an accessible query to find: it is a deliberately non-accessible element
 * (`.claude/rules/svelte-tests.md`, case 2). It is read off the new-password
 * field's own form, which is also the fact a password manager depends on.
 */
const usernameField = () =>
	(passwordField().element() as HTMLInputElement).form?.querySelector<HTMLInputElement>(
		'input[autocomplete="username"]'
	) ?? undefined;

const accountLine = () => testPage.getByText('You are resetting the password for anne-marie+reset@example.test.');

describe('the account a password manager saves the new password against (#1813)', () => {
	it('shows the account email from the link as a line of text, not as a field', async () => {
		await setupFromLink();

		await expect.element(accountLine()).toBeVisible();
		// #1888's read-only Email field looked as if it could be changed.
		await expect.element(testPage.getByRole('textbox', { name: 'Email address' })).not.toBeInTheDocument();
	});

	it('puts the email in a hidden username field in the same form as the new password', async () => {
		await setupFromLink();

		const username = usernameField();
		expect(username).toBeDefined();
		expect(username).toHaveAttribute('type', 'email');
		expect(username).toHaveValue('anne-marie+reset@example.test');
		// Chromium documents a CSS-hidden username field as supported; walk 2
		// step 3 on #1813 proved it, with the URL change below.
		expect(username).not.toBeVisible();
		await expect.element(passwordField()).toHaveAttribute('autocomplete', 'new-password');
	});

	it('reads the email from the fragment, not the query string (#1827)', async () => {
		const inQuery = new URL(fixture.url);
		inQuery.hash = '';
		inQuery.searchParams.set('email', 'anne-marie+reset@example.test');
		await setupFromLink(inQuery);

		await expect.element(accountLine()).not.toBeInTheDocument();
		expect(usernameField()).toBeUndefined();
	});

	it('renders no account line and no username field for a link that carries no email', async () => {
		const withoutEmail = new URL(fixture.url);
		withoutEmail.hash = '';
		await setupFromLink(withoutEmail);

		await expect.element(testPage.getByText(/You are resetting the password for/)).not.toBeInTheDocument();
		expect(usernameField()).toBeUndefined();
	});

	it('tells Safari the rule, so a generated password passes it', async () => {
		await render(Page, {});

		await expect.element(passwordField()).toHaveAttribute('passwordrules', 'minlength: 15;');
	});
});

describe('the URL change that tells Chrome the reset worked (#1813)', () => {
	it('pushes a new URL on success, with no token and no email in it', async () => {
		globalFetch.mockResolvedValueOnce(jsonResponse({}, 204));
		await setupFromLink();
		await passwordField().fill('a password nobody else has chosen');
		await submit();

		await expect.element(testPage.getByText(/Your password has been reset/)).toBeVisible();
		// A push, as walked on 2026-10-24 (walk 2, step 2 and step 3).
		expect(pushState).toHaveBeenCalledTimes(1);
		const [url] = pushState.mock.calls[0];
		expect(url).toBe('/reset-password?done=1');
		expect(url).not.toContain('reset-token-1');
		expect(url).not.toContain('email');
	});

	it('does not change the URL when the reset is refused', async () => {
		// staffauth/reset.go's answer to a spent or expired token.
		globalFetch.mockResolvedValueOnce(
			jsonResponse({ message: 'this link is invalid or has expired -- ask for a new one' }, 400)
		);
		await setupFromLink();
		await passwordField().fill('a password nobody else has chosen');
		await submit();

		await expect.element(testPage.getByRole('button', { name: 'Reset password' })).toBeVisible();
		expect(pushState).not.toHaveBeenCalled();
	});

	it('shows the used-link notice, not a success, on a reload of the success URL', async () => {
		pageState.url = new URL('https://example.test/reset-password?done=1');
		try {
			await render(Page, {});
		} finally {
			pageState.url = new URL(fixture.url);
		}

		await expect.element(testPage.getByText('This link has expired or was already used.')).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Ask for a new link' })).toBeVisible();
		await expect.element(testPage.getByText(/Your password has been reset/)).not.toBeInTheDocument();
		await expect.element(passwordField()).not.toBeInTheDocument();
	});
});

describe('the password rule on the reset screen (#1538)', () => {
	it('states the rule before she types', async () => {
		await render(Page, {});

		await expect
			.element(testPage.getByText('Must be 15 characters or more, and not a commonly used password'))
			.toBeVisible();
	});

	it('refuses a commonly used password before anything is sent (#1539)', async () => {
		await render(Page, {});
		await passwordField().fill('passwordpassword');
		await submit();

		await expect
			.element(testPage.getByRole('link', { name: 'Password is too common. Enter a different password' }))
			.toBeVisible();
		// The list is in the browser: checking a password against it is no request at all.
		expect(globalFetch).not.toHaveBeenCalled();
	});

	it('refuses 14 characters before anything is sent', async () => {
		await render(Page, {});
		await passwordField().fill('a'.repeat(14));
		await submit();

		await expect
			.element(testPage.getByRole('link', { name: 'Password must be 15 characters or more' }))
			.toBeVisible();
		expect(globalFetch).not.toHaveBeenCalled();
	});

	it('sends 64 characters with spaces in them', async () => {
		const long = 'correct horse battery staple '.repeat(3).slice(0, 64);
		globalFetch.mockResolvedValueOnce(jsonResponse({}, 204));
		await render(Page, {});
		await passwordField().fill(long);
		await submit();

		await expect.element(testPage.getByText(/Your password has been reset/)).toBeVisible();
		// The one request this screen makes: the BFF spends the link and hands
		// the password to Identity Platform's Admin SDK (ADR-0026, #613). The
		// common-password check added nothing to it and made no request of its own.
		expect(globalFetch).toHaveBeenCalledTimes(1);
		const [url, init] = globalFetch.mock.calls[0];
		expect(url).toBe('/api/staff/password-reset');
		expect(JSON.parse(init.body)).toEqual({
			token: new URL(fixture.url).searchParams.get('token'),
			newPassword: long
		});
	});
});
