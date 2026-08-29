import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';

// The Firebase Auth emulator and the Go BFF -- both host processes -- see
// e2e/global-setup.ts and e2e/stack.ts for how these get started.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

test('a Staff member signs out and can no longer reach an authenticated screen', async ({
	page,
	request
}) => {
	// The random suffix (not just Date.now(), millisecond-resolution) avoids
	// EMAIL_EXISTS collisions with other *.e2e.ts files' own staff-<ts>@
	// emails when Playwright's parallel workers start within the same
	// millisecond of each other.
	const email = `signout-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const password = 'password123';

	// Provision the account and Practice directly, the way signup would --
	// this test is about *sign-out*, not re-proving signup.
	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password, returnSecureToken: true } }
	);
	expect(signUp.ok(), `signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken } = await signUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${idToken}` },
		data: { practiceName: 'Lakeside Doulas', staffName: 'Robin Owner', staffEmail: email , workState: 'NY' }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// The control is in the Staff authenticated layout, so it is on this
	// screen and on every other one under practices/[practiceId].
	await page.getByRole('button', { name: 'Sign out' }).click();

	// A deliberate sign-out lands on the plain login screen -- no
	// sessionEnded flag, which is api.ts's "your session expired under you"
	// path, not this one.
	await expect(page).toHaveURL(/\/login$/);

	// The cookie was the browser's only credential, so it is gone with the
	// session.
	const cookies = await page.context().cookies();
	expect(
		cookies.find((c) => c.name === '__session'),
		'browser still holds a __session cookie after signing out'
	).toBeFalsy();

	// Going back to an authenticated screen bounces to login -- this time
	// through api.ts's central 401 path, which carries sessionEnded=true.
	await page.goto(`/practices/${practiceId}`);
	await expect(page).toHaveURL(/\/login\?sessionEnded=true$/);
	await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible();
});

// The stale-tab case from #152: two tabs share one cookie, so the second
// tab signs out against a session the first already ended. It must land
// on the login screen like any other sign-out, not report an error --
// the end-session endpoint is idempotent.
test('a second tab signing out after the first shows no error', async ({ page, request }) => {
	const email = `signout-twice-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const password = 'password123';

	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password, returnSecureToken: true } }
	);
	expect(signUp.ok(), `signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken } = await signUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${idToken}` },
		data: { practiceName: 'Hillcrest Doulas', staffName: 'Sam Owner', staffEmail: email , workState: 'NY' }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// A second tab on the same browser context, so it carries the same
	// __session cookie -- and holds it after the first tab signs out.
	const staleTab = await page.context().newPage();
	await staleTab.goto(`/practices/${practiceId}`);
	await expect(staleTab.getByRole('button', { name: 'Sign out' })).toBeVisible();

	await page.getByRole('button', { name: 'Sign out' }).click();
	await expect(page).toHaveURL(/\/login$/);

	await staleTab.getByRole('button', { name: 'Sign out' }).click();

	await expect(staleTab).toHaveURL(/\/login$/);
	await expect(staleTab.getByRole('alert')).toHaveCount(0);
});

// #155: the other half of the two-tab guarantee -- a tab that never signs
// itself out still loses access once the *first* tab has ended the shared
// session, because both tabs carry the same __session cookie.
test('a second tab loses access once the first tab signs out, without itself signing out', async ({
	page,
	request
}) => {
	const email = `signout-second-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const password = 'password123';

	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password, returnSecureToken: true } }
	);
	expect(signUp.ok(), `signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken } = await signUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${idToken}` },
		data: { practiceName: 'Elm Street Doulas', staffName: 'Robin Owner', staffEmail: email , workState: 'NY' }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	const secondTab = await page.context().newPage();
	await secondTab.goto(`/practices/${practiceId}`);
	await expect(secondTab.locator('h1')).toHaveText('Welcome to Elm Street Doulas');

	await page.getByRole('button', { name: 'Sign out' }).click();
	await expect(page).toHaveURL(/\/login$/);

	// No click, no reload of the tab's own doing -- a fresh navigation is
	// the case that matters: no tab is left holding live access once the
	// shared cookie is gone.
	await secondTab.goto(`/practices/${practiceId}`);
	await expect(secondTab).toHaveURL(/\/login\?sessionEnded=true$/);
	await expect(secondTab.getByRole('heading', { name: 'Log in' })).toBeVisible();
});
