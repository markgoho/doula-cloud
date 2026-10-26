import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { SIGN_OUT_FAILED_MESSAGE } from '#lib/signOut.js';
import { inReaderZone } from '#lib/testZone.js';
import type { RootLanding } from './+page.js';
import Page from './+page.svelte';
import { data as staffPickerData, fixture, portalPickerData, signedOut } from './page.fixture.js';

const SIGNED_OUT = signedOut.props.data;

const goto = vi.hoisted(() => vi.fn());
const refreshAll = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto, refreshAll }));

const signOutOfSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/signOut.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/signOut.js')>()),
	signOutOfSession
}));

const NO_ENGAGEMENT: RootLanding = { type: 'portal-picker', engagements: [] };

beforeEach(() => {
	for (const mock of [goto, refreshAll, signOutOfSession]) mock.mockReset();
});

afterEach(() => {
	vi.useRealTimers();
});

// The portal picker's labels print each Engagement's `createdAt`, an
// instant, in the reader's zone: this file names the zone those days are
// true in rather than trusting the runner's (#1982).
inReaderZone('America/New_York');

interface SetupOptions {
	data?: RootLanding;
}

async function setup({ data = staffPickerData }: SetupOptions = {}) {
	await render(Page, { params: fixture.params, data });
}

describe('/+page.svelte', () => {
	it('offers a signed-out visitor the three real entry points, as three doors', async () => {
		await setup({ data: SIGNED_OUT });

		// #678: the heading names what the page is for rather than
		// repeating the brand, which the signed-out bar above it now
		// carries. #1645 kept it word for word.
		await expect
			.element(testPage.getByRole('heading', { level: 1, name: 'Sign in or set up a Practice' }))
			.toBeVisible();
		// `exact`: each door's accessible name is its name alone, not its
		// name run together with the line under it (#1645).
		for (const [name, href] of [
			['Staff sign in', '/login'],
			['Client portal sign in', '/portal/login'],
			['Set up a Practice', '/signup']
		] as const) {
			const door = testPage.getByRole('link', { name, exact: true });
			await expect.element(door).toBeVisible();
			await expect.element(door).toHaveAttribute('href', href);
		}
		await expect
			.element(testPage.getByRole('link', { name: 'Staff sign in', exact: true }))
			.toHaveAccessibleDescription('For doulas and practice owners.');
		await expect
			.element(testPage.getByRole('link', { name: 'Client portal sign in', exact: true }))
			.toHaveAccessibleDescription('Your contract, your birth plan, and your invoices.');
		await expect
			.element(testPage.getByRole('link', { name: 'Set up a Practice', exact: true }))
			.toHaveAccessibleDescription('New here? Your first three clients are free.');
		await expect.element(testPage.getByText('Welcome to DoulaCloud.')).toBeVisible();
		await expect.element(testPage.getByText('your session ended', { exact: false })).not.toBeInTheDocument();
	});

	// #1645: the greeting reads the device clock -- nobody is signed in, so
	// it is the only clock. Each time is built from local parts, so the
	// answer is the same in Eastern time on a laptop and in UTC on CI.
	it.each([
		[2, 'Up late? Welcome.'],
		[9, 'Good morning.'],
		[14, 'Good afternoon.'],
		[19, 'Good evening.'],
		[23, 'Up late? Welcome.']
	] as const)('greets a signed-out visitor at %i:00 with "%s"', async (hour, greeting) => {
		vi.useFakeTimers({ toFake: ['Date'] });
		vi.setSystemTime(new Date(2026, 9, 2, hour, 0));

		await setup({ data: SIGNED_OUT });

		const line = testPage.getByText(greeting, { exact: true });
		await expect.element(line).toBeVisible();
		// A paragraph, so the h1 alone still names the page (ADR-0021).
		expect(line.element().tagName).toBe('P');
	});

	// The greeting is in the first paint and stays: the page reads the clock
	// once, so passing noon while it is open swaps nothing and moves nothing.
	it('keeps the greeting it first painted when the hour turns while the page is open', async () => {
		vi.useFakeTimers({ toFake: ['Date'] });
		vi.setSystemTime(new Date(2026, 9, 2, 11, 59, 59));
		await setup({ data: SIGNED_OUT });
		const greeting = testPage.getByText('Good morning.', { exact: true });
		await expect.element(greeting).toBeVisible();
		const before = greeting.element().getBoundingClientRect();

		vi.setSystemTime(new Date(2026, 9, 2, 12, 0, 30));
		await new Promise((resolve) => setTimeout(resolve, 50));

		await expect.element(greeting).toBeVisible();
		await expect.element(testPage.getByText('Good afternoon.')).not.toBeInTheDocument();
		expect(greeting.element().getBoundingClientRect()).toEqual(before);
	});

	it('leaves the other states of / on their own template, with no greeting', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Choose a Practice' })).toBeVisible();
		await expect.element(testPage.getByText('Welcome to DoulaCloud.')).not.toBeInTheDocument();
	});

	it("offers a signed-in Staff visitor's several Practices as doors, each with the roles held there", async () => {
		await setup();

		// #2027: a door per Practice, the name as the link and the roles
		// as its description -- `PracticeChooser.svelte.spec.ts` covers
		// the door itself.
		const [firstMembership, secondMembership] = staffPickerData.memberships;
		const link = testPage.getByRole('link', { name: firstMembership.practiceName });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', `/practices/${firstMembership.practiceId}`);
		await expect.element(link).toHaveAccessibleDescription('Owner, Admin, Doula');
		await expect
			.element(testPage.getByRole('link', { name: secondMembership.practiceName }))
			.toHaveAccessibleDescription('Doula');
	});

	// #745 removed this screen's own empty-picker branch: a Staff visitor
	// with no Membership never reaches it, because `+page.ts` redirects
	// her to `/no-practice` first. `root-load.spec.ts` covers that.

	// #312: the portal root lists every Engagement her Portal Account
	// reaches, across every Practice, past and present -- so the fixture
	// deliberately carries one `completed` Engagement, at a different
	// Practice from the `active` one, rather than two Engagements in the
	// same status.
	it("offers a signed-in Client-portal visitor's several Engagements as doors, across Practices and statuses", async () => {
		await setup({ data: portalPickerData });

		const [ongoing, ended] = portalPickerData.engagements;
		// engagementLabel (#310): the Practice name plus when the Engagement
		// began, which is also what tells two Engagements at one Practice
		// apart -- see clientRegister.spec.ts for that case directly. The
		// Client register's fixed labels (ADR-0015) are each door's line,
		// not the raw `active`/`completed` enum values -- a `completed`
		// Engagement stays listed and reads honestly as "Care ended".
		const link = testPage.getByRole('link', { name: `${ongoing.practiceName}, started Jan 15, 2026` });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', `/portal/engagements/${ongoing.engagementId}`);
		await expect.element(link).toHaveAccessibleDescription('Ongoing');
		await expect
			.element(testPage.getByRole('link', { name: `${ended.practiceName}, started Mar 12, 2026` }))
			.toHaveAccessibleDescription('Care ended');
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
		expect(refreshAll).toHaveBeenCalled();
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
