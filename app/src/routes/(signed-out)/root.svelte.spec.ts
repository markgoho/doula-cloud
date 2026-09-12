import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { SIGN_OUT_FAILED_MESSAGE } from '#lib/signOut.js';
import type { RootLanding } from './+page.js';
import Page from './+page.svelte';
import { data as staffPickerData, fixture } from './page.fixture.js';

const goto = vi.hoisted(() => vi.fn());
const invalidateAll = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto, invalidateAll }));

const signOutOfSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/signOut.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/signOut.js')>()),
	signOutOfSession
}));

const NO_ENGAGEMENT: RootLanding = { type: 'portal-picker', engagements: [] };

beforeEach(() => {
	for (const mock of [goto, invalidateAll, signOutOfSession]) mock.mockReset();
});

interface SetupOptions {
	data?: RootLanding;
}

async function setup({ data = staffPickerData }: SetupOptions = {}) {
	await render(Page, { params: fixture.params, data });
}

describe('/+page.svelte', () => {
	it('offers a signed-out visitor the three real entry points, none implied as the main one', async () => {
		await setup({ data: { type: 'signed-out' } });

		// #678: the heading names what the page is for rather than
		// repeating the brand, which the signed-out bar above it now
		// carries.
		await expect
			.element(testPage.getByRole('heading', { level: 1, name: 'Sign in or set up a Practice' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Staff log in' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Set up a Practice' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Client portal log in' })).toBeVisible();
		await expect.element(testPage.getByText('your session ended', { exact: false })).not.toBeInTheDocument();
	});

	it("lists a signed-in Staff visitor's several Practices when there is no single or last-used one", async () => {
		// The fixture's own Membership carries #530's URL; a second one is
		// added here rather than invented from scratch, since "several" is
		// content the fixture -- one Membership -- does not itself hold.
		await setup({
			data: {
				...staffPickerData,
				memberships: [
					...staffPickerData.memberships,
					{ practiceId: 'practice-2', practiceName: 'Hilltop Doulas', roles: ['doula'] }
				]
			}
		});

		const [firstMembership] = staffPickerData.memberships;
		const link = testPage.getByRole('link', { name: firstMembership.practiceName });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', `/practices/${firstMembership.practiceId}`);
		await expect.element(testPage.getByRole('link', { name: 'Hilltop Doulas' })).toBeVisible();
	});

	// #745 removed this screen's own empty-picker branch: a Staff visitor
	// with no Membership never reaches it, because `+page.ts` redirects
	// her to `/no-practice` first. `root-load.spec.ts` covers that.

	// #312: the portal root lists every Engagement her Portal Account
	// reaches, across every Practice, past and present -- so this fixture
	// deliberately carries one `completed` Engagement, at a different
	// Practice from the `active` one, rather than two Engagements in the
	// same status.
	it("lists a signed-in Client-portal visitor's several Engagements, across Practices and statuses", async () => {
		await setup({
			data: {
				type: 'portal-picker',
				engagements: [
					{
						engagementId: 'engagement-1',
						practiceName: 'Riverside Doulas',
						status: 'active',
						createdAt: '2026-01-15T20:00:00Z'
					},
					{
						engagementId: 'engagement-2',
						practiceName: 'Hilltop Doulas',
						status: 'completed',
						createdAt: '2026-03-12T20:00:00Z'
					}
				]
			}
		});

		// engagementLabel (#310): the Practice name plus when the Engagement
		// began, which is also what tells two Engagements at one Practice
		// apart -- see clientRegister.spec.ts for that case directly.
		const link = testPage.getByRole('link', { name: 'Riverside Doulas, started Jan 15, 2026' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/portal/engagements/engagement-1');
		await expect
			.element(testPage.getByRole('link', { name: 'Hilltop Doulas, started Mar 12, 2026' }))
			.toBeVisible();

		// The Client register's fixed labels (ADR-0015), not the raw
		// `active`/`completed` enum values -- a `completed` Engagement
		// stays listed and reads honestly as "Care ended" rather than
		// dropping off the list or reading as a raw status a Client has no
		// register entry for.
		await expect.element(testPage.getByText('Ongoing')).toBeVisible();
		await expect.element(testPage.getByText('Care ended')).toBeVisible();
	});

	// #1116: the one state of `/` that offers no destination. Two people
	// reach it -- one whose Practice has not set her care up yet, and one
	// who signed in with an address her Practice does not have -- so the
	// screen names the state, gives what to do about each cause, and
	// carries the door out of the session it says leads nowhere.
	it('names the state for a Client-portal visitor with no care set up, rather than showing an empty list', async () => {
		await setup({ data: NO_ENGAGEMENT });

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: "You don't have care set up yet" }))
			.toBeVisible();
		await expect.element(testPage.getByText('Ask your Practice to set it up', { exact: false })).toBeVisible();
		await expect
			.element(testPage.getByText('signed in with a different email address', { exact: false }))
			.toBeVisible();
	});

	it('lets a Client-portal visitor with no care set up end the session she is holding', async () => {
		signOutOfSession.mockResolvedValue({ ok: true });
		await setup({ data: NO_ENGAGEMENT });

		await testPage.getByRole('button', { name: 'Sign out' }).click();

		// The portal's own login screen, not the Staff one (#153), and no
		// push scope to unregister: she has no Engagement to be subscribed
		// under.
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/portal/login'));
		expect(signOutOfSession).toHaveBeenCalledWith(expect.objectContaining({ unsubscribeURL: undefined }));
		expect(invalidateAll).toHaveBeenCalled();
	});

	it('keeps her here, told she is still signed in, when sign-out does not go through', async () => {
		signOutOfSession.mockResolvedValue({ ok: false, message: SIGN_OUT_FAILED_MESSAGE });
		await setup({ data: NO_ENGAGEMENT });

		await testPage.getByRole('button', { name: 'Sign out' }).click();

		await expect.element(testPage.getByText(SIGN_OUT_FAILED_MESSAGE)).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	// The three other states of `/` each offer a destination that lands in
	// a fully dressed shell carrying its own sign-out, so the reduced bar
	// above them stays as `+layout.svelte` describes it.
	it('leaves the states that do offer a destination without a sign-out of their own', async () => {
		await setup();

		await expect.element(testPage.getByRole('button', { name: 'Sign out' })).not.toBeInTheDocument();
	});
});
