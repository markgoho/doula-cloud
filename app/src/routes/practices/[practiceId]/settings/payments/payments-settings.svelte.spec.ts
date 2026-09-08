import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { CONNECT_STATUS_CHECK_FAILED_MESSAGE, CONNECT_STATUS_POLL_DELAYS_MS } from '#lib/payments.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * The `page` this route reads comes from its own fixture (#596), so the
 * params installed here and the params the continuum sweep installs are
 * one description. The screen also reads the `connect` query parameter
 * Stripe redirects back with, so `url` has to stay settable per test
 * rather than fixed at module scope -- `pageState.url` is reassigned
 * outright, same as the old `mockPage.url` was, rather than mutated
 * through `searchParams` the way clients-list.svelte.spec.ts does it.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

function returnedFromStripe(parameter: 'return' | 'refresh') {
	pageState.url = new URL(`${fixture.url}?connect=${parameter}`);
}

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	// website.ts reads a failure through this; the screen only ever hits
	// the happy path here, but the mock has to carry every export the
	// module tree imports.
	apiErrorMessage: (response: Response) => response.text()
}));

interface MockOptions {
	status?: string;
	roles?: string[];
	requirementsDue?: string[];
	/* What #440's endpoint reports. `own` is the default because it is the
	   precondition every other assertion on this screen depends on --
	   without a declared website there is no button to assert about. */
	websiteMode?: 'undeclared' | 'own' | 'hosted';
	/* Whether the page we publish for her has been confirmed to load
	   (#443). `pending` is the default because it is where every hosted
	   page starts and where the happy path passes through. */
	pageState?: '' | 'pending' | 'live' | 'failed';
}

function mockApi({
	status = 'not_connected',
	roles = [],
	requirementsDue = [],
	websiteMode = 'own',
	pageState: websitePageState = 'pending'
}: MockOptions = {}) {
	// The Membership (roles) comes off page.data.session (#835), not a
	// fetch this mock has to answer.
	pageState.data = {
		session: { practiceId: 'practice-1', practiceName: 'Riverside Doula Collective', roles, isContractor: false }
	};
	// The real gate is Owner-or-Admin (#267), so the mock answers the way
	// the BFF does rather than 200 unconditionally: a Doula who somehow
	// reached the request would get the shared refusal, which is exactly
	// what the screen must never render. The Doula test asserts the request
	// is never made at all, so this branch is a tripwire -- if the guard
	// regresses, the 403 body appears on screen and the test fails loudly
	// rather than passing on a fabricated 200.
	const isOwnerOrAdmin = roles.includes('owner') || roles.includes('admin');
	apiFetchWithSession.mockImplementation((path: string) => {
		if (!isOwnerOrAdmin) {
			return Promise.resolve(new Response('not permitted to read this', { status: 403 }));
		}
		if (path.endsWith('/website')) {
			return Promise.resolve(
				jsonResponse({
					mode: websiteMode,
					ownUrl: websiteMode === 'own' ? 'https://rochesterdoulas.com' : '',
					serviceDescription: '',
					cancellationPolicy: '',
					updatedBy: '',
					updatedAt: '',
					pageState: websiteMode === 'hosted' ? websitePageState : '',
					pageCheckedAt: '',
					pageCheckDetail: '',
					pageUrl: websiteMode === 'hosted' ? 'https://doula.cloud/p/rochester-doulas' : ''
				})
			);
		}
		return Promise.resolve(
			jsonResponse({
				status,
				cardPaymentsStatus: 'unsupported',
				payoutsStatus: 'unsupported',
				requirementsDue
			})
		);
	});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	pageState.url = new URL(fixture.url);
});

afterEach(() => {
	vi.useRealTimers();
});

/** How many times the connect-status endpoint itself has been asked --
 * excludes the website read, which every test below leaves fixed. */
function connectCallCount(): number {
	return apiFetchWithSession.mock.calls.filter((call: unknown[]) => !(call[0] as string).endsWith('/website'))
		.length;
}

type StatusReply = 'error' | { status: string; requirementsDue?: string[] };

/* Answers the connect-status endpoint with one entry per call, holding on
   the last entry once the list runs out -- so a test can drive the exact
   sequence a poll, or the on-demand button, will see across several
   reads. The website endpoint is answered once, fixed, with a website
   already declared: these tests are about the Connect status re-read,
   not the website gates the existing describe blocks above already
   cover. */
function mockApiSequence(replies: StatusReply[], { roles = ['owner'] }: { roles?: string[] } = {}) {
	pageState.data = {
		session: { practiceId: 'practice-1', practiceName: 'Riverside Doula Collective', roles, isContractor: false }
	};
	let call = 0;
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.endsWith('/website')) {
			return Promise.resolve(
				jsonResponse({
					mode: 'own',
					ownUrl: 'https://rochesterdoulas.com',
					serviceDescription: '',
					cancellationPolicy: '',
					updatedBy: '',
					updatedAt: '',
					pageState: '',
					pageCheckedAt: '',
					pageCheckDetail: '',
					pageUrl: ''
				})
			);
		}
		const reply = replies[Math.min(call, replies.length - 1)];
		call += 1;
		if (reply === 'error') {
			return Promise.resolve(new Response('Connect status check failed', { status: 500 }));
		}
		return Promise.resolve(
			jsonResponse({
				status: reply.status,
				cardPaymentsStatus: 'unsupported',
				payoutsStatus: 'unsupported',
				requirementsDue: reply.requirementsDue ?? []
			})
		);
	});
}

describe('payments settings screen', () => {
	it('shows a Connect Stripe button for an Owner when not connected', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Stripe Connect status:')).toBeVisible();
		await expect.element(testPage.getByText('Not connected')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).toBeVisible();
	});

	// #267 split the old single "non-Owner" test in two, because the two
	// populations it covered no longer see the same screen: an Admin reads
	// the status and is told who connects it, a Doula reads nothing at all.
	it('shows an Admin the status and who connects it, with no button', async () => {
		mockApi({ status: 'not_connected', roles: ['admin'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Stripe Connect status:')).toBeVisible();
		await expect.element(testPage.getByText('Not connected')).toBeVisible();
		await expect.element(testPage.getByText('Ask a Practice Owner to connect Stripe.')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
		// The checklist is written in the second person to whoever will sit
		// through Stripe's form -- "have your phone and your bank details
		// with you", "your date of birth". None of that is the Admin's to
		// do, and asserting only the sentence above would pass on a screen
		// that told her to go and find her Social Security number.
		await expect.element(testPage.getByText('What Stripe will ask you for')).not.toBeInTheDocument();
	});

	// The blocked states used to show an Admin nothing at all: the sentence
	// lived beside the button, under a condition that is false whenever the
	// Owner still has the website question to answer.
	it('tells an Admin who connects Stripe even while the website question blocks it', async () => {
		mockApi({ status: 'not_connected', roles: ['admin'], websiteMode: 'undeclared' });
		await render(Page, {});

		await expect.element(testPage.getByText('Ask a Practice Owner to connect Stripe.')).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Answer the website question' })).not.toBeInTheDocument();
	});

	it('asks the BFF nothing for a Doula, and never prints its refusal', async () => {
		mockApi({ status: 'not_connected', roles: ['doula'] });
		await render(Page, {});

		await expect
			.element(testPage.getByText('Only a Practice Owner or Admin can see how this Practice gets paid.'))
			.toBeVisible();
		expect(apiFetchWithSession).not.toHaveBeenCalled();
		await expect.element(testPage.getByText('not permitted to read this')).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Stripe Connect status:')).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Loading your Stripe Connect status')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});

	it('hides the connect button once the account is active, even for an Owner', async () => {
		mockApi({ status: 'active', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Stripe Connect status:')).toBeVisible();
		await expect.element(testPage.getByText('Active', { exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: /Stripe/ })).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Ask a Practice Owner to connect Stripe.')).not.toBeInTheDocument();
	});

	it('offers to continue onboarding for an Owner mid-onboarding', async () => {
		mockApi({ status: 'onboarding_incomplete', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeVisible();
	});

});

describe('what Getting paid is, who it is between, and how it differs from Credits (#256)', () => {
	it('is titled Getting paid, not Payments', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'] });
		await render(Page, {});

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: 'Getting paid' }))
			.toBeVisible();
	});

	// The intro is a fixed sentence, not the Connect status -- present
	// before that fetch resolves (FormPage renders `intro` in its
	// `loading` branch too, #256) and for every role, not only an Owner
	// or Admin.
	it('states its purpose and names Credits as the other screen, before the Connect status resolves', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'] });
		await render(Page, {});

		await expect
			.element(testPage.getByText('This is where a Practice connects Stripe, so its Clients can pay it directly.'))
			.toBeVisible();
		await expect
			.element(
				testPage.getByText('Credits is a separate screen, where this Practice buys Credits from Doula Cloud.')
			)
			.toBeVisible();
	});

	it('states its purpose for a Doula too, who sees no Connect status at all', async () => {
		mockApi({ status: 'not_connected', roles: ['doula'] });
		await render(Page, {});

		await expect
			.element(testPage.getByText('This is where a Practice connects Stripe, so its Clients can pay it directly.'))
			.toBeVisible();
	});
});

describe('payments settings screen: the states Accounts v1 could not report', () => {
	it('offers no onboarding button while Stripe is reviewing', async () => {
		mockApi({ status: 'pending', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Awaiting Stripe review')).toBeVisible();
		await expect
			.element(testPage.getByText('Stripe is reviewing the details you submitted. Nothing is needed from you.'))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).not.toBeInTheDocument();
	});

	it('says invoicing works when only payouts are held up', async () => {
		mockApi({ status: 'payouts_restricted', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Taking payments, payouts on hold')).toBeVisible();
		await expect
			.element(
				testPage.getByText('Clients can pay their invoices, but Stripe cannot send the money to your bank yet.')
			)
			.toBeVisible();
	});

	it('counts what Stripe is still waiting on without leaking its field paths', async () => {
		mockApi({
			status: 'onboarding_incomplete',
			roles: ['owner'],
			requirementsDue: ['configuration.merchant.mcc', 'configuration.merchant.support.phone']
		});
		await render(Page, {});

		await expect.element(testPage.getByText('Stripe needs 2 more details from you.')).toBeVisible();
		await expect.element(testPage.getByText('configuration.merchant.mcc')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeVisible();
	});

	// The same count, said to somebody who is not the one Stripe will ask.
	// "from you" is the Owner's sentence; the Admin reads the state of the
	// Practice's account, not an errand she could run if she wanted to.
	it('names the Owner rather than the reader when an Admin reads the count', async () => {
		mockApi({
			status: 'onboarding_incomplete',
			roles: ['admin'],
			requirementsDue: ['configuration.merchant.mcc']
		});
		await render(Page, {});

		await expect
			.element(testPage.getByText('Stripe needs 1 more detail from a Practice Owner.'))
			.toBeVisible();
		await expect.element(testPage.getByText('Stripe needs 1 more detail from you.')).not.toBeInTheDocument();
	});

	it('offers no onboarding button when payouts are held up with nothing to supply', async () => {
		mockApi({ status: 'payouts_restricted', roles: ['owner'], requirementsDue: [] });
		await render(Page, {});

		await expect.element(testPage.getByText('Taking payments, payouts on hold')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).not.toBeInTheDocument();
	});
});

describe('payments settings screen: what #442 refuses and what it warns about', () => {
	it('refuses the button, and says where to fix it, when no website has been declared', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'], websiteMode: 'undeclared' });
		await render(Page, {});

		await expect
			.element(
				testPage.getByText(
					'Stripe will not let you take Client payments until it can see where you are online.',
					{ exact: false }
				)
			)
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Answer the website question' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});

	it('opens the flow once a page is published here, not only when she has her own site', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'], websiteMode: 'hosted' });
		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).toBeVisible();
	});

	/* #443. The URL Stripe would be handed 404s, and #382 established the
	   review of that URL is ongoing with no published SLA -- so the
	   rejection arrives weeks later with no visible cause. Blocked on the
	   same rule as the missing answer above, and PostConnectHandler
	   refuses the request too. */
	it('refuses the button when the page published for her does not load', async () => {
		mockApi({
			status: 'not_connected',
			roles: ['owner'],
			websiteMode: 'hosted',
			pageState: 'failed'
		});
		await render(Page, {});

		await expect
			.element(testPage.getByText('The page we publish for you is not loading', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Go to website settings' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});

	/* A page still waiting for its deploy must not block her: every
	   Practice passes through `pending` on the way to `live`. */
	it('opens the flow while her page is still waiting for its deploy', async () => {
		mockApi({
			status: 'not_connected',
			roles: ['owner'],
			websiteMode: 'hosted',
			pageState: 'pending'
		});
		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).toBeVisible();
	});

	it('says what Stripe will ask for before the button, not after it', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByRole('heading', { name: 'What Stripe will ask you for' })).toBeVisible();
		await expect.element(testPage.getByText('two-step authentication', { exact: false })).toBeVisible();
		await expect
			.element(testPage.getByText('last four digits of your Social Security number', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByText('bank routing and account numbers', { exact: false })).toBeVisible();
	});

	it('tells her where the text on her Clients card statements comes from', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'] });
		await render(Page, {});

		await expect
			.element(
				testPage.getByText("Stripe puts a short version of your Practice's name on your Clients' card statements", {
					exact: false
				})
			)
			.toBeVisible();
	});

	it('does not congratulate a Practice who came back from Stripe still restricted', async () => {
		returnedFromStripe('return');
		mockApi({
			status: 'onboarding_incomplete',
			roles: ['owner'],
			requirementsDue: ['defaults.profile.business_url']
		});
		await render(Page, {});

		await expect
			.element(testPage.getByText('Stripe still needs something from you before Clients can pay.', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByText('Stripe onboarding finished.', { exact: false })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeVisible();
	});
});

describe('payments settings screen: the one question the two website answers do not share', () => {
	it("warns a Practice on her own site that Stripe wants a description she has not written", async () => {
		mockApi({ status: 'not_connected', roles: ['owner'], websiteMode: 'own' });
		await render(Page, {});

		await expect
			.element(testPage.getByText('A short description of what your Practice offers', { exact: false }))
			.toBeVisible();
	});

	it('does not warn a Practice whose published page already carries one', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'], websiteMode: 'hosted' });
		await render(Page, {});

		await expect
			.element(testPage.getByText('A short description of what your Practice offers', { exact: false }))
			.not.toBeInTheDocument();
	});
});

describe('payments settings screen: re-reading Connect status on return from Stripe (#259)', () => {
	it('re-reads Connect status after a growing delay while the status can still move on its own', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		mockApiSequence([{ status: 'pending' }, { status: 'active' }]);

		await render(Page, {});
		expect(connectCallCount()).toBe(1);
		await expect.element(testPage.getByText('Awaiting Stripe review')).toBeVisible();

		await vi.advanceTimersByTimeAsync(CONNECT_STATUS_POLL_DELAYS_MS[0]);

		expect(connectCallCount()).toBe(2);
		await expect.element(testPage.getByText('Active', { exact: true })).toBeVisible();
	});

	it('updates the badge, explanation, requirement count and banner together, and announces the change', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		mockApiSequence([
			{ status: 'payouts_restricted', requirementsDue: [] },
			{ status: 'active' }
		]);

		const { container } = await render(Page, {});
		await expect.element(testPage.getByText('Taking payments, payouts on hold')).toBeVisible();
		const statusRegion = container.querySelector('[aria-live="polite"]');
		expect(statusRegion).not.toBeNull();

		await vi.advanceTimersByTimeAsync(CONNECT_STATUS_POLL_DELAYS_MS[0]);

		await expect.element(testPage.getByText('Active', { exact: true })).toBeVisible();
		await expect
			.element(testPage.getByText('Clients can pay their invoices and payouts reach your bank.'))
			.toBeVisible();
		await expect.element(testPage.getByText('Taking payments, payouts on hold')).not.toBeInTheDocument();
		expect(statusRegion?.textContent).toContain('Active');
	});

	it('promises to keep checking only while a check is actually still scheduled', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		mockApiSequence([{ status: 'pending' }, { status: 'active' }]);

		await render(Page, {});
		await expect
			.element(
				testPage.getByText("We're checking with Stripe again shortly", { exact: false })
			)
			.toBeVisible();

		await vi.advanceTimersByTimeAsync(CONNECT_STATUS_POLL_DELAYS_MS[0]);

		await expect.element(testPage.getByText('Stripe onboarding finished.', { exact: true })).toBeVisible();
		await expect
			.element(testPage.getByText("We're checking with Stripe again", { exact: false }))
			.not.toBeInTheDocument();
	});

	it('shows the plain finished banner, with no promise to keep checking, for a status already settled', async () => {
		returnedFromStripe('return');
		mockApiSequence([{ status: 'active' }]);

		await render(Page, {});

		await expect.element(testPage.getByText('Stripe onboarding finished.', { exact: true })).toBeVisible();
		await expect
			.element(testPage.getByText("We're checking with Stripe again", { exact: false }))
			.not.toBeInTheDocument();
	});

	it('does not read again once the status has settled', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		mockApiSequence([{ status: 'active' }]);

		await render(Page, {});
		expect(connectCallCount()).toBe(1);

		const totalDelay = CONNECT_STATUS_POLL_DELAYS_MS.reduce((sum, ms) => sum + ms, 0);
		await vi.advanceTimersByTimeAsync(totalDelay);

		expect(connectCallCount()).toBe(1);
	});

	it('does not read again without the return parameter, even for a status that could still move', async () => {
		vi.useFakeTimers();
		mockApiSequence([{ status: 'pending' }]);

		await render(Page, {});
		expect(connectCallCount()).toBe(1);

		const totalDelay = CONNECT_STATUS_POLL_DELAYS_MS.reduce((sum, ms) => sum + ms, 0);
		await vi.advanceTimersByTimeAsync(totalDelay);

		expect(connectCallCount()).toBe(1);
	});

	it('stops re-reading once the delay schedule is exhausted, rather than reading forever', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		// Every reply is 'pending' -- an account whose review never finishes
		// inside the schedule, the case the ceiling exists for.
		mockApiSequence([{ status: 'pending' }]);

		await render(Page, {});

		const totalDelay = CONNECT_STATUS_POLL_DELAYS_MS.reduce((sum, ms) => sum + ms, 0);
		await vi.advanceTimersByTimeAsync(totalDelay);
		expect(connectCallCount()).toBe(1 + CONNECT_STATUS_POLL_DELAYS_MS.length);

		await vi.advanceTimersByTimeAsync(60_000);
		expect(connectCallCount()).toBe(1 + CONNECT_STATUS_POLL_DELAYS_MS.length);
	});

	it('stops re-reading once the screen is left', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		mockApiSequence([{ status: 'pending' }]);

		const { unmount } = await render(Page, {});
		expect(connectCallCount()).toBe(1);
		// Lets onMount's own async work -- the two awaited fetches, then
		// starting the poll -- finish assigning `pollHandle` before this
		// leaves, the same way a real visit always outlives that work.
		await vi.advanceTimersByTimeAsync(0);

		await unmount();
		await vi.advanceTimersByTimeAsync(CONNECT_STATUS_POLL_DELAYS_MS[0]);

		expect(connectCallCount()).toBe(1);
	});

	it('never starts a poll for a screen left before its own first read finishes', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		mockApiSequence([{ status: 'pending' }]);

		// Unmounts before the awaited initial fetch settles, and before
		// `pollHandle` has anything to stop -- the guard this is proving is
		// the `destroyed` flag onMount checks between its own two awaits
		// and the point it would otherwise start the poll.
		const { unmount } = await render(Page, {});
		await unmount();
		await vi.advanceTimersByTimeAsync(60_000);

		expect(connectCallCount()).toBe(1);
	});

	it('keeps the last good status on screen and reports failure when a re-read fails', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		mockApiSequence([{ status: 'pending' }, 'error']);

		await render(Page, {});
		await expect.element(testPage.getByText('Awaiting Stripe review')).toBeVisible();

		await vi.advanceTimersByTimeAsync(CONNECT_STATUS_POLL_DELAYS_MS[0]);

		// Still the same status -- not blanked, and not replaced by
		// FormPage's whole-page loadError, which would drop this text.
		await expect.element(testPage.getByText('Stripe Connect status:')).toBeVisible();
		await expect.element(testPage.getByText('Awaiting Stripe review')).toBeVisible();
		await expect.element(testPage.getByText(CONNECT_STATUS_CHECK_FAILED_MESSAGE)).toBeVisible();
	});

	it('lets the person check the status again on demand, and the Continue button follows the new answer', async () => {
		mockApiSequence([
			{ status: 'payouts_restricted', requirementsDue: ['configuration.merchant.mcc'] },
			{ status: 'payouts_restricted', requirementsDue: [] }
		]);

		await render(Page, {});
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeVisible();

		await testPage.getByRole('button', { name: 'Check status again' }).click();

		await expect
			.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' }))
			.not.toBeInTheDocument();
	});

	it('reports a failed on-demand check without blanking the status', async () => {
		mockApiSequence([{ status: 'not_connected' }, 'error']);

		await render(Page, {});
		await testPage.getByRole('button', { name: 'Check status again' }).click();

		await expect.element(testPage.getByText(CONNECT_STATUS_CHECK_FAILED_MESSAGE)).toBeVisible();
		await expect.element(testPage.getByText('Not connected')).toBeVisible();
	});
});
