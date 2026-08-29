import { expect, test } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';

// The Firebase Auth emulator and the Go BFF -- both host processes -- see
// e2e/global-setup.ts and e2e/stack.ts for how these get started.
const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

test('Staff login lands on their practice-scoped URL', async ({ page, request }) => {
	// The random suffix (not just Date.now(), millisecond-resolution) avoids
	// EMAIL_EXISTS collisions with other *.e2e.ts files' own staff-<ts>@
	// emails when Playwright's parallel workers start within the same
	// millisecond of each other -- confirmed as a real, intermittent
	// failure across this suite, not a theoretical one.
	const email = `staff-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const password = 'password123';

	// Provision the account and Practice directly against the emulator and
	// BFF, the way createUserWithEmailAndPassword + POST /api/staff/signup
	// would from the signup page -- this test is about the *login* landing
	// behaviour, not re-proving signup.
	const signUp = await request.post(
		`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`,
		{ data: { email, password, returnSecureToken: true } }
	);
	expect(signUp.ok(), `signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken } = await signUp.json();

	const signup = await request.post(`${API_URL}/api/staff/signup`, {
		headers: { Authorization: `Bearer ${idToken}` },
		data: { practiceName: 'Riverside Doulas', staffName: 'Jamie Owner', staffEmail: email , workState: 'NY' }
	});
	const signupBody = await signup.text();
	expect(signup.ok(), `signup failed: ${signup.status()} ${signupBody}`).toBe(true);
	const { practiceId } = JSON.parse(signupBody);

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();

	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));
	await expect(page.locator('h1')).toHaveText('Welcome to Riverside Doulas');

	// #149: the app exchanges the ID token for the session cookie and
	// signs out of the Firebase JS SDK locally -- the SDK's default
	// persistence (indexedDBLocalPersistence) is where a lingering
	// credential would live, so a signed-out browser holds no records in
	// its firebaseLocalStorage object store.
	const authRecordCount = await page.evaluate(
		() =>
			new Promise<number>((resolve) => {
				const openRequest = indexedDB.open('firebaseLocalStorageDb');
				openRequest.addEventListener('error', () => resolve(0));
				openRequest.addEventListener('success', () => {
					const database = openRequest.result;
					if (!database.objectStoreNames.contains('firebaseLocalStorage')) {
						database.close();
						resolve(0);
						return;
					}
					const countRequest = database
						.transaction('firebaseLocalStorage', 'readonly')
						.objectStore('firebaseLocalStorage')
						.count();
					countRequest.addEventListener('success', () => {
						database.close();
						resolve(countRequest.result);
					});
					countRequest.addEventListener('error', () => {
						database.close();
						resolve(0);
					});
				});
			})
	);
	expect(authRecordCount, 'Identity Platform credential left behind in IndexedDB after Staff sign-in').toBe(0);

	// HttpOnly means no script on the page -- including one smuggled in by
	// an XSS bug -- can read the session credential this way. Name only,
	// never a value: see session-cookie.e2e.ts for the attribute checks.
	const readableCookies = await page.evaluate(() => document.cookie);
	expect(readableCookies, 'session cookie is readable from document.cookie').not.toContain('__session');

	// Closing the tab and returning within the session lifetime leaves the
	// person signed in: the __session cookie lives on the browser context,
	// not the tab, so a fresh page navigating straight to the practice URL
	// should land there without a redirect to /login.
	await page.close();
	const reopened = await page.context().newPage();
	await reopened.goto(`/practices/${practiceId}`);
	await expect(reopened).toHaveURL(new RegExp(`/practices/${practiceId}$`));
	await expect(reopened.locator('h1')).toHaveText('Welcome to Riverside Doulas');
});
