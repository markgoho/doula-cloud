import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';

// The Firebase Auth emulator and the Go BFF -- both host processes -- see
// e2e/global-setup.ts and e2e/stack.ts for how these get started.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

// This is the one seam (#144) that proves a real browser -- not just a
// Go http.Client in the unit suite -- actually accepts and stores the
// __session cookie create-session sets: the browser's own cookie jar,
// via page.request (which shares it with page's BrowserContext, unlike
// the standalone `request` fixture other e2e specs use for setup calls).
test('a browser accepts and clears the __session cookie', async ({ page }) => {
	const email = `session-cookie-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const password = 'password123';

	const signUp = await page.request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password, returnSecureToken: true } }
	);
	expect(signUp.ok(), `signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken } = await signUp.json();

	const create = await page.request.post(`${API_URL}/api/session`, {
		headers: { Authorization: `Bearer ${idToken}` }
	});
	expect(create.ok(), `create-session failed: ${create.status()} ${await create.text()}`).toBe(true);

	// No URL filter: context.cookies(url) matches a Secure cookie only
	// against an https:// filter URL, even though Chromium genuinely
	// stores and would send it for http://127.0.0.1 -- loopback is a
	// trustworthy origin. Confirmed empirically while writing this test;
	// filtering by API_URL here reports the cookie as missing when it is
	// not.
	const afterCreate = await page.context().cookies();
	const created = afterCreate.find((c) => c.name === '__session');
	expect(created, 'no __session cookie stored after create-session').toBeTruthy();
	expect(created?.httpOnly).toBe(true);
	expect(created?.secure).toBe(true);
	expect(created?.sameSite).toBe('Lax');
	expect(created?.path).toBe('/');
	expect(created?.value).not.toBe('');

	const end = await page.request.delete(`${API_URL}/api/session`);
	expect(end.ok(), `end-session failed: ${end.status()} ${await end.text()}`).toBe(true);

	const afterEnd = await page.context().cookies();
	const cleared = afterEnd.find((c) => c.name === '__session');
	expect(cleared, 'browser still holds a __session cookie after end-session').toBeFalsy();
});
