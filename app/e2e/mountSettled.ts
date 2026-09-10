import type { Page, Response } from '@playwright/test';

/**
 * Why a spec ever has to wait for these.
 *
 * Every authenticated screen fetches its own content on mount through
 * `apiFetchWithSession`, and that helper's 401 handling is a navigation:
 * a refusal sends the whole tab to its population's login screen carrying
 * `sessionEnded=true`. So a tab still inside its mount chain when *another*
 * tab ends the shared session is taken to that URL by the refusal -- which
 * overwrites wherever the tab was going on its own, including the plain
 * `/login` its own Sign out is heading for (#854).
 *
 * That is a real thing the product does, and both `sign-out.e2e.ts` and
 * `portal-sign-out.e2e.ts` assert it on purpose in their own
 * "loses access" tests. What it must not do is arrive *inside* a different
 * test's window, where whichever navigation resolves last decides the URL.
 * Waiting for the mount chain's own responses first leaves the tab with
 * nothing in flight, so the sign-out under test is the only navigation in
 * play.
 *
 * These wait on named responses rather than a load state on purpose:
 * `waitForLoadState('networkidle')` is a per-navigation lifecycle fact, so
 * a document that reached idle before hydration fired its first fetch
 * satisfies it immediately, with the whole chain still ahead -- the same
 * shape of one-shot read that #849 removed from `signInPortalClient`.
 *
 * Each helper names every request its screen's mount chain makes through
 * `apiFetchWithSession`, so a call added to one of those chains has to be
 * added here too. The deterministic reproduction recorded on #854 is how
 * that shows up: hold one of these responses past a sign-out and the
 * spec fails on `?sessionEnded=true`.
 */

/**
 * Resolves once a tab has finished the Staff Practice landing screen's
 * mount chain: the authenticated layout's who-am-I read, and the landing
 * page's own three sequential reads, whose last is the "waiting on a
 * reply" roll-up -- so awaiting that one covers the two before it. Call it
 * *before* the navigation, so the listeners are attached when the
 * responses arrive, and await it after.
 */
export function staffLandingSettled(tab: Page, practiceId: string): Promise<Response[]> {
	return Promise.all([
		tab.waitForResponse((response) => response.url().endsWith('/api/staff/session')),
		tab.waitForResponse((response) =>
			response.url().includes(`/practices/${practiceId}/messages/awaiting-reply`)
		)
	]);
}

/**
 * The Client portal's own version, for the Engagement hub: the detail read
 * first, then the activity ledger and the Visits table together. The two
 * concurrent ones settle in no fixed order, so both are named rather than
 * one standing in for the other; the detail read is the one they both wait
 * on, so it needs no line of its own.
 */
export function portalEngagementSettled(tab: Page, engagementId: string): Promise<Response[]> {
	return Promise.all([
		tab.waitForResponse((response) =>
			response.url().includes(`/portal/engagements/${engagementId}/activity`)
		),
		tab.waitForResponse((response) =>
			response.url().includes(`/portal/engagements/${engagementId}/visits`)
		)
	]);
}
