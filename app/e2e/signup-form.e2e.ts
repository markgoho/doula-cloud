import { expect, test } from '@playwright/test';

// Every other spec in this suite provisions its Practice with
// POST /api/staff/signup directly (fixture setup is not automation, #207) --
// this is the one spec that drives the /signup screen itself, so the form
// under the whole suite is exercised at least once.
//
// #606: the Practice this creates has a brand-new Owner with no second
// factor enrolled at all, and an Owner is gated behind one at every
// Practice-scoped route (staffauth.Middleware) -- so signup drives her
// into MFA enrolment rather than landing her on the Practice it just
// created. The IndexedDB/cookie checks below are about the signup
// exchange itself, not about which screen she lands on afterward.
test('Signing up through the /signup form drives the new Owner into MFA enrolment', async ({ page }) => {
	// Random suffix, not just Date.now(): see staff-login.e2e.ts for why
	// millisecond-only uniqueness collides across parallel workers.
	const email = `signup-form-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
	const password = 'password123';

	await page.goto('/signup');
	await page.getByLabel('Practice name').fill('Riverside Doulas');
	await page.getByLabel('Your name').fill('Jamie Owner');
	await page.getByLabel('Which state do you work from?').selectOption('New York');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Create Practice' }).click();

	await expect(page).toHaveURL(/\/mfa\/enroll\?returnTo=%2Fpractices%2F[^&]+$/);

	// #149: the app exchanges the ID token for the session cookie and signs
	// out of the Firebase JS SDK locally -- a lingering credential would show
	// up in the SDK's default persistence, indexedDBLocalPersistence.
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
	expect(authRecordCount, 'Identity Platform credential left behind in IndexedDB after signup').toBe(0);

	// HttpOnly means no script on the page can read the session credential
	// this way. Name only, never a value: see session-cookie.e2e.ts for the
	// attribute checks.
	const readableCookies = await page.evaluate(() => document.cookie);
	expect(readableCookies, 'session cookie is readable from document.cookie').not.toContain('__session');
});
