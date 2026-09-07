import { expect, test, type APIRequestContext } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, PREVIEW_SERVER_ORIGIN } from './ports';
import { seedFoundingOwner } from './staffSignup';
import { signInEnrolled } from './mfa';
import { seedContractorDoula } from './portalClient';

// #871: a Practice deleting itself -- ADR-0031's rule. This spec proves
// the two things unit tests can't: `staffauth.Middleware`'s lockout
// (api/internal/staffauth/middleware.go) actually reaches the running
// app, and an Owner navigating to a live page mid-deletion actually
// lands on the restore screen -- not just that the load() function
// returns the right redirect target in isolation
// (practice-layout-load.spec.ts already covers that).
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

async function seedEnrolledOwner(request: APIRequestContext, practiceName: string) {
	const { email, idToken, localId, practiceId } = await seedFoundingOwner(request, { practiceName });
	const headers = await signInEnrolled(request, idToken, localId);
	return { email, practiceId, headers };
}

test('an Owner clicks through the real confirm dialog, then revisits and restores', async ({
	page,
	context,
	request
}) => {
	const { practiceId, headers } = await seedEnrolledOwner(request, 'Riverside Doulas');

	const token = headers.Cookie.replace('__session=', '');
	await context.addCookies([
		{ name: '__session', value: token, url: PREVIEW_SERVER_ORIGIN, httpOnly: true, secure: false, sameSite: 'Lax' }
	]);

	// The golden path itself: the button, the dialog naming the Practice
	// (AC #7), and the confirm -- not a bare API POST standing in for it.
	await page.goto(`/practices/${practiceId}/settings/delete`);
	await page.getByRole('button', { name: 'Delete this Practice' }).click();

	const dialog = page.getByRole('dialog');
	await expect(dialog.getByRole('heading', { name: 'Delete Riverside Doulas' })).toBeVisible();
	await expect(dialog.getByText(/countdown to delete Riverside Doulas/)).toBeVisible();
	await dialog.getByRole('button', { name: 'Delete this Practice' }).click();

	await expect(page.getByText(/scheduled to be deleted on/)).toBeVisible();

	// The Practice landing page is where every other e2e spec proves an
	// Owner lands after sign-in -- this is the redirect this ticket adds
	// to +layout.ts, not a direct visit to the destination itself.
	await page.goto(`/practices/${practiceId}`);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/settings/delete$`));
	await expect(page.getByRole('heading', { name: 'Delete this Practice' })).toBeVisible();
	await expect(page.getByText(/scheduled to be deleted on/)).toBeVisible();

	const restoreButton = page.getByRole('button', { name: 'Restore this Practice' });
	await expect(restoreButton).toBeVisible();
	await restoreButton.click();
	await expect(page.getByText('This Practice has been restored.')).toBeVisible();

	// Restored: an ordinary route is reachable again, no redirect back.
	await page.goto(`/practices/${practiceId}`);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));
});

test('a non-Owner Staff member is redirected to the same screen, with no action available to her', async ({
	page,
	context,
	request
}) => {
	const { practiceId, headers: ownerHeaders } = await seedEnrolledOwner(request, 'Hilltop Doulas');
	const doula = await seedContractorDoula(request, practiceId, ownerHeaders);

	const initiate = await request.post(`${API_URL}/api/practices/${practiceId}/deletion`, {
		headers: { ...ownerHeaders, 'X-Confirmed': 'true' }
	});
	expect(initiate.ok()).toBe(true);

	const token = doula.headers.Cookie.replace('__session=', '');
	await context.addCookies([
		{ name: '__session', value: token, url: PREVIEW_SERVER_ORIGIN, httpOnly: true, secure: false, sameSite: 'Lax' }
	]);

	await page.goto(`/practices/${practiceId}`);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/settings/delete$`));
	await expect(
		page.getByText('This Practice is scheduled for deletion. Only an Owner can restore it.')
	).toBeVisible();
	await expect(page.getByRole('button', { name: 'Restore this Practice' })).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Delete this Practice' })).toHaveCount(0);
});
