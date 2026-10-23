import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { toPageState } from '../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * #1539: the common-password list is a chunk of its own, and a chunk can
 * fail to load. A file of its own because the mock below replaces the list
 * for every test in the file. The same refusal reaches /signup and
 * /accept-invite through their own catch-alls (authRefusal's default).
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

vi.mock('#lib/api.js', () => ({ apiBaseURL: () => '' }));
// The browser mocker cannot fail an import itself, so the list fails the
// moment it is read, which reaches the screen the same way.
vi.mock('#lib/commonPasswords.js', () => ({
	get COMMON_PASSWORDS(): never {
		throw new Error('chunk failed to load');
	}
}));

const globalFetch = vi.hoisted(() => vi.fn());
vi.stubGlobal('fetch', globalFetch);

describe('the reset screen when the common-password list cannot be loaded', () => {
	it('refuses the password and says the service has a problem, rather than let it through unchecked', async () => {
		await render(Page, {});
		await testPage.getByLabelText('New password').fill('a password nobody else has chosen');
		await testPage.getByRole('button', { name: 'Reset password' }).click();

		await expect
			.element(testPage.getByText('There is a problem with the service. Try again in a few minutes.').first())
			.toBeVisible();
		expect(globalFetch).not.toHaveBeenCalled();
	});
});
