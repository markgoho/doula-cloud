import { expect, test } from '@playwright/test';
import { portalEngagementSettled } from './mountSettled';
import { seedPortalClient, signInPortalClient } from './portalClient';

test('a Client signs out and can no longer reach their Engagement', async ({ page, request }) => {
	const practiceName = 'Meadowbrook Doulas';
	const { clientEmail, engagementId } = await seedPortalClient(request, practiceName);

	// #854: registered before the navigation, awaited after it -- a tab
	// still inside the Engagement hub's mount chain when a session ends is
	// taken to `?sessionEnded=true` by the refusal, which would overwrite
	// the plain login screen this sign-out is heading for. See
	// mountSettled.ts.
	const settled = portalEngagementSettled(page, engagementId);
	await signInPortalClient(page, request, clientEmail);
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
	await expect(page.getByRole('heading', { name: 'Your care' })).toBeVisible();
	await settled;

	// The control is in the portal authenticated layout, so it is on this
	// screen and on every other one under portal/engagements/[engagementId].
	// Sign out moved behind the avatar menu when the shell landed (#452).
	await page.getByRole('button', { name: /Your account/ }).first().click();
	await page.getByRole('button', { name: 'Sign out' }).first().click();

	// The *portal* login screen, not the Staff one at /login -- and the
	// plain one, with no sessionEnded flag: that is api.ts's "your session
	// expired under you" path, not a deliberate sign-out.
	await expect(page).toHaveURL(/\/portal\/login$/);

	// The cookie was the browser's only credential, so it is gone with the
	// session.
	const cookies = await page.context().cookies();
	expect(
		cookies.find((c) => c.name === '__session'),
		'browser still holds a __session cookie after signing out'
	).toBeFalsy();

	// Pressing Back is the case that matters for a borrowed phone: a
	// history pop, not a fresh navigation. Nothing on the Engagement screen
	// renders before its own authenticated fetch answers (the page has no
	// SSR'd copy to fall back on), so the 401 bounces the pop to the portal
	// login screen with no pregnancy or birth information ever painted.
	await page.goBack();
	await expect(page).toHaveURL(/\/portal\/login\?sessionEnded=true$/);
	// #296: the hub's own `<h1>`. It has to be the heading the hub renders
	// today, not the one it used to: a name nothing renders any more would
	// pass this forever without proving the Engagement screen is gone.
	await expect(page.getByRole('heading', { name: 'Your care' })).toHaveCount(0);

	// And a fresh navigation to the Engagement is refused the same way.
	await page.goto(`/portal/engagements/${engagementId}`);
	await expect(page).toHaveURL(/\/portal\/login\?sessionEnded=true$/);
	await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Your care' })).toHaveCount(0);
});

// The stale-tab case from #153: two tabs share one cookie, so the second
// tab signs out against a session the first already ended. It must land on
// the portal login screen like any other sign-out, not report an error --
// the end-session endpoint is idempotent.
test('a Client second tab signing out after the first shows no error', async ({ page, request }) => {
	const { clientEmail, engagementId } = await seedPortalClient(request, 'Fernwood Doulas');

	// #854: see mountSettled.ts, and the first test's own note above.
	const settled = portalEngagementSettled(page, engagementId);
	await signInPortalClient(page, request, clientEmail);
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
	await settled;

	// A second tab on the same browser context, so it carries the same
	// __session cookie -- and holds it after the first tab signs out.
	const staleTab = await page.context().newPage();
	// #854: the same wait the first tab gets -- this is the tab whose
	// sign-out the test is about, so it is the one a late refusal would
	// steer off the plain login screen it is heading for.
	const staleTabSettled = portalEngagementSettled(staleTab, engagementId);
	await staleTab.goto(`/portal/engagements/${engagementId}`);
	await staleTabSettled;
	await staleTab.getByRole('button', { name: /Your account/ }).first().click();
	await expect(staleTab.getByRole('button', { name: 'Sign out' }).first()).toBeVisible();

	// Sign out moved behind the avatar menu when the shell landed (#452).
	await page.getByRole('button', { name: /Your account/ }).first().click();
	await page.getByRole('button', { name: 'Sign out' }).first().click();
	await expect(page).toHaveURL(/\/portal\/login$/);

	await staleTab.getByRole('button', { name: 'Sign out' }).first().click();

	await expect(staleTab).toHaveURL(/\/portal\/login$/);
	await expect(staleTab.getByRole('alert')).toHaveCount(0);
});
