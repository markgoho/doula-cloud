import { expect, test } from '@playwright/test';
import { PORTAL_CLIENT_PASSWORD, seedPortalClient } from './portalClient';

test('Client-portal login lands on their engagement-scoped URL', async ({ page, request }) => {
	const practiceName = 'Riverside Doulas';
	const { clientEmail, engagementId } = await seedPortalClient(request, practiceName);

	await page.goto('/portal/login');
	await page.getByLabel('Email').fill(clientEmail);
	await page.getByLabel('Password').fill(PORTAL_CLIENT_PASSWORD);
	await page.getByRole('button', { name: 'Log in' }).click();

	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
	await expect(page.locator('h1')).toHaveText(`Welcome to ${practiceName}`);

	// #150: the app exchanges the ID token for the session cookie and
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
	expect(authRecordCount, 'Identity Platform credential left behind in IndexedDB after Client sign-in').toBe(0);

	// Closing the tab and returning within the session lifetime leaves the
	// person signed in: the __session cookie lives on the browser context,
	// not the tab, so a fresh page navigating straight to the Engagement
	// URL should land there without a redirect to /portal/login.
	await page.close();
	const reopened = await page.context().newPage();
	await reopened.goto(`/portal/engagements/${engagementId}`);
	await expect(reopened).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
	await expect(reopened.locator('h1')).toHaveText(`Welcome to ${practiceName}`);
});
