import { expect, test } from '@playwright/test';
import { seedFoundingOwner } from './staffSignup';

// #606: a fresh signup has no second factor enrolled at all, and an Owner
// is gated behind one at every Practice-scoped route (staffauth.Middleware) --
// so she is driven into enrolment rather than landing on her Practice.
// This is still the right spec for login mechanics (the session cookie,
// the cleared Firebase JS SDK credential, session persistence across a
// closed tab): none of those depend on which screen she lands on, only on
// the sign-in itself having gone through.
test('Staff login drives an unenrolled Owner into MFA enrolment, carrying returnTo', async ({ page, request }) => {
	const { email, password, practiceId } = await seedFoundingOwner(request);

	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Log in' }).click();

	const returnTo = encodeURIComponent(`/practices/${practiceId}`);
	await expect(page).toHaveURL(new RegExp(String.raw`/mfa/enroll\?returnTo=${returnTo}$`));

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
	// lands on the same MFA-enrolment redirect a live session gets -- not
	// on /login, which is what an ended or missing session would produce.
	await page.close();
	const reopened = await page.context().newPage();
	await reopened.goto(`/practices/${practiceId}`);
	await expect(reopened).toHaveURL(new RegExp(String.raw`/mfa/enroll\?returnTo=${returnTo}$`));
});
