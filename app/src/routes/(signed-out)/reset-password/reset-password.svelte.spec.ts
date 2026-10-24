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

const globalFetch = vi.hoisted(() => vi.fn());

beforeEach(() => {
	vi.stubGlobal('fetch', globalFetch);
	globalFetch.mockReset();
});

const passwordField = () => testPage.getByLabelText('New password');
const submit = () => testPage.getByRole('button', { name: 'Reset password' }).click();

const emailField = () => testPage.getByRole('textbox', { name: 'Email' });

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

describe('the account a password manager saves the new password against (#1813)', () => {
	it('shows the account email from the link in a read-only Email field, the username of the form', async () => {
		await setupFromLink();

		// Visible, not `hidden`: on the 2026-10-24 real-Chrome walk, Chrome
		// ignored a display:none username field and saved the password with none.
		await expect.element(emailField()).toBeVisible();
		await expect.element(emailField()).toHaveValue('anne-marie+reset@example.test');
		await expect.element(emailField()).toHaveAttribute('readonly');
		await expect.element(emailField()).toHaveAttribute('type', 'email');
		await expect.element(emailField()).toHaveAttribute('autocomplete', 'username');
		await expect.element(passwordField()).toHaveAttribute('autocomplete', 'new-password');
	});

	it('puts the Email field in the same form as the new password', async () => {
		await setupFromLink();

		const email = emailField().element() as HTMLInputElement;
		const password = passwordField().element() as HTMLInputElement;
		expect(email.form).not.toBeNull();
		expect(email.form).toBe(password.form);
	});

	it('reads the email from the fragment, not the query string (#1827)', async () => {
		const inQuery = new URL(fixture.url);
		inQuery.hash = '';
		inQuery.searchParams.set('email', 'anne-marie+reset@example.test');
		await setupFromLink(inQuery);

		await expect.element(emailField()).not.toBeInTheDocument();
	});

	it('renders no Email field for a link that carries no email', async () => {
		const withoutEmail = new URL(fixture.url);
		withoutEmail.hash = '';
		await setupFromLink(withoutEmail);

		await expect.element(emailField()).not.toBeInTheDocument();
	});

	it('tells Safari the rule, so a generated password passes it', async () => {
		await render(Page, {});

		await expect.element(passwordField()).toHaveAttribute('passwordrules', 'minlength: 15;');
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
