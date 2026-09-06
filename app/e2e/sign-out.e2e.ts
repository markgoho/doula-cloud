import { expect, test } from '@playwright/test';
import { signInEnrolled, enterPracticeAsEnrolled } from './mfa';
import { seedFoundingOwner } from './staffSignup';

test('a Staff member signs out and can no longer reach an authenticated screen', async ({
	page,
	request,
	context
}) => {
	const { idToken, localId, practiceId } = await seedFoundingOwner(request, {
		practiceName: 'Lakeside Doulas',
		staffName: 'Robin Owner'
	});

	// #606: an Owner is gated behind a second factor at every Practice-scoped
	// route (see mfa.ts's signInEnrolled doc comment).
	const staffHeaders = await signInEnrolled(request, idToken, localId);
	await enterPracticeAsEnrolled(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// The control is in the Staff authenticated layout, so it is on this
	// screen and on every other one under practices/[practiceId].
	// Sign out moved behind the avatar menu when the shell landed (#452):
	// the menu is the person's, and Sign out is the one thing in it that
	// every screen under practices/[practiceId] still carries.
	await page.getByRole('button', { name: /Your account/ }).first().click();
	await page.getByRole('button', { name: 'Sign out' }).first().click();

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
test('a second tab signing out after the first shows no error', async ({ page, request, context }) => {
	const { idToken, localId, practiceId } = await seedFoundingOwner(request, {
		practiceName: 'Hillcrest Doulas',
		staffName: 'Sam Owner'
	});

	// #606: an Owner is gated behind a second factor at every Practice-scoped
	// route (see mfa.ts's signInEnrolled doc comment).
	const staffHeaders = await signInEnrolled(request, idToken, localId);
	await enterPracticeAsEnrolled(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// A second tab on the same browser context, so it carries the same
	// __session cookie -- and holds it after the first tab signs out.
	const staleTab = await page.context().newPage();
	await staleTab.goto(`/practices/${practiceId}`);
	await staleTab.getByRole('button', { name: /Your account/ }).first().click();
	await expect(staleTab.getByRole('button', { name: 'Sign out' }).first()).toBeVisible();

	// Sign out moved behind the avatar menu when the shell landed (#452):
	// the menu is the person's, and Sign out is the one thing in it that
	// every screen under practices/[practiceId] still carries.
	await page.getByRole('button', { name: /Your account/ }).first().click();
	await page.getByRole('button', { name: 'Sign out' }).first().click();
	await expect(page).toHaveURL(/\/login$/);

	await staleTab.getByRole('button', { name: 'Sign out' }).first().click();

	await expect(staleTab).toHaveURL(/\/login$/);
	await expect(staleTab.getByRole('alert')).toHaveCount(0);
});

// #155: the other half of the two-tab guarantee -- a tab that never signs
// itself out still loses access once the *first* tab has ended the shared
// session, because both tabs carry the same __session cookie.
test('a second tab loses access once the first tab signs out, without itself signing out', async ({
	page,
	request,
	context
}) => {
	const { idToken, localId, practiceId } = await seedFoundingOwner(request, {
		practiceName: 'Elm Street Doulas',
		staffName: 'Robin Owner'
	});

	// #606: an Owner is gated behind a second factor at every Practice-scoped
	// route (see mfa.ts's signInEnrolled doc comment).
	const staffHeaders = await signInEnrolled(request, idToken, localId);
	await enterPracticeAsEnrolled(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	const secondTab = await page.context().newPage();
	await secondTab.goto(`/practices/${practiceId}`);
	await expect(secondTab.locator('h1')).toHaveText('Welcome to Elm Street Doulas');

	// Sign out moved behind the avatar menu when the shell landed (#452):
	// the menu is the person's, and Sign out is the one thing in it that
	// every screen under practices/[practiceId] still carries.
	await page.getByRole('button', { name: /Your account/ }).first().click();
	await page.getByRole('button', { name: 'Sign out' }).first().click();
	await expect(page).toHaveURL(/\/login$/);

	// No click, no reload of the tab's own doing -- a fresh navigation is
	// the case that matters: no tab is left holding live access once the
	// shared cookie is gone.
	await secondTab.goto(`/practices/${practiceId}`);
	await expect(secondTab).toHaveURL(/\/login\?sessionEnded=true$/);
	await expect(secondTab.getByRole('heading', { name: 'Log in' })).toBeVisible();
});
