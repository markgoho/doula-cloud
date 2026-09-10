import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import {
	CONNECT_NUDGE_SENT_MESSAGE,
	CONNECT_OWNERS_ALREADY_EMAILED_MESSAGE,
	CONNECT_STATUS_CHECK_FAILED_MESSAGE,
	CONNECT_STATUS_POLL_DELAYS_MS
} from '#lib/payments.js';
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
	/* #917: the sentence the BFF refuses the connect nudge with, when it
	   refuses at all. Undefined means it accepts, which is the ordinary
	   case -- a refusal is the cooldown, or an Owner who connected in
	   another tab, neither of which the screen can see coming. */
	nudgeRefusal?: string;
	/* What #271's billing-mode read answers with. The default is a
	   Practice that has already chosen Stripe, because that is the
	   established state every other assertion here is made against; `{}`
	   is a Practice that has raised no Invoice yet and so has never been
	   asked. */
	billingModeBody?: unknown;
}

function mockApi({
	status = 'not_connected',
	roles = [],
	requirementsDue = [],
	websiteMode = 'own',
	pageState: websitePageState = 'pending',
	nudgeRefusal,
	billingModeBody = { billingMode: 'stripe' }
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
		// #271: read by any Staff member, unconditionally, before the
		// Owner/Admin gate below -- so it must not consume a slot from
		// this mock's connect-status sequencing, nor trip the not-permitted
		// tripwire the Doula tests below rely on.
		if (path.endsWith('/payments/billing-mode')) {
			return Promise.resolve(jsonResponse(billingModeBody));
		}
		// #768: payment terms are read by any Staff member too, for the
		// same reason, and are outside the connect-status sequencing
		// below in exactly the same way.
		if (path.endsWith('/payments/payment-terms')) {
			return Promise.resolve(jsonResponse({ netDays: 30, isDefault: true }));
		}
		if (!isOwnerOrAdmin) {
			return Promise.resolve(new Response('not permitted to read this', { status: 403 }));
		}
		// #917. Placed above the connect-status fallback so it is never
		// answered with a status body, and above nothing else -- it is
		// Owner-and-Admin gated exactly like the read below it.
		if (path.endsWith('/payments/connect/nudge')) {
			return Promise.resolve(
				nudgeRefusal === undefined
					? new Response(undefined, { status: 202 })
					: new Response(nudgeRefusal, { status: 409 })
			);
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
	return apiFetchWithSession.mock.calls.filter((call: unknown[]) => {
		const path = call[0] as string;
		return (
			!path.endsWith('/website') &&
			!path.endsWith('/payments/billing-mode') &&
			!path.endsWith('/payments/payment-terms')
		);
	}).length;
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
		// #271 and #768: see the same guards in mockApi above -- neither
		// may consume a slot from this mock's connect-status reply
		// sequence.
		if (path.endsWith('/payments/billing-mode')) {
			return Promise.resolve(jsonResponse({ billingMode: 'stripe' }));
		}
		if (path.endsWith('/payments/payment-terms')) {
			return Promise.resolve(jsonResponse({ netDays: 30, isDefault: true }));
		}
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

/* This file's `setup()`, per `.claude/rules/svelte-tests.md` -- the two
   mocks above already hold what a `setup()` would construct, so it joins
   one of them to the `render()` rather than building a screen of its own.
   It sits at module scope, shared by every `describe` below, because each
   block varies the same `MockOptions` the mock already names: eight
   per-block copies would be byte-identical, and a test's own options say
   which state of the screen it is about far better than which block it
   sits in.

   `roles` stays explicit at every call site rather than defaulting to an
   Owner: almost every test here is named for the person reading the
   screen, and the role is the fact that makes its assertion true.

   It returns nothing: every test it serves asserts through the role tree
   and reads no handle off the render. */
async function setup(options: MockOptions = {}) {
	mockApi(options);
	await render(Page, {});
}

/** The same, for the #259 block: a poll drives several reads, so its
 * screen is constructed from a sequence of replies rather than one
 * state. It returns the two handles that block does read -- `container`
 * for the live region, `unmount` for leaving the screen mid-poll. */
async function setupSequence(replies: StatusReply[], options: { roles?: string[] } = {}) {
	mockApiSequence(replies, options);
	const { container, unmount } = await render(Page, {});
	return { container, unmount };
}

describe('payments settings screen', () => {
	it('shows a Connect Stripe button for an Owner when not connected', async () => {
		await setup({ status: 'not_connected', roles: ['owner'] });

		await expect.element(testPage.getByText('Stripe Connect status:')).toBeVisible();
		await expect.element(testPage.getByText('Not connected')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).toBeVisible();
	});

	// #267 split the old single "non-Owner" test in two, because the two
	// populations it covered no longer see the same screen: an Admin reads
	// the status and is told who connects it, a Doula reads nothing at all.
	it('shows an Admin the status and who connects it, with no button', async () => {
		await setup({ status: 'not_connected', roles: ['admin'] });

		await expect.element(testPage.getByText('Stripe Connect status:')).toBeVisible();
		await expect.element(testPage.getByText('Not connected')).toBeVisible();
		await expect.element(testPage.getByText('A Practice Owner has to connect Stripe.')).toBeVisible();
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
		await setup({ status: 'not_connected', roles: ['admin'], websiteMode: 'undeclared' });

		await expect.element(testPage.getByText('A Practice Owner has to connect Stripe.')).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Answer the website question' })).not.toBeInTheDocument();
	});

	it('asks the BFF nothing for Connect status for a Doula, and never prints its refusal', async () => {
		await setup({ status: 'not_connected', roles: ['doula'] });

		await expect
			.element(testPage.getByText('Only a Practice Owner or Admin can see how this Practice gets paid.'))
			.toBeVisible();
		// #271: billing mode is the one exception -- readable by any Staff
		// member, so this is the only call a Doula's session makes here.
		expect(connectCallCount()).toBe(0);
		await expect.element(testPage.getByText('This Practice bills Clients through Stripe.')).toBeVisible();
		await expect.element(testPage.getByText('not permitted to read this')).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Stripe Connect status:')).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Loading your Stripe Connect status')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});

	it('hides the connect button once the account is active, even for an Owner', async () => {
		await setup({ status: 'active', roles: ['owner'] });

		await expect.element(testPage.getByText('Stripe Connect status:')).toBeVisible();
		await expect.element(testPage.getByText('Active', { exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: /Stripe/ })).not.toBeInTheDocument();
		await expect.element(testPage.getByText('A Practice Owner has to connect Stripe.')).not.toBeInTheDocument();
	});

	it('offers to continue onboarding for an Owner mid-onboarding', async () => {
		await setup({ status: 'onboarding_incomplete', roles: ['owner'] });

		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeVisible();
	});

});

/*
 * #917 (ADR-0035). The Admin can see Stripe is unconnected and cannot
 * connect it; whether the screen offers her a way to say so turns on
 * whether anything else in the product would reach an Owner about it.
 * Only `not_connected` produces no Stripe webhook, so only there does
 * #343's payout Notification fail to fire on its own.
 */
describe('telling an Owner the Practice still has to connect Stripe (#917)', () => {
	it('offers an Admin the ask when nothing else would reach an Owner', async () => {
		await setup({ status: 'not_connected', roles: ['admin'] });

		await expect.element(testPage.getByText('A Practice Owner has to connect Stripe.')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Email the Practice Owners' })).toBeVisible();
	});

	it('confirms the send, and takes the control away so a second press is not offered', async () => {
		await setup({ status: 'not_connected', roles: ['admin'] });

		await testPage.getByRole('button', { name: 'Email the Practice Owners' }).click();

		await expect.element(testPage.getByText(CONNECT_NUDGE_SENT_MESSAGE)).toBeVisible();
		await expect
			.element(testPage.getByRole('button', { name: 'Email the Practice Owners' }))
			.not.toBeInTheDocument();
		const nudged = apiFetchWithSession.mock.calls.filter((call: unknown[]) =>
			(call[0] as string).endsWith('/payments/connect/nudge')
		);
		expect(nudged).toHaveLength(1);
	});

	it("says why the ask was refused rather than swallowing the server's sentence", async () => {
		// The ordinary way to meet this is a colleague having asked
		// yesterday from her own screen, which this one cannot see.
		const refusal = 'Doula Cloud was already asked to email every Practice Owner about this in the last week.';
		await setup({ status: 'not_connected', roles: ['admin'], nudgeRefusal: refusal });

		await testPage.getByRole('button', { name: 'Email the Practice Owners' }).click();

		await expect.element(testPage.getByText(refusal)).toBeVisible();
		await expect.element(testPage.getByText(CONNECT_NUDGE_SENT_MESSAGE)).not.toBeInTheDocument();
	});

	it('tells an Admin mid-onboarding that the Owners already have the email, and offers no second one', async () => {
		await setup({ status: 'onboarding_incomplete', roles: ['admin'], requirementsDue: ['individual.dob'] });

		await expect.element(testPage.getByText(CONNECT_OWNERS_ALREADY_EMAILED_MESSAGE)).toBeVisible();
		await expect
			.element(testPage.getByRole('button', { name: 'Email the Practice Owners' }))
			.not.toBeInTheDocument();
	});

	it('never offers the ask to an Owner, who can connect Stripe herself', async () => {
		await setup({ status: 'not_connected', roles: ['owner'] });

		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).toBeVisible();
		await expect
			.element(testPage.getByRole('button', { name: 'Email the Practice Owners' }))
			.not.toBeInTheDocument();
	});
});

describe('what Getting paid is, who it is between, and how it differs from Credits (#256)', () => {
	it('is titled Getting paid, not Payments', async () => {
		await setup({ status: 'not_connected', roles: ['owner'] });

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: 'Getting paid' }))
			.toBeVisible();
	});

	// The intro is a fixed sentence, not the Connect status -- present
	// before that fetch resolves (FormPage renders `intro` in its
	// `loading` branch too, #256) and for every role, not only an Owner
	// or Admin.
	it('states its purpose and names Credits as the other screen, before the Connect status resolves', async () => {
		await setup({ status: 'not_connected', roles: ['owner'] });

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
		await setup({ status: 'not_connected', roles: ['doula'] });

		await expect
			.element(testPage.getByText('This is where a Practice connects Stripe, so its Clients can pay it directly.'))
			.toBeVisible();
	});
});

describe('payments settings screen: the states Accounts v1 could not report', () => {
	it('offers no onboarding button while Stripe is reviewing', async () => {
		await setup({ status: 'pending', roles: ['owner'] });

		await expect.element(testPage.getByText('Awaiting Stripe review')).toBeVisible();
		await expect
			.element(testPage.getByText('Stripe is reviewing the details already submitted. Nothing more is needed right now.'))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).not.toBeInTheDocument();
	});

	it('says invoicing works when only payouts are held up', async () => {
		await setup({ status: 'payouts_restricted', roles: ['owner'] });

		await expect.element(testPage.getByText('Taking payments, payouts on hold')).toBeVisible();
		await expect
			.element(
				testPage.getByText("Clients can pay their invoices, but Stripe cannot send the money to this Practice's bank yet.")
			)
			.toBeVisible();
	});

	it('counts what Stripe is still waiting on without leaking its field paths', async () => {
		await setup({
			status: 'onboarding_incomplete',
			roles: ['owner'],
			requirementsDue: ['configuration.merchant.mcc', 'configuration.merchant.support.phone']
		});

		await expect.element(testPage.getByText('Stripe needs 2 more details from you.')).toBeVisible();
		await expect.element(testPage.getByText('configuration.merchant.mcc')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeVisible();
	});

	// The same count, said to somebody who is not the one Stripe will ask.
	// "from you" is the Owner's sentence; the Admin reads the state of the
	// Practice's account, not an errand she could run if she wanted to.
	it('names the Owner rather than the reader when an Admin reads the count', async () => {
		await setup({
			status: 'onboarding_incomplete',
			roles: ['admin'],
			requirementsDue: ['configuration.merchant.mcc']
		});

		await expect
			.element(testPage.getByText('Stripe needs 1 more detail from a Practice Owner.'))
			.toBeVisible();
		await expect.element(testPage.getByText('Stripe needs 1 more detail from you.')).not.toBeInTheDocument();
	});

	it('offers no onboarding button when payouts are held up with nothing to supply', async () => {
		await setup({ status: 'payouts_restricted', roles: ['owner'], requirementsDue: [] });

		await expect.element(testPage.getByText('Taking payments, payouts on hold')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).not.toBeInTheDocument();
	});
});

describe('payments settings screen: what #442 refuses and what it warns about', () => {
	it('refuses the button, and says where to fix it, when no website has been declared', async () => {
		await setup({ status: 'not_connected', roles: ['owner'], websiteMode: 'undeclared' });

		await expect
			.element(
				testPage.getByText(
					'Stripe will not process Client payments until it can see this Practice online.',
					{ exact: false }
				)
			)
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Answer the website question' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});

	it('opens the flow once a page is published here, not only when she has her own site', async () => {
		await setup({ status: 'not_connected', roles: ['owner'], websiteMode: 'hosted' });

		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).toBeVisible();
	});

	/* #443. The URL Stripe would be handed 404s, and #382 established the
	   review of that URL is ongoing with no published SLA -- so the
	   rejection arrives weeks later with no visible cause. Blocked on the
	   same rule as the missing answer above, and PostConnectHandler
	   refuses the request too. */
	it('refuses the button when the page published for her does not load', async () => {
		await setup({
			status: 'not_connected',
			roles: ['owner'],
			websiteMode: 'hosted',
			pageState: 'failed'
		});

		await expect
			.element(testPage.getByText('The page published for this Practice is not loading', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Go to website settings' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});

	/* A page still waiting for its deploy must not block her: every
	   Practice passes through `pending` on the way to `live`. */
	it('opens the flow while her page is still waiting for its deploy', async () => {
		await setup({
			status: 'not_connected',
			roles: ['owner'],
			websiteMode: 'hosted',
			pageState: 'pending'
		});

		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).toBeVisible();
	});

	it('says what Stripe will ask for before the button, not after it', async () => {
		await setup({ status: 'not_connected', roles: ['owner'] });

		await expect.element(testPage.getByRole('heading', { name: 'What Stripe will ask you for' })).toBeVisible();
		await expect.element(testPage.getByText('two-step authentication', { exact: false })).toBeVisible();
		await expect
			.element(testPage.getByText('last four digits of your Social Security number', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByText('bank routing and account numbers', { exact: false })).toBeVisible();
	});

	it('tells her where the text on her Clients card statements comes from', async () => {
		await setup({ status: 'not_connected', roles: ['owner'] });

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
		await setup({
			status: 'onboarding_incomplete',
			roles: ['owner'],
			requirementsDue: ['defaults.profile.business_url']
		});

		await expect
			.element(testPage.getByText('Stripe still needs something before Clients can pay this Practice.', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByText('Stripe onboarding finished.', { exact: false })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeVisible();
	});
});

describe('payments settings screen: the one question the two website answers do not share', () => {
	it("warns a Practice on her own site that Stripe wants a description she has not written", async () => {
		await setup({ status: 'not_connected', roles: ['owner'], websiteMode: 'own' });

		await expect
			.element(testPage.getByText('A short description of what your Practice offers', { exact: false }))
			.toBeVisible();
	});

	it('does not warn a Practice whose published page already carries one', async () => {
		await setup({ status: 'not_connected', roles: ['owner'], websiteMode: 'hosted' });

		await expect
			.element(testPage.getByText('A short description of what your Practice offers', { exact: false }))
			.not.toBeInTheDocument();
	});
});

describe('payments settings screen: re-reading Connect status on return from Stripe (#259)', () => {
	it('re-reads Connect status after a growing delay while the status can still move on its own', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		await setupSequence([{ status: 'pending' }, { status: 'active' }]);
		expect(connectCallCount()).toBe(1);
		await expect.element(testPage.getByText('Awaiting Stripe review')).toBeVisible();

		await vi.advanceTimersByTimeAsync(CONNECT_STATUS_POLL_DELAYS_MS[0]);

		expect(connectCallCount()).toBe(2);
		await expect.element(testPage.getByText('Active', { exact: true })).toBeVisible();
	});

	it('updates the badge, explanation, requirement count and banner together, and announces the change', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		const { container } = await setupSequence([
			{ status: 'payouts_restricted', requirementsDue: [] },
			{ status: 'active' }
		]);
		await expect.element(testPage.getByText('Taking payments, payouts on hold')).toBeVisible();
		const statusRegion = container.querySelector('[aria-live="polite"]');
		expect(statusRegion).not.toBeNull();

		await vi.advanceTimersByTimeAsync(CONNECT_STATUS_POLL_DELAYS_MS[0]);

		await expect.element(testPage.getByText('Active', { exact: true })).toBeVisible();
		await expect
			.element(testPage.getByText("Clients can pay their invoices, and payouts reach this Practice's bank."))
			.toBeVisible();
		await expect.element(testPage.getByText('Taking payments, payouts on hold')).not.toBeInTheDocument();
		expect(statusRegion?.textContent).toContain('Active');
	});

	it('promises to keep checking only while a check is actually still scheduled', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		await setupSequence([{ status: 'pending' }, { status: 'active' }]);
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
		await setupSequence([{ status: 'active' }]);

		await expect.element(testPage.getByText('Stripe onboarding finished.', { exact: true })).toBeVisible();
		await expect
			.element(testPage.getByText("We're checking with Stripe again", { exact: false }))
			.not.toBeInTheDocument();
	});

	it('does not read again once the status has settled', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		await setupSequence([{ status: 'active' }]);
		expect(connectCallCount()).toBe(1);

		const totalDelay = CONNECT_STATUS_POLL_DELAYS_MS.reduce((sum, ms) => sum + ms, 0);
		await vi.advanceTimersByTimeAsync(totalDelay);

		expect(connectCallCount()).toBe(1);
	});

	it('does not read again without the return parameter, even for a status that could still move', async () => {
		vi.useFakeTimers();
		await setupSequence([{ status: 'pending' }]);
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
		await setupSequence([{ status: 'pending' }]);

		const totalDelay = CONNECT_STATUS_POLL_DELAYS_MS.reduce((sum, ms) => sum + ms, 0);
		await vi.advanceTimersByTimeAsync(totalDelay);
		expect(connectCallCount()).toBe(1 + CONNECT_STATUS_POLL_DELAYS_MS.length);

		await vi.advanceTimersByTimeAsync(60_000);
		expect(connectCallCount()).toBe(1 + CONNECT_STATUS_POLL_DELAYS_MS.length);
	});

	it('stops re-reading once the screen is left', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		const { unmount } = await setupSequence([{ status: 'pending' }]);
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
		// Unmounts before the awaited initial fetch settles, and before
		// `pollHandle` has anything to stop -- the guard this is proving is
		// the `destroyed` flag onMount checks between its own two awaits
		// and the point it would otherwise start the poll.
		const { unmount } = await setupSequence([{ status: 'pending' }]);
		await unmount();
		await vi.advanceTimersByTimeAsync(60_000);

		expect(connectCallCount()).toBe(1);
	});

	it('keeps the last good status on screen and reports failure when a re-read fails', async () => {
		vi.useFakeTimers();
		returnedFromStripe('return');
		await setupSequence([{ status: 'pending' }, 'error']);
		await expect.element(testPage.getByText('Awaiting Stripe review')).toBeVisible();

		await vi.advanceTimersByTimeAsync(CONNECT_STATUS_POLL_DELAYS_MS[0]);

		// Still the same status -- not blanked, and not replaced by
		// FormPage's whole-page loadError, which would drop this text.
		await expect.element(testPage.getByText('Stripe Connect status:')).toBeVisible();
		await expect.element(testPage.getByText('Awaiting Stripe review')).toBeVisible();
		await expect.element(testPage.getByText(CONNECT_STATUS_CHECK_FAILED_MESSAGE)).toBeVisible();
	});

	it('lets the person check the status again on demand, and the Continue button follows the new answer', async () => {
		await setupSequence([
			{ status: 'payouts_restricted', requirementsDue: ['configuration.merchant.mcc'] },
			{ status: 'payouts_restricted', requirementsDue: [] }
		]);
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeVisible();

		await testPage.getByRole('button', { name: 'Check status again' }).click();

		await expect
			.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' }))
			.not.toBeInTheDocument();
	});

	it('reports a failed on-demand check without blanking the status', async () => {
		await setupSequence([{ status: 'not_connected' }, 'error']);
		await testPage.getByRole('button', { name: 'Check status again' }).click();

		await expect.element(testPage.getByText(CONNECT_STATUS_CHECK_FAILED_MESSAGE)).toBeVisible();
		await expect.element(testPage.getByText('Not connected')).toBeVisible();
	});
});

// #271: a Practice-level billing rail choice, readable by any Staff
// member and changeable only by an Owner -- a second fact this screen
// carries alongside Stripe Connect status, not gated by it.
describe('payments settings screen: billing mode (#271)', () => {
	it('shows the current mode with no change control for an Admin', async () => {
		await setup({ roles: ['admin'] });

		await expect.element(testPage.getByText('This Practice bills Clients through Stripe.')).toBeVisible();
		await expect.element(testPage.getByRole('radio', { name: 'By hand' })).not.toBeInTheDocument();
	});

	it('lets an Owner submit a change to an already-established mode', async () => {
		await setup({ roles: ['owner'] });

		await expect.element(testPage.getByText('This Practice bills Clients through Stripe.')).toBeVisible();
		await testPage.getByLabelText('By hand').click();
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect
			.poll(() =>
				apiFetchWithSession.mock.calls.some(
					(call: unknown[]) =>
						call[0] === '/api/practices/practice-1/payments/billing-mode' &&
						(call[1] as RequestInit | undefined)?.method === 'PUT'
				)
			)
			.toBe(true);
	});

	it('reports a failed billing mode change through the same Notice error pattern as the load failure', async () => {
		await setup({ roles: ['owner'] });

		await expect.element(testPage.getByText('This Practice bills Clients through Stripe.')).toBeVisible();
		await testPage.getByLabelText('By hand').click();

		apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
			if (path.endsWith('/payments/billing-mode') && init?.method === 'PUT') {
				return Promise.resolve(new Response('could not save billing mode', { status: 500 }));
			}
			return Promise.resolve(jsonResponse({ billingMode: 'stripe' }));
		});
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByRole('alert')).toBeVisible();
	});

	it('names the not-yet-chosen state rather than a raw null', async () => {
		// A Practice that has raised no Invoice yet: the billing-mode read
		// answers with no `billingMode` field at all rather than a chosen
		// rail. Every other endpoint answers exactly as it does for the
		// rest of this block, which is what `mockApi`'s own defaults say.
		await setup({ roles: ['owner'], billingModeBody: {} });

		await expect
			.element(testPage.getByText('Not chosen yet -- this is asked the first time Staff raises an Invoice.'))
			.toBeVisible();
	});
});
