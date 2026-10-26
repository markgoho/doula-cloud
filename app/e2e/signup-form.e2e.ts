import { expect, test } from '@playwright/test';
import { uniqueEmail, STAFF_PASSWORD } from './staffSignup';
import { seedClient } from './portalClient';

// Every other spec in this suite provisions its Practice with
// POST /api/staff/signup directly (fixture setup is not automation, #207) --
// this is the one spec that drives the /signup screen itself, so the form
// under the whole suite is exercised at least once.
//
// #1531: the Practice this creates has a brand-new Owner with an
// unverified email and no second factor, and neither keeps her out of
// it: she lands on her Practice overview straight after Create
// Practice, with no visit to /mfa/enroll (ADR-0026's amendment for
// #1492). The IndexedDB/cookie checks below are about the signup
// exchange itself.
test('Signing up through the /signup form lands the new Owner on her Practice overview', async ({ page }) => {
	const email = uniqueEmail('signup-form');
	const password = STAFF_PASSWORD;

	await page.goto('/signup');
	await page.getByLabel('First name').fill('Jamie');
	await page.getByLabel('Last name').fill('Owner');
	await page.getByLabel('Practice name (optional)').fill('Riverside Doulas');
	await page.getByLabel('Which state do you work from?').selectOption('New York');
	// #1166: chosen rather than left on whatever zone the runner reports,
	// so what the form sends is the same on a laptop and in CI.
	await page
		.getByLabel('What timezone does this Practice work in?')
		.selectOption('America/New_York');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill(password);
	// #1547: the agreement sentence sits above the button and is its description.
	await expect(page.getByRole('button', { name: 'Create Practice' })).toHaveAccessibleDescription(
		/^By creating your Practice, you agree to the Terms of Service \(opens in new tab\) and the Privacy Policy \(opens in new tab\)/
	);
	await page.getByRole('button', { name: 'Create Practice' }).click();

	await expect(page).toHaveURL(/\/practices\/[^/?]+$/);
	await expect(page.getByRole('heading', { level: 1, name: 'Welcome to Riverside Doulas' })).toBeVisible();

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

// #1536: a solo doula who works under her own name leaves the Practice
// name empty, and the Practice she lands in is called by her name, first
// name then last name.
test('An empty Practice name gives the new Practice her own name', async ({ page }) => {
	await page.goto('/signup');
	await page.getByLabel('First name').fill('Jamie');
	await page.getByLabel('Last name').fill('Owner');
	await page.getByLabel('Which state do you work from?').selectOption('New York');
	await page
		.getByLabel('What timezone does this Practice work in?')
		.selectOption('America/New_York');
	await page.getByLabel('Email').fill(uniqueEmail('signup-own-name'));
	await page.getByLabel('Password').fill(STAFF_PASSWORD);
	await page.getByRole('button', { name: 'Create Practice' }).click();

	await expect(page).toHaveURL(/\/practices\/[^/?]+$/);
	await expect(page.getByRole('heading', { level: 1, name: 'Welcome to Jamie Owner' })).toBeVisible();
});

// #1547 (ADR-0053): pressing Create Practice is the Practice's agreement
// to the Terms of Service and the Privacy Policy, and the Practice's
// Recent activity shows it in plain words, with her name and the date.
// The feed shows once the Practice holds a Client, so the spec adds one.
test("The Practice's activity shows that its Owner agreed to the terms at signup", async ({ page, request }) => {
	await page.goto('/signup');
	await page.getByLabel('First name').fill('Jamie');
	await page.getByLabel('Last name').fill('Owner');
	await page.getByLabel('Which state do you work from?').selectOption('New York');
	await page
		.getByLabel('What timezone does this Practice work in?')
		.selectOption('America/New_York');
	await page.getByLabel('Email').fill(uniqueEmail('signup-terms'));
	await page.getByLabel('Password').fill(STAFF_PASSWORD);
	await page.getByRole('button', { name: 'Create Practice' }).click();
	await expect(page).toHaveURL(/\/practices\/[^/?]+$/);

	const practiceId = new URL(page.url()).pathname.split('/').at(-1)!;
	const cookies = await page.context().cookies();
	const session = cookies.find((cookie) => cookie.name === '__session');
	expect(session, 'no __session cookie after signup').toBeDefined();
	await seedClient(request, practiceId, { Cookie: `__session=${session!.value}` }, { givenName: 'Ada' });
	await page.reload();

	const agreement = page.getByRole('row').filter({ hasText: 'Terms accepted' });
	await expect(agreement).toBeVisible();
	await expect(agreement).toContainText('Jamie Owner');
	await expect(agreement.locator('time')).toHaveAttribute('datetime', /^\d{4}-\d{2}-\d{2}T/);
});
