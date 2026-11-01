import { randomUUID } from 'node:crypto';
import AxeBuilder from '@axe-core/playwright';
import { expect, test, type APIRequestContext, type BrowserContext, type Cookie, type Page } from '@playwright/test';
import { API_URL, PREVIEW_SERVER_ORIGIN } from './ports';
import {
	seedClient,
	seedContractorDoula,
	seedPortalClient,
	signInPortalClient,
	PORTAL_CLIENT_PASSWORD,
	type SeededPortalClient
} from './portalClient';
import { seedEngagement, seedEngagementRequest, seedInvoice } from './stack';
import { signInEnrolled, verifyEmail } from './mfa';
import { seedAccountWithNoPractice, seedFoundingOwner } from './staffSignup';
import { enterPractice, signIn } from './auth';
import { drainUntilMailArrives } from './outboxMail';
import { stubTotpFactor } from './totpStub';
import { WCAG_TAGS } from './wcag';

/**
 * The automated half of the accessibility gate (#447). Everything about
 * why this exists, what it owns, and what it deliberately cannot see is
 * in docs/testing/e2e.md -- read that before adding an assertion here.
 */

/*
 * Every `+page.svelte` under `app/src/routes` is a `key` in this file, or
 * is named in the "Scanned elsewhere" or the "Not scanned" list in
 * `axeInventory.ts` beside it, with the reason (#1810).
 * `src/lib/axeInventory.usage.spec.ts` reads all three and fails the unit
 * suite when a route is in none of them, so a new screen cannot be
 * missed. The lists are a module of their own, not constants here,
 * because a spec file may not export anything.
 *
 * Not a route, so in neither list, but a decision of the same kind:
 *
 * - `/`'s two picker states -- the Staff picker and the portal picker.
 *   `/` itself is scanned, in its signed-out state, in the archetype A
 *   batch below: #678 settled that it carries a shell and moved it into
 *   the `(signed-out)` group to get one, closing the gap `/account` used
 *   to share until #484. The two picker states are not separately
 *   scanned because they render the same nodes as the state that is --
 *   an `<h1>` and a stack of `DoorLink`s, each line joined to its link
 *   by `aria-describedby` (#2027) -- while costing a Staff member with
 *   two Memberships and a Portal Account with two Engagements to
 *   provision. What they add is a `<ul>` around the doors and
 *   `EntryPage` in place of `LandingPage`; the join is asserted directly
 *   in this repo's unit suite rather than here, since axe scans the DOM
 *   instances it is pointed at and not the pattern behind them.
 */

/**
 * One route to scan. `key` is the stable name a KNOWN entry points at --
 * the route pattern, not the provisioned URL, so an allowance survives a
 * new fixture. `h1` is ONE of three ready signals: this is a client-rendered
 * SPA, so `goto` resolves long before the data lands, and axe run against
 * a half-loaded page finds a different set of violations every time --
 * which CI's `retries: 2` would then quietly launder into green.
 *
 * The other two are `LOADING_AFFORDANCE_SELECTOR` and `LOAD_ERROR_SELECTOR`
 * below, and a new row needs nothing for either: `scan` waits on both for
 * every route at once. Read those comments before adding a row -- they
 * are why a row does not have to name a node its own loaded state
 * renders, and why a row whose screen never finishes loading, or never
 * finishes loading successfully, fails here rather than scanning a
 * Skeleton or an error Notice.
 *
 * `reach` is optional, and is what lets a row describe a screen that is
 * a *state* rather than a route (#1240) -- one reached by acting on the
 * page after `goto`, not by navigating anywhere else. `scan` runs it, if
 * present, before its own three ready waits, so a `reach` only has to
 * drive the page to the state those waits (and then axe) should measure
 * -- it does not have to wait for readiness itself. Most rows need none:
 * their `url` already is the screen. TOTP enrollment's second step is
 * the first that does, since it is reached by answering step one's
 * password prompt rather than by navigating to it.
 */
interface Route {
	key: string;
	archetype: string;
	url: string;
	h1: string | RegExp;
	reach?: (page: Page) => Promise<void>;
}

/**
 * A violation that is real, filed, and not fixed here. Every entry names
 * the issue that owns it, so nothing is left as prose. The list is
 * self-emptying: an entry whose rule no longer fires on that route fails
 * the scan, exactly like DataTable.usage.spec.ts's pagination list.
 */
interface Known {
	/**
	 * A Route's `key`, or `'*'` for a violation every route carries.
	 */
	key: string;
	ruleId: string;
	issue: number;
	reason: string;
}

// Empty since #487: every scanned route now sets a real <title> via the
// shared `PageTitle` primitive, so `document-title` no longer needs an
// allowance here. Left as a typed, empty list rather than deleted -- the
// shape stays ready for the next genuinely-known violation.
const KNOWN: Known[] = [];

/*
 * The loading affordance, as one selector every route is held against
 * (#1152). `Skeleton` is the only one in the app, and it is matched by
 * what it means rather than by a test hook: `role="status"` plus
 * `aria-busy="true"` is what `Skeleton.svelte` renders, and the pair
 * cannot be silently dropped without a screen-reader user losing the
 * announcement too. An unlabeled `Skeleton` carries neither (#2248), but it
 * is only ever a second region of one load, drawn and removed with a
 * labeled one, so this selector still sees every load in flight. `Button`'s in-flight state also sets `aria-busy` but
 * carries the button role, so it is not matched here -- and nothing on a
 * freshly-loaded route is submitting anyway.
 *
 * Why the h1 alone was never a ready signal for most of the inventory,
 * what axe actually measured against a held Skeleton, and why this shape
 * was chosen over the other two candidates -- a node named per row, or
 * `FormPage` no longer rendering its title while loading, which it still
 * does -- are all in docs/testing/e2e.md, per this file's own header: the
 * argument lives there, and this file holds the assertion.
 *
 * It is a retrying `expect` rather than #1126's `awaitSettled` because
 * that helper is an in-page DOM predicate written for the vitest-browser
 * checks, and Playwright drives from Node. The two properties that matter
 * are already a retrying locator assertion's: no fixed sleep, and a
 * budget that expires LOUDLY rather than falling through into a
 * measurement -- the same shape the h1 wait above it uses.
 */
const LOADING_AFFORDANCE_SELECTOR = '[role="status"][aria-busy="true"]';

/*
 * The load-failure signal (#1258), and the third state `scan` refuses
 * rather than measures. `FormPage` and `ListPage` render their `<h1>` and
 * no `Skeleton` in their `loadError` branch too -- only a `Notice` beside
 * the title -- so a route whose own data read fails under the e2e stack
 * satisfied both waits above already, and axe scanned the one-`Notice`
 * error screen while the row's key claimed it scanned the form or list.
 * The same shape of miss #1152 fixed for the loading branch, one branch
 * over.
 *
 * `[role="alert"]` alone was tried first and refused on `accept-invite`:
 * its no-token state is `EntryPage`, not a `Template` with a `loadError`
 * prop, and its "This link is not complete." `Notice` is the row's own,
 * deliberately-scanned state ("the form is the page", per that row's own
 * comment below). That Notice's markup -- an `<h1>` plus one
 * `role="alert"` paragraph and nothing else -- is byte-identical to a
 * `Template`'s `loadError` branch, so no selector built from what `Notice`
 * renders can tell the two apart; only the `Template` itself knows which
 * one it is in. `data-load-error` is that fact, stated once on
 * `FormPage`'s and `ListPage`'s own `loadError` branch (their doc
 * comments carry the reasoning) rather than guessed at from a node three
 * layers down -- presence-only, the same shape as Skeleton's own
 * `aria-busy`, and invisible to sight and to assistive tech, so nothing a
 * person meets changes. It holds for every row at once, the same property
 * that picked `LOADING_AFFORDANCE_SELECTOR` over naming a node per row.
 *
 * Ordering matters here, not just narration: this wait runs AFTER the
 * loading-affordance one. A route that goes loading -> loadError would
 * otherwise pass this check during the loading window, before the
 * attribute exists, then pass the loading check once the Skeleton leaves
 * -- and axe would scan the error state regardless. Checking
 * loading-affordance first, then this one, is what makes the pair catch
 * that route in either order it fails.
 */
const LOAD_ERROR_SELECTOR = '[data-load-error]';

async function scan(page: Page, route: Route) {
	await page.goto(route.url);
	await route.reach?.(page);
	await expect(
		page.getByRole('heading', { level: 1, name: route.h1 }),
		`${route.key} never finished loading -- axe would have scanned a skeleton`
	).toBeVisible();
	await expect(
		page.locator(LOADING_AFFORDANCE_SELECTOR),
		`${route.key} still had a loading affordance on screen -- axe would have scanned a Skeleton and reported the placeholder's own clean bill of health as the screen's`
	).toHaveCount(0);
	await expect(
		page.locator(LOAD_ERROR_SELECTOR),
		`${route.key} failed to load -- axe would have scanned the error Notice and reported its own clean bill of health as the screen's`
	).toHaveCount(0);

	const { violations } = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze();

	const found = new Set(violations.map((violation) => violation.id));
	const allowed = KNOWN.filter((k) => k.key === route.key || k.key === '*').map((k) => k.ruleId);

	const unexpected = violations.filter((v) => !allowed.includes(v.id));
	expect(
		unexpected.map(
			(v) => `${v.id} (${v.impact}) on ${v.nodes.length}: ${v.nodes[0]?.target.join(' ')} -- ${v.help}`
		),
		`accessibility violations on ${route.key} (archetype ${route.archetype}, ${route.url})`
	).toEqual([]);

	const stale = allowed.filter((ruleId) => !found.has(ruleId));
	expect(
		stale,
		`${route.key} no longer breaks these rules -- narrow or delete their KNOWN entries`
	).toEqual([]);
}

/*
 * The time budget (#1823). Each test has Playwright's default 30 seconds,
 * and one scan -- `goto`, the three ready waits and axe -- took about 1.2
 * seconds on CI in the run that failed. So a row is a test of its own wherever the session
 * it needs can be provisioned once and shared: the batches below seed in
 * a `beforeAll` and give every row its own test, its own fresh page, and
 * its own 30 seconds. A new row adds a test, and takes nothing from any
 * other row's budget.
 *
 * Until #1823 the Staff batch was one test that looped over thirty rows,
 * and the thirtieth ran out of the shared 30 seconds on CI. Playwright
 * then closed the page under `page.goto('/account')`, which is the
 * `net::ERR_ABORTED; maybe frame was detached?` that run reported.
 *
 * A test that still scans more than one row does so because each row is
 * a later state of the same session, reached by what the row before it
 * did (the no-Practice and verify-first tests, and the contractor's two
 * rows). Each holds three rows or fewer, about ten seconds of the 30. A
 * fourth row there is a test of its own, not a longer budget.
 */

/**
 * Puts a Staff or portal session into the browser, the same injection
 * auth.ts's `enterPractice` makes: nothing here needs an
 * interactive sign-in.
 */
async function enterSession(context: BrowserContext, token: string) {
	await context.addCookies([
		{
			name: '__session',
			value: token,
			url: PREVIEW_SERVER_ORIGIN,
			httpOnly: true,
			secure: false,
			sameSite: 'Lax'
		}
	]);
}

/**
 * A row whose `url` and `reach` are built from what a batch's `beforeAll`
 * seeded. A test's title is fixed when the file loads, before any seed
 * exists, so the row carries a function of the seed rather than a URL.
 */
interface SeededRoute<Seed> extends Omit<Route, 'url' | 'reach'> {
	url: (seed: Seed) => string;
	reach?: (page: Page, seed: Seed) => Promise<void>;
}

function routeFor<Seed>(row: SeededRoute<Seed>, seed: Seed): Route {
	const { reach } = row;
	return {
		...row,
		url: row.url(seed),
		reach: reach && ((page) => reach(page, seed))
	};
}

// Archetype A, and the only batch that needs no fixtures at all: every
// one of these is what a person meets before there is an account. The
// Staff invitation is scanned in its no-token entry state, which is the
// state a person actually arrives in when the link is stale -- the form
// is the page.
//
// The four link screens below (#1810) read their token only when she
// presses the button, never on arrival, so a made-up token shows exactly
// the screen a fresh link shows. Each is scanned in that state, the one a
// person opening the mail meets. `verify-email` is the exception: it
// spends its token on arrival, so it is scanned here in its failed-link
// state, with no token; the verified state is scanned in the verify-first
// test below, signed in. Signed out, that state differs from it by one
// link's label.
const LINK_TOKEN = 'a11y-scan-token';
const SIGNED_OUT_ROUTES: Route[] = [
	{ key: '/', archetype: 'A', url: '/', h1: 'Sign in or set up a Practice' },
	{ key: 'login', archetype: 'A', url: '/login', h1: 'Sign in' },
	// #2049: setting up a Practice is three question pages and a check
	// page. Each is reached by its address with nothing typed, which is
	// the state a person who follows a link to one of them meets.
	{
		key: 'signup',
		archetype: 'E',
		url: '/signup',
		h1: 'About you'
	},
	{
		key: 'signup/practice',
		archetype: 'E',
		url: '/signup/practice',
		h1: /What is your Practice called\? \(optional\)$/
	},
	{
		key: 'signup/where',
		archetype: 'E',
		url: '/signup/where',
		h1: 'Where do you work?'
	},
	{
		key: 'signup/check',
		archetype: 'E',
		url: '/signup/check',
		h1: 'Check your answers'
	},
	{
		key: 'accept-invite',
		archetype: 'A',
		url: '/accept-invite',
		h1: 'Accept your invite'
	},
	{
		key: 'offers/[offerId]',
		archetype: 'A',
		url: '/offers/00000000-0000-0000-0000-000000000000',
		h1: 'An offer of work'
	},
	{ key: 'portal/login', archetype: 'A', url: '/portal/login', h1: 'Sign in' },
	// #694: the way back for a locked-out person, and one of the few
	// screens somebody meets while already in trouble -- which is
	// exactly when an accessibility defect costs the most.
	{
		key: 'recovery-code',
		archetype: 'A',
		url: '/recovery-code',
		h1: 'Use a recovery code'
	},
	{
		key: 'forgot-password',
		archetype: 'A',
		url: '/forgot-password',
		h1: 'Forgot your password?'
	},
	{
		key: 'reset-password',
		archetype: 'A',
		url: `/reset-password?token=${LINK_TOKEN}`,
		h1: 'Reset your password'
	},
	{
		key: 'verify-email (signed out, failed link)',
		archetype: 'A',
		url: '/verify-email',
		h1: 'Verify your email',
		reach: (page) => expect(page.getByText('This link is not complete.')).toBeVisible()
	},
	{
		key: 'portal/accept-invite',
		archetype: 'A',
		url: `/portal/accept-invite?token=${LINK_TOKEN}`,
		h1: 'Accept your portal invite',
		reach: (page) => expect(page.getByRole('button', { name: 'Continue' })).toBeVisible()
	},
	{
		key: 'portal/sign-in',
		archetype: 'A',
		url: `/portal/sign-in?token=${LINK_TOKEN}`,
		h1: /^Sign in$/,
		reach: (page) => expect(page.getByRole('button', { name: 'Continue' })).toBeVisible()
	},
	{
		key: 'portal/confirm-sign-in-address',
		archetype: 'A',
		url: `/portal/confirm-sign-in-address?token=${LINK_TOKEN}`,
		h1: 'Confirm your sign-in address',
		reach: (page) => expect(page.getByRole('button', { name: 'Continue' })).toBeVisible()
	}
];

test.describe('Archetype A -- the screens a person meets signed out', () => {
	for (const route of SIGNED_OUT_ROUTES) {
		test(route.key, async ({ page }) => {
			await scan(page, route);
		});
	}
});

// Archetype A's two outliers (#745, #749, #1114): `/no-practice` and
// `mfa/enroll` each need a live session, unlike every route in the
// no-fixture batch above. Opened with no session at all, each hits its
// own onMount redirect to /login (see the pages' doc comments) and a
// scan there would measure the login screen, not these -- so the session
// here has to resolve to no Practice specifically, not to no session.
// seedAccountWithNoPractice (staffSignup.ts) provisions exactly that: an
// Identity Platform account exchanged for a session with no POST
// /api/staff/signup in between. The cookie is injected straight into the
// browser context -- the same shape auth.ts's enterPractice
// uses -- since nothing here needs an interactive sign-in.
//
// Both behind one seed rather than a test each: they want the identical
// session, and provisioning it twice would buy nothing but a second
// account. `mfa/enroll` reads that session only for the email it
// re-authenticates with, so a Practice-less one realizes its step one
// exactly as a Practice-bearing one would -- see the comment ahead of
// its own scan below for what that costs and how it is answered.
//
// Step two joins step one here rather than in a test of its own (#1240):
// it shares this same session, and reaching it is exactly what a `reach`
// on the step-two row below is for. `stubTotpFactor` (totpStub.ts) is
// installed ahead of both `mfa/enroll` scans -- step one's submit opens
// a real TOTP enrollment session with Identity Platform, and the
// vendored emulator has no such path (#1132) -- even though only the
// second scan's `reach` ever exercises it.
test('Archetype A -- the two screens behind a session with no Practice', async ({
	page,
	request,
	context
}) => {
	const { headers, password, email, localId } = await seedAccountWithNoPractice(request);
	// #1504: an unverified address is shown the verify-first step, so step
	// two's QR code is reached only once the address is verified. The
	// verify-first step itself is scanned in the next test (#1782).
	await verifyEmail(request, localId);
	await enterSession(context, headers.Cookie.replace('__session=', ''));

	await scan(page, {
		key: 'no-practice',
		archetype: 'A',
		url: '/no-practice',
		h1: 'Your account is not part of a Practice'
	});

	// `/api/staff/session` is answered here rather than left to the real
	// endpoint, which 404s for this identity -- see docs/testing/e2e.md's own
	// account of the race that answer closes, ahead of this session's
	// `mfa/enroll` scans below. Installed after the no-practice scan
	// above, which depends on the real 404 for its own screen.
	await page.route(
		(url) => url.pathname === '/api/staff/session',
		(route) => route.fulfill({ status: 200, json: { email } })
	);
	await stubTotpFactor(page, request);

	await scan(page, {
		key: 'mfa/enroll',
		archetype: 'A',
		url: '/mfa/enroll',
		h1: 'Set up two-factor authentication'
	});

	// #1240: reached by answering step one's password prompt rather than
	// by navigating anywhere, so `reach` drives that submit and waits for
	// the QR code step two renders -- the state the three ready waits
	// below and axe should measure, not step one's own screen (which the
	// h1 alone cannot tell apart from this, since both steps share one
	// title).
	await scan(page, {
		key: 'mfa/enroll (step two)',
		archetype: 'A',
		url: '/mfa/enroll',
		h1: 'Set up two-factor authentication',
		reach: async (page) => {
			await page.getByLabel('Password').fill(password);
			await page.getByRole('button', { name: 'Continue' }).click();
			await expect(
				page.getByRole('img', {
					name: 'QR code for setting up two-factor authentication in an authenticator app'
				})
			).toBeVisible();
		}
	});

	/*
	 * The shell itself, asserted rather than left to the scan above
	 * (#1114). axe's `landmark-one-main` and `region` rules live under
	 * its `best-practice` tag, which WCAG_TAGS deliberately leaves out --
	 * so a clean scan of this route would have been just as clean while
	 * it still rendered no shell at all, which is the gap #1114 closed.
	 * These three are what "renders inside a shell" means, named where a
	 * regression would show: the skip link, the reduced bar, and the
	 * `<main>` the skip link lands in.
	 */
	await expect(page.getByRole('link', { name: 'Skip to main content' })).toBeAttached();
	await expect(page.getByRole('banner')).toBeVisible();
	await expect(page.getByRole('main')).toBeVisible();
});

// #1782: the verify-first step of `mfa/enroll` (#1504), and the link that
// ends it. Since #1531 a new Owner enters her Practice straight after
// signup with an unverified address and no second factor, so the person
// who meets this step is that Owner, opening enrollment from `/account`
// or a `SecondFactorNeeded` link. A founding Owner is seeded for it
// rather than the Practice-less session above: her session lookup is
// real, so neither screen needs the stand-in answer that test installs,
// and signup itself queues the verification mail whose link
// `/verify-email` is scanned with.
//
// No `stubTotpFactor`: the verify-first step is shown before any TOTP
// enrollment session is opened, so nothing here reaches the path the
// emulator lacks.
test('Archetype A -- the verify-first step and its link, for an Owner whose email is not verified', async ({
	page,
	request,
	context
}) => {
	const { email, password, idToken } = await seedFoundingOwner(request);
	const headers = await signIn(request, API_URL, idToken);
	await enterSession(context, headers.Cookie.replace('__session=', ''));

	// Step one's password prompt is answered the same way the step-two row
	// above answers it. The address is unverified, so the step that
	// follows is the verify-first one, which the `<h2>` names: the `<h1>`
	// is the same on every step.
	await scan(page, {
		key: 'mfa/enroll (verify-first)',
		archetype: 'A',
		url: '/mfa/enroll',
		h1: 'Set up two-factor authentication',
		reach: async (page) => {
			await page.getByLabel('Password').fill(password);
			await page.getByRole('button', { name: 'Continue' }).click();
			await expect(
				page.getByRole('heading', {
					level: 2,
					name: 'Verify your email address first'
				})
			).toBeVisible();
		}
	});

	// The verification mail signup queued for her. Drained until it
	// arrives, not once (#1141): outboxMail.ts says why.
	const message = await drainUntilMailArrives(
		request,
		'process-staff-token-mail-outbox',
		email,
		'Verify your email address'
	);
	const linkToken = /\/verify-email\?token=(\S+)/.exec(message.text)?.[1];
	expect(linkToken, `no verification link in the mail:\n${message.text}`).toBeTruthy();

	// The link opened in the browser she signed up in, so she is still
	// signed in. `reach` waits for the verified state: "Checking your
	// link…" is an info Notice, not a Skeleton, and every state shares the
	// `<h1>`, so neither ready wait in `scan` can tell the two apart.
	await scan(page, {
		key: 'verify-email (signed in)',
		archetype: 'A',
		url: `/verify-email?token=${linkToken}`,
		h1: 'Verify your email',
		reach: async (page) => {
			await expect(page.getByText('Your email address is verified.')).toBeVisible();
			await expect(page.getByRole('link', { name: 'Go to your Practice' })).toBeVisible();
		}
	});
});

/**
 * What the Staff batch's `beforeAll` seeds, and every one of its rows is
 * built from.
 */
interface StaffSeed {
	practiceId: string;
	staffId: string;
	engagementId: string;
	// Jane Smith, the plain Client.
	clientId: string;
	// Casey Pending, the Client with a pending Engagement Request.
	pendingRequestClientId: string;
	pendingRequestId: string;
	// Pat Client's email, which the edit-path duplicate row collides on.
	patEmail: string;
	// The Owner's `__session` value, with a second factor.
	enrolledToken: string;
	// The same Owner's `__session` value from before she enrolled one, with
	// no second factor: the session a new Owner holds since #1531 (#1857).
	noSecondFactorToken: string;
}

// The one section the Practice's Client Field Template has, so the details
// journey has a section page to scan (#1810). With no section the route
// is never reached: SectionQuestion.svelte sends it to the check page.
const CLIENT_FIELD_SECTION = 'Birth history';

async function seedStaffSide(request: APIRequestContext): Promise<StaffSeed> {
	const seeded = await seedPortalClient(request, 'Riverside Doulas');
	const { practiceId, staffId, engagementId } = seeded;

	// #280: archetype G, the same as the portal's own Birth Plan below --
	// provisioned through the API first, same as that one, so the scan
	// covers the document view rather than the "none yet" empty state.
	const birthPlan = await request.post(
		`${API_URL}/api/practices/${practiceId}/engagements/${engagementId}/plans/birth_plan`,
		{ headers: seeded.staffHeaders }
	);
	expect(birthPlan.ok(), `create birth plan failed: ${birthPlan.status()} ${await birthPlan.text()}`).toBe(true);

	// A second, plain Client -- no portal account -- for the three
	// clients/[clientId] screens (#516). Given an Engagement of her own so
	// the hub's Engagements table scans with a row in it rather than its
	// empty message, which is a different set of nodes for axe to read.
	const clientId = await seedClient(request, practiceId, seeded.staffHeaders, {
		givenName: 'Jane',
		familyName: 'Smith'
	});
	seedEngagement(clientId, practiceId);

	// A third Client, held in its own pending-Request state (#535): the
	// hub's Withdraw block (a Notice describing the pending Request, plus
	// a Withdraw Button shown only to the Request's own requester) never
	// gets scanned otherwise, since Jane above carries only a live
	// Engagement. Seeded directly via seedEngagementRequest rather than
	// through the real endpoint -- the signed-in Owner already holds
	// approval authority, so driving POST .../engagement-requests would
	// collapse straight to 'approved' and never leave a Request pending.
	// A separate Client, not a second kind on Jane, so this fixture's
	// blast radius stays off every other route that reuses her clientId.
	// Its note is left unset (#528): a value holding one long unbreakable
	// token (e.g. a bare URL) trips a known DescriptionList overflow on
	// the approval screen below, tracked and fixed separately as #530.
	//
	// The returned id also seeds the approval screen route further down --
	// this is the only pending Request the Staff-session test has, so
	// both the hub's Withdraw block and the approval screen scan the same
	// fixture rather than paying for a second Request.
	const pendingRequestClientId = await seedClient(request, practiceId, seeded.staffHeaders, {
		givenName: 'Casey',
		familyName: 'Pending'
	});
	const pendingRequestId = seedEngagementRequest(pendingRequestClientId, practiceId, staffId);

	// #1810: one section, with one field under it, for the details
	// journey's section page. It also puts a row in the Client Fields
	// settings list, and the field on Jane's details, so those rows scan
	// a Practice that has added something rather than none.
	const fields = await request.put(`${API_URL}/api/practices/${practiceId}/client-field-template`, {
		headers: seeded.staffHeaders,
		data: {
			fields: [
				{
					id: 'birth-history',
					type: 'section_header',
					label: CLIENT_FIELD_SECTION
				},
				{ id: 'earlier-births', type: 'short_text', label: 'Earlier births' }
			]
		}
	});
	expect(fields.ok(), `set client fields failed: ${fields.status()} ${await fields.text()}`).toBe(true);

	const enrolledHeaders = await signInEnrolled(request, seeded.staffIdToken, seeded.staffLocalId);

	return {
		practiceId,
		staffId,
		engagementId,
		clientId,
		pendingRequestClientId,
		pendingRequestId,
		patEmail: seeded.clientEmail,
		// The rows are scanned as an Owner WITH a second factor, enrolled
		// here rather than in seedPortalClient, whose Owner has none
		// (#1854). The rows that reach one of the five acts draw their full
		// Owner branch only for her: `settings/mfa` (the toggle),
		// `settings/delete` (the delete form), `staff/[staffId]/mfa-recovery`
		// (the vouch form), `settings` and `agreement` (the export action),
		// and `clients/[clientId]` (erasure). For an Owner with no second
		// factor each draws SecondFactorNeeded instead, and the landing page
		// adds a recommendation; the NO_SECOND_FACTOR_ROUTES rows scan those
		// branches (#1857), under seedPortalClient's own session, from before
		// she enrolled. The setup calls above need no second factor, so they
		// use that session too. Every other test in this file that calls
		// seedPortalClient runs under it, with no second factor.
		enrolledToken: enrolledHeaders.Cookie.replace('__session=', ''),
		noSecondFactorToken: seeded.staffHeaders.Cookie.replace('__session=', '')
	};
}

const STAFF_ROUTES: SeededRoute<StaffSeed>[] = [
	{
		key: 'practices/[practiceId]',
		archetype: 'B',
		url: (s) => `/practices/${s.practiceId}`,
		h1: 'Overview'
	},
	{
		key: 'practices/[practiceId]/clients',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/clients`,
		h1: 'Clients'
	},
	{
		key: 'practices/[practiceId]/invoices',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/invoices`,
		h1: 'Invoices'
	},
	{
		key: 'practices/[practiceId]/billing',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/billing`,
		h1: 'Credits'
	},
	{
		key: 'practices/[practiceId]/staff',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/staff`,
		h1: 'Staff'
	},
	{
		// #694: the Owner-vouch screen, scanned against the Owner's own
		// roster row -- this Practice has exactly one Staff member, and
		// the endpoint takes any Membership at the Practice, so no second
		// person has to be seeded to realize the screen.
		key: 'practices/[practiceId]/staff/[staffId]/mfa-recovery',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/staff/${s.staffId}/mfa-recovery`,
		h1: 'Help Alex Owner sign in again'
	},
	{
		// #2041: a Staff member's own page, the roster's one row action,
		// scanned on the Owner's own row for the reason the row above gives.
		// Her Owner session draws every section, the Access one included.
		key: 'practices/[practiceId]/staff/[staffId]',
		archetype: 'D',
		url: (s) => `/practices/${s.practiceId}/staff/${s.staffId}`,
		h1: 'Alex Owner'
	},
	{
		// #2041: the two question pages that page's Changes open.
		key: 'practices/[practiceId]/staff/[staffId]/roles',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/staff/${s.staffId}/roles`,
		h1: 'Which roles does Alex Owner have?'
	},
	{
		key: 'practices/[practiceId]/staff/[staffId]/employment-type',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/staff/${s.staffId}/employment-type`,
		h1: 'Is Alex Owner an employee or a contractor?'
	},
	{
		// #2006: the whole activity ledger the Overview links to.
		key: 'practices/[practiceId]/activity',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/activity`,
		h1: 'Activity'
	},
	{
		key: 'practices/[practiceId]/offers',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/offers`,
		h1: 'Your offers'
	},
	{
		key: 'practices/[practiceId]/engagement-requests',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/engagement-requests`,
		h1: 'Requests awaiting approval'
	},
	{
		key: 'practices/[practiceId]/engagements/[engagementId]',
		archetype: 'D',
		url: (s) => `/practices/${s.practiceId}/engagements/${s.engagementId}`,
		h1: 'Pat'
	},
	{
		// #280: the Birth Plan's own address on the Practice side,
		// mirroring the portal's own G-archetype Birth Plan route below.
		key: 'practices/[practiceId]/engagements/[engagementId]/birth-plan',
		archetype: 'G',
		url: (s) => `/practices/${s.practiceId}/engagements/${s.engagementId}/birth-plan`,
		// clientDisplayName is personname.Full (#2350): this seed's given
		// and family names, "Pat Client".
		h1: "Pat Client's Birth Plan"
	},
	{
		key: 'practices/[practiceId]/clients/[clientId]',
		archetype: 'D',
		url: (s) => `/practices/${s.practiceId}/clients/${s.clientId}`,
		h1: 'Jane Smith'
	},
	{
		// #535: the same hub, scanned a second time for a Client whose
		// pending Engagement Request was requested by this signed-in
		// session -- the only state that puts the Withdraw Button and
		// its surrounding pending-Request Notice in the DOM.
		key: 'practices/[practiceId]/clients/[clientId] (pending request)',
		archetype: 'D',
		url: (s) => `/practices/${s.practiceId}/clients/${s.pendingRequestClientId}`,
		h1: 'Casey Pending'
	},
	{
		// #528: the approval screen for the same pending Request above.
		// Unlike the Engagement Request form's anchored row below, no
		// regex/exact match is needed here even though the h1 changes
		// once the record loads (from "Approve an Engagement Request"
		// to "Approve work with Casey Pending"): neither string is a
		// substring of the other, so Playwright's default substring
		// match can never lock onto the earlier, half-loaded heading.
		key: 'practices/[practiceId]/engagement-requests/[requestId]',
		archetype: 'D',
		url: (s) => `/practices/${s.practiceId}/engagement-requests/${s.pendingRequestId}`,
		h1: 'Approve work with Casey Pending'
	},
	{
		// Anchored, unlike every other h1 here: the heading is the submit
		// button's own label, and the Doula wording ("Ask to start work
		// with Jane Smith") renders in the window between the Client
		// landing and the session roles arriving. A substring match --
		// Playwright's default -- would accept that intermediate heading
		// and let axe scan a page still one fetch from done.
		key: 'practices/[practiceId]/clients/[clientId]/engagement-requests/new',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/${s.clientId}/engagement-requests/new`,
		h1: /^Start work with Jane Smith$/
	},
	{
		// #1610: her details journey, opened through its door the way
		// her Engagement's link opens it, at its first question (#2048).
		key: 'practices/[practiceId]/clients/[clientId]/details/name',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/${s.clientId}/details`,
		h1: "What is Jane's name?"
	},
	{
		key: 'practices/[practiceId]/clients/[clientId]/details/date-of-birth',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/${s.clientId}/details/date-of-birth`,
		h1: /date of birth\? \(optional\)$/
	},
	{
		key: 'practices/[practiceId]/clients/[clientId]/details/check',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/${s.clientId}/details/check`,
		h1: /^Check Jane's details/
	},
	{
		key: 'practices/[practiceId]/clients/search',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/search`,
		h1: 'Find a Client'
	},
	{
		key: 'practices/[practiceId]/clients/new/name',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/new/name`,
		h1: "What is the Client's name?"
	},
	{
		key: 'practices/[practiceId]/invite',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/invite`,
		h1: 'Invite a Staff member'
	},
	{
		key: 'practices/[practiceId]/settings',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings`,
		h1: 'Settings'
	},
	{
		key: 'practices/[practiceId]/settings/payments',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/payments`,
		h1: 'Getting paid'
	},
	// #2046: Getting paid's payment terms, a question page of its own.
	{
		key: 'practices/[practiceId]/settings/payments/payment-terms',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/settings/payments/payment-terms`,
		h1: 'How many days does a Client have to pay an Invoice?'
	},
	{
		key: 'practices/[practiceId]/settings/contract-template',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/contract-template`,
		h1: 'Contract Template'
	},
	// #966: scanned with no rate set at all, the state every fresh
	// Practice actually starts in and the one its own AC names as
	// valid -- not seeded through the API first, unlike the birth plan
	// above, because the "none yet" state here is the representative
	// one to prove accessible rather than one to skip past.
	{
		key: 'practices/[practiceId]/settings/rates',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/rates`,
		h1: 'Rates'
	},
	{
		key: 'practices/[practiceId]/settings/plan-templates',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/plan-templates`,
		h1: 'Plan Templates'
	},
	// #2193: "Add a question" asks the kind of answer on a page of its own,
	// then opens the new question on its own page. A question not yet
	// saved needs no seeded id, so it is the one scanned; a saved one adds
	// the place select and Remove, both atoms already scanned elsewhere.
	{
		key: 'practices/[practiceId]/settings/plan-templates/[planType]/kind',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/settings/plan-templates/care_plan/kind`,
		h1: 'What kind of answer do you need?'
	},
	{
		key: 'practices/[practiceId]/settings/plan-templates/[planType]/[questionId]',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/settings/plan-templates/care_plan/new?kind=single_select`,
		h1: 'Add a question'
	},
	// The settings screens from here down postdate #405's A-G survey --
	// the first two arrived with #452's Settings hub -- so they are
	// archetype F by shape rather than by that table.
	{
		key: 'practices/[practiceId]/settings/website',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/website`,
		h1: 'Your website'
	},
	{
		key: 'practices/[practiceId]/settings/client-fields',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/client-fields`,
		h1: 'Client Fields'
	},
	// #2193: the Client Fields' own kind and question pages, as above.
	{
		key: 'practices/[practiceId]/settings/client-fields/kind',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/settings/client-fields/kind`,
		h1: 'What kind of answer do you need?'
	},
	{
		key: 'practices/[practiceId]/settings/client-fields/[questionId]',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/settings/client-fields/new?kind=multi_select`,
		h1: 'Add a question'
	},
	{
		// #871: postdates the #405 survey the same way website/client-fields
		// do -- archetype F by shape. Nothing pending on this seeded
		// Practice, so this scans the ordinary "ready to delete" state,
		// not the locked one.
		key: 'practices/[practiceId]/settings/delete',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/delete`,
		h1: 'Delete this Practice'
	},
	{
		// #894: archetype F by shape, like the settings screens above
		// that postdate #405's survey. This batch signs in as the
		// founding Owner, so the screen renders its Owner branch -- the
		// status Badge, the Staff-count sentence and the toggle -- and
		// not the one-Notice screen a non-Owner meets. In its default
		// "optional for other Staff" state, since a fresh Practice
		// starts there and the required state differs by a Badge label
		// and a Button label, not by a node axe reads differently.
		key: 'practices/[practiceId]/settings/mfa',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/mfa`,
		h1: 'Multi-factor authentication'
	},
	{
		// Scoped to the person, not this Practice (#437), so it sits
		// outside the practiceId group every other route above is under
		// -- the one route in this loop not built from `practiceId`. Now
		// carries the same shell as the rest of them (#484).
		key: 'account',
		archetype: 'D',
		url: () => '/account',
		h1: 'Your account'
	},
	// #2044: the summary's two Changes, each a question page of its own.
	{
		key: 'account/name',
		archetype: 'E',
		url: () => '/account/name',
		h1: 'What is your name?'
	},
	{
		key: 'account/work-state',
		archetype: 'E',
		url: () => '/account/work-state',
		h1: 'Which state do you work from?'
	},
	// #1810: the screens that were in neither this inventory nor the
	// not-scanned list. Archetype by shape: a ListPage is C, a
	// FormPage under settings is F, a question page is E.
	{
		key: 'practices/[practiceId]/contracts',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/contracts`,
		h1: 'Contracts'
	},
	{
		key: 'practices/[practiceId]/on-call',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/on-call`,
		h1: 'On call'
	},
	{
		key: 'practices/[practiceId]/schedule',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/schedule`,
		h1: 'Schedule'
	},
	{
		key: 'practices/[practiceId]/settings/blocked-addresses',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/blocked-addresses`,
		h1: 'Blocked email addresses'
	},
	{
		key: 'practices/[practiceId]/settings/name',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/name`,
		h1: 'Practice name'
	},
	{
		key: 'practices/[practiceId]/settings/on-call',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/on-call`,
		h1: 'On call'
	},
	{
		key: 'practices/[practiceId]/settings/timezone',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/timezone`,
		h1: 'Timezone'
	},
	{
		key: 'practices/[practiceId]/clients/[clientId]/details/email',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/${s.clientId}/details/email`,
		h1: "What is Jane's email address? (optional)"
	},
	{
		key: 'practices/[practiceId]/clients/[clientId]/details/phone',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/${s.clientId}/details/phone`,
		h1: "What is Jane's phone number? (optional)"
	},
	{
		key: 'practices/[practiceId]/clients/[clientId]/details/address',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/${s.clientId}/details/address`,
		h1: "What is Jane's address? (optional)"
	},
	{
		key: 'practices/[practiceId]/clients/[clientId]/details/sections/[sectionIndex]',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/${s.clientId}/details/sections/0`,
		h1: `${CLIENT_FIELD_SECTION} (optional)`
	},
	{
		// Intake's duplicate question, reached the way a person reaches it:
		// the name of a Client already on file, saved. Nothing is written --
		// the save stops at the question (new/duplicate's doc comment).
		key: 'practices/[practiceId]/clients/new/duplicate',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/new/name`,
		h1: 'Is this the same person?',
		reach: async (page) => {
			await expect(
				page.getByRole('heading', {
					level: 1,
					name: "What is the Client's name?"
				})
			).toBeVisible();
			await expect(page.locator(LOADING_AFFORDANCE_SELECTOR)).toHaveCount(0);
			await page.getByLabel('Given name').fill('Jane');
			await page.getByLabel('Family name (optional)').fill('Smith');
			await page.getByRole('button', { name: 'Save and continue' }).click();
			await expect(page).toHaveURL(/\/clients\/new\/duplicate$/);
		}
	},
	{
		// The edit path's possible-duplicate question (gate two, #814), on
		// the details journey since #2048: Jane's email changed to Pat's,
		// from the Change on her record's email row, collides on the email
		// alone, which is not a name substitution, so the save goes to this
		// page rather than to the substitution dialog. Nothing is written.
		key: 'practices/[practiceId]/clients/[clientId]/details/duplicate',
		archetype: 'E',
		url: (s) => `/practices/${s.practiceId}/clients/${s.clientId}/details?step=email`,
		h1: 'Is this the same person?',
		reach: async (page, { patEmail }) => {
			// The question fills in from her record once it loads, so it is
			// typed into only after that.
			await expect(
				page.getByRole('heading', { level: 1, name: "What is Jane's email address? (optional)" })
			).toBeVisible();
			await expect(page.locator(LOADING_AFFORDANCE_SELECTOR)).toHaveCount(0);
			await page.getByRole('textbox').fill(patEmail);
			await page.getByRole('button', { name: 'Save' }).click();
			await expect(page).toHaveURL(/\/details\/duplicate$/);
		}
	}
];

/**
 * The wait for SecondFactorNeeded's own link, which the branch with a
 * second factor never draws.
 */
const awaitSecondFactorNeeded = (page: Page) =>
	expect(page.getByRole('link', { name: 'Set up two-factor authentication' })).toBeVisible();

/*
 * The screens an Owner with no second factor meets (#1857), scanned under
 * `noSecondFactorToken`. Since #1531 a new Owner enters her Practice with
 * none, so these are the branches she meets first: SecondFactorNeeded in
 * place of each act that needs one, and the recommendation on the landing
 * page.
 *
 * Every row has a `reach`, because each branch has the same `<h1>` as the
 * one STAFF_ROUTES scans, and on the landing page, `settings/mfa` and
 * `clients/[clientId]` the branch is drawn from a read that lands after
 * it. Without the wait, axe could read the screen before the branch is on
 * it. Each screen but the landing page has one "Set up two-factor
 * authentication" link, the one SecondFactorNeeded draws, so the link is
 * the wait (`awaitSecondFactorNeeded`, above). The agreement row is a
 * test of its own below, because its stand-in has to be in place before
 * `goto`.
 */
const NO_SECOND_FACTOR_ROUTES: SeededRoute<StaffSeed>[] = [
	{
		// #1533: seedPortalClient's Engagement is First Value, so the
		// recommendation shows, and nothing in this file closes it.
		key: 'practices/[practiceId] (no second factor)',
		archetype: 'B',
		url: (s) => `/practices/${s.practiceId}`,
		h1: 'Overview',
		reach: (page) =>
			expect(page.getByRole('heading', { level: 2, name: 'Add a second step when you sign in' })).toBeVisible()
	},
	{
		key: 'practices/[practiceId]/staff/[staffId]/mfa-recovery (no second factor)',
		archetype: 'C',
		url: (s) => `/practices/${s.practiceId}/staff/${s.staffId}/mfa-recovery`,
		h1: 'Help Alex Owner sign in again',
		reach: awaitSecondFactorNeeded
	},
	{
		key: 'practices/[practiceId]/clients/[clientId] (no second factor)',
		archetype: 'D',
		url: (s) => `/practices/${s.practiceId}/clients/${s.clientId}`,
		h1: 'Jane Smith',
		reach: awaitSecondFactorNeeded
	},
	{
		key: 'practices/[practiceId]/settings (no second factor)',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings`,
		h1: 'Settings',
		reach: awaitSecondFactorNeeded
	},
	{
		key: 'practices/[practiceId]/settings/delete (no second factor)',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/delete`,
		h1: 'Delete this Practice',
		reach: awaitSecondFactorNeeded
	},
	{
		key: 'practices/[practiceId]/settings/mfa (no second factor)',
		archetype: 'F',
		url: (s) => `/practices/${s.practiceId}/settings/mfa`,
		h1: 'Multi-factor authentication',
		reach: awaitSecondFactorNeeded
	}
];

/**
 * Makes the agreement screen due (#1549). The e2e stack has no material
 * change to the Terms of Service or the Privacy Policy, so the Practice's
 * session read is answered with one due version added to the real
 * answer: the same network-boundary stand-in the `mfa/enroll` rows use for
 * `/api/staff/session`. That read is the only place the screen learns
 * what is due (its own doc comment); everything else on it is the real
 * session.
 */
async function makeAgreementDue(page: Page, practiceId: string) {
	await page.route(
		(url) => url.pathname === `/api/practices/${practiceId}/session`,
		async (route) => {
			const response = await route.fetch();
			const session = await response.json();
			await route.fulfill({
				response,
				json: {
					...session,
					agreementDue: [
						{
							document: 'terms',
							name: 'Terms of Service',
							version: '2026-11-01',
							change: 'Raises the price of a Credit, for every Credit bought from that day.'
						}
					]
				}
			});
		}
	);
}

// Archetypes B, C, D, E, F and G, all behind one Staff session, seeded once
// in `beforeAll`: signup + login is ~4s of the same work every time, and
// none of these scans depends on any other having run. Each row is its
// own test, for the budget the comment above `enterSession` gives.
test.describe('Archetypes B, C, D, E, F, G -- the Staff side', () => {
	let seed: StaffSeed;

	test.beforeAll(async ({ request }) => {
		seed = await seedStaffSide(request);
	});

	// The budget is one row per test: 30 seconds for a scan that took about
	// 1.2 seconds on CI's failed run 40055897529 (its trace has twenty-three
	// scans in 28 seconds). Seeding is in the hook, which has 30 seconds of
	// its own, and does not grow with the rows.
	for (const row of STAFF_ROUTES) {
		test(row.key, async ({ page, context }) => {
			await enterSession(context, seed.enrolledToken);
			await scan(page, routeFor(row, seed));
		});
	}

	// #1549 (ADR-0053): the screen an Owner meets after a material change
	// to the Terms of Service or the Privacy Policy starts. The BFF reads
	// them from `legal`'s own version list, and none is material yet, so
	// `makeAgreementDue` makes the screen due.
	test('practices/[practiceId]/agreement', async ({ page, context }) => {
		await makeAgreementDue(page, seed.practiceId);
		await enterSession(context, seed.enrolledToken);
		await scan(page, {
			key: 'practices/[practiceId]/agreement',
			// EntryPage, the archetype A frame, though it is behind a session.
			archetype: 'A',
			url: `/practices/${seed.practiceId}/agreement`,
			h1: 'The Terms of Service have changed'
		});
	});

	// #1857: the same Owner, with no second factor. Her session is put in
	// each test's own fresh page, the same as the rows above.
	for (const row of NO_SECOND_FACTOR_ROUTES) {
		test(row.key, async ({ page, context }) => {
			await enterSession(context, seed.noSecondFactorToken);
			await scan(page, routeFor(row, seed));
		});
	}

	// The agreement screen's export line, which gives way to
	// SecondFactorNeeded for her.
	test('practices/[practiceId]/agreement (no second factor)', async ({ page, context }) => {
		await makeAgreementDue(page, seed.practiceId);
		await enterSession(context, seed.noSecondFactorToken);
		await scan(page, {
			key: 'practices/[practiceId]/agreement (no second factor)',
			archetype: 'A',
			url: `/practices/${seed.practiceId}/agreement`,
			h1: 'The Terms of Service have changed',
			reach: awaitSecondFactorNeeded
		});
	});
});

// Archetype E's contractor branch (#525, #539, ADR-0017): the explainer
// door #501 built in place of the search screen for a Staff member who
// holds the `doula` role and a contractor employment type, with neither
// `owner` nor `admin`. A separate test rather than a third route in the
// loop above: that loop is scanned under the Owner's own session, and
// this branch renders only under a session the Owner can never hold.
test('Archetype E -- the contractor Doula door onto clients/search', async ({ page, request }) => {
	const seeded = await seedPortalClient(request, 'Riverside Doulas');
	const { practiceId } = seeded;
	const contractor = await seedContractorDoula(request, practiceId, seeded.staffHeaders);

	await page.goto('/login');
	await page.getByLabel('Email address').fill(contractor.email);
	await page.getByLabel('Password').fill(PORTAL_CLIENT_PASSWORD);
	await page.getByRole('button', { name: 'Sign in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// #539: a freshly seeded contractor has no attached Clients yet, so
	// this is also the only place the empty-state "How to add Clients of
	// your own" line onto #501's door gets an axe pass -- the Owner's own
	// Clients scan above always has rows.
	await scan(page, {
		key: 'practices/[practiceId]/clients (contractor, empty)',
		archetype: 'C',
		url: `/practices/${practiceId}/clients`,
		h1: 'Clients'
	});

	await scan(page, {
		key: 'practices/[practiceId]/clients/search (contractor)',
		archetype: 'E',
		url: `/practices/${practiceId}/clients/search`,
		h1: 'Add a Client'
	});
});

// Archetypes D and G, behind a Client-portal session. The Birth Plan and
// the Contract are each scanned twice: first empty, before the API
// provisions them, and then loaded. The empty state is a screen of its own
// (#1574): it sits in DocumentPage's frame with its own <h1>, and that is
// what a Client meets for as long as her Practice has not made one. The
// Invoice page has no empty state -- a missing Invoice is a load error --
// so it is scanned loaded only.
//
// One Client is seeded and signed in once, in the outer `beforeAll`, and
// each row is its own test (the budget comment above `enterSession`). The
// order the two states need is the order Playwright runs a file's tests
// in: the empty rows come first, and the nested describe's `beforeAll`,
// which provisions the documents, runs only once they have.
interface PortalSeed {
	engagementId: string;
	// The Invoice on the Stripe rail, the one the Invoice page shows.
	invoiceId: string;
}

const PORTAL_EMPTY_ROUTES: SeededRoute<PortalSeed>[] = [
	{
		key: 'portal/engagements/[engagementId]/birth-plan (empty)',
		archetype: 'G',
		url: (s) => `/portal/engagements/${s.engagementId}/birth-plan`,
		h1: 'Birth plan',
		reach: (p) => expect(p.getByText('No birth plan has been created for your care yet.')).toBeVisible()
	},
	{
		key: 'portal/engagements/[engagementId]/contract (empty)',
		archetype: 'G',
		url: (s) => `/portal/engagements/${s.engagementId}/contract`,
		h1: 'Contract',
		reach: (p) => expect(p.getByText('No contract has been sent for your care yet.')).toBeVisible()
	}
];

const PORTAL_ROUTES: SeededRoute<PortalSeed>[] = [
	{
		key: 'portal/engagements/[engagementId]',
		archetype: 'D',
		url: (s) => `/portal/engagements/${s.engagementId}`,
		h1: 'Your care'
	},
	{
		key: 'portal/engagements/[engagementId]/birth-plan',
		archetype: 'G',
		url: (s) => `/portal/engagements/${s.engagementId}/birth-plan`,
		h1: 'Birth plan'
	},
	{
		key: 'portal/engagements/[engagementId]/contract',
		archetype: 'G',
		url: (s) => `/portal/engagements/${s.engagementId}/contract`,
		h1: 'Contract'
	},
	// Also younger than the archetype survey: Messages joined the
	// portal nav on #431, and it is a RecordDetail like the hub above.
	{
		key: 'portal/engagements/[engagementId]/messages',
		archetype: 'D',
		url: (s) => `/portal/engagements/${s.engagementId}/messages`,
		h1: 'Messages'
	},
	// #303: the notification-preference settings screen, a FormPage
	// (archetype G) like Birth Plan and Contract above.
	{
		key: 'portal/engagements/[engagementId]/notifications',
		archetype: 'G',
		url: (s) => `/portal/engagements/${s.engagementId}/notifications`,
		h1: 'Notifications'
	},
	// #1810: the address she signs in with (#619), a summary since #2045,
	// and its Change, a question page.
	{
		key: 'portal/engagements/[engagementId]/sign-in-address',
		archetype: 'D',
		url: (s) => `/portal/engagements/${s.engagementId}/sign-in-address`,
		h1: 'Sign-in address'
	},
	{
		key: 'portal/engagements/[engagementId]/sign-in-address/change',
		archetype: 'E',
		url: (s) => `/portal/engagements/${s.engagementId}/sign-in-address/change`,
		h1: 'What is your new sign-in address?'
	},
	// #1564: the Client's money -- an index of Invoices (archetype D,
	// like the hub) and a page per Invoice (archetype G, like the
	// Contract it bills).
	{
		key: 'portal/engagements/[engagementId]/invoices',
		archetype: 'D',
		url: (s) => `/portal/engagements/${s.engagementId}/invoices`,
		h1: 'Invoices'
	},
	{
		key: 'portal/engagements/[engagementId]/invoices/[invoiceId]',
		archetype: 'G',
		url: (s) => `/portal/engagements/${s.engagementId}/invoices/${s.invoiceId}`,
		h1: 'Invoice A4B2-0011'
	}
];

test.describe('Archetypes D, G -- the Client portal', () => {
	let seeded: SeededPortalClient;
	let portalSession: Cookie;
	const seed: PortalSeed = { engagementId: '', invoiceId: '' };

	test.beforeAll(async ({ browser, request }) => {
		seeded = await seedPortalClient(request, 'Riverside Doulas');
		seed.engagementId = seeded.engagementId;

		// The one real walk of the magic link, in a browser of its own, so
		// each test can start from the session it leaves behind.
		const signInContext = await browser.newContext({
			baseURL: PREVIEW_SERVER_ORIGIN
		});
		const signInPage = await signInContext.newPage();
		await signInPortalClient(signInPage, request, seeded.clientEmail);
		await expect(signInPage).toHaveURL(new RegExp(`/portal/engagements/${seeded.engagementId}$`));
		const cookies = await signInContext.cookies();
		const cookie = cookies.find(({ name }) => name === '__session');
		expect(cookie, 'the magic link left no __session cookie').toBeTruthy();
		portalSession = cookie!;
		await signInContext.close();
	});

	for (const row of PORTAL_EMPTY_ROUTES) {
		test(row.key, async ({ page, context }) => {
			await context.addCookies([portalSession]);
			await scan(page, routeFor(row, seed));
		});
	}

	test.describe('with the Birth Plan, the Contract and two Invoices', () => {
		test.beforeAll(async ({ request }) => {
			seed.invoiceId = await provisionPortalDocuments(request, seeded);
		});

		for (const row of PORTAL_ROUTES) {
			test(row.key, async ({ page, context }) => {
				await context.addCookies([portalSession]);
				await scan(page, routeFor(row, seed));
			});
		}
	});
});

/**
 * Makes her Practice's Birth Plan, sends her Contract, and raises two
 * Invoices on it. Returns the open Invoice's id.
 */
async function provisionPortalDocuments(request: APIRequestContext, seeded: SeededPortalClient): Promise<string> {
	const { practiceId, engagementId, staffHeaders } = seeded;
	const engagementURL = `${API_URL}/api/practices/${practiceId}/engagements/${engagementId}`;

	const plan = await request.post(`${engagementURL}/plans/birth_plan`, {
		headers: staffHeaders
	});
	expect(plan.ok(), `create birth plan failed: ${plan.status()} ${await plan.text()}`).toBe(true);

	const contract = await request.post(`${engagementURL}/contract`, {
		headers: staffHeaders
	});
	expect(
		contract.ok(),
		`create contract failed: ${contract.status()} ${await contract.text()}`
	).toBe(true);
	// #258: Send now refuses a Contract with any blank merge field.
	// practice_name and client_name are already resolved by creation;
	// #967 resolves price the same way, from the rate card seedPortalClient
	// seeds (portalClient.ts) -- PutContractHandler refuses a request that
	// tries to set it directly, so it's excluded here even though the POST
	// response above already carries it resolved. The default seeded
	// template's other two merge fields (scope_of_service, the two
	// engagement dates) have no such column backing them, so they're
	// filled in by hand before Send.
	const { values } = await contract.json();
	// eslint-disable-next-line @typescript-eslint/no-unused-vars -- destructured only to exclude it from editableValues below
	const { price: _price, ...editableValues } = values;
	const filled = await request.put(`${engagementURL}/contract`, {
		headers: staffHeaders,
		data: {
			values: {
				...editableValues,
				scope_of_service: '12 prenatal visits',
				engagement_start_date: '2027-01-01',
				engagement_end_date: '2027-06-01'
			}
		}
	});
	expect(filled.ok(), `fill contract values failed: ${filled.status()} ${await filled.text()}`).toBe(true);
	const sent = await request.post(`${engagementURL}/contract/send`, {
		headers: staffHeaders
	});
	expect(sent.ok(), `send contract failed: ${sent.status()} ${await sent.text()}`).toBe(true);

	// #1564: the money screens read nothing until an Invoice exists.
	const invoiceId = seedInvoice(practiceId, engagementId, {
		reference: 'A4B2-0011',
		// Unique per seed: the column is UNIQUE, so a fixed id fails every
		// repeat after the first (`--repeat-each`).
		stripeInvoiceId: `in_a11y_${randomUUID()}`
	});
	seedInvoice(practiceId, engagementId, {
		status: 'paid',
		reference: 'INV-0002'
	});
	return invoiceId;
}

// The guard on the gate itself (#1152), and the one test here that is not a
// scan: it holds a screen in its loading state on purpose and asserts that
// `scan` REFUSES it.
//
// Why this exists rather than trusting the wait. A ready signal that stops
// being one fails silently -- the sweep goes green having measured a
// placeholder, which is exactly what the h1-only signal did on every
// FormPage and ListPage row until this ticket. Nothing about a green run
// can tell those two apart, so the refusal is asserted directly.
//
// `settings/mfa` is the subject because its loading state is reached by
// holding ONE request. `loading` there is derived from its Owner-only
// impact read having not answered yet, and that read is this page's own --
// not the authenticated layout's session read, which every screen shares
// and whose 401 handling is a navigation (see mountSettled.ts). The route
// handler below never fulfills, continues or aborts, so the request stays
// in flight for the life of the test and the Skeleton never leaves: the
// screen is held, rather than raced.
//
// It costs its own session rather than joining the Staff loop above. A
// deliberately stalled route must never share a page with a real scan --
// the interception would outlive the row that wanted it and quietly gate
// another one. The session is seedPortalClient's, with no second factor:
// the loading and loadError branches are drawn before the page reads it,
// so either Owner gives the same screen here.
test('the sweep refuses a screen whose loading affordance is still up', async ({
	page,
	request,
	context
}) => {
	const seeded = await seedPortalClient(request, 'Riverside Doulas');
	const { practiceId } = seeded;
	await enterPractice(context, page, seeded.staffHeaders, practiceId);

	await page.route('**/mfa-required/impact', () => {
		// Deliberately unanswered. Fulfilling, continuing or aborting here
		// would each let the screen finish one way or another.
	});

	const held: Route = {
		key: 'practices/[practiceId]/settings/mfa (held loading)',
		// Not a row in the inventory above, and the label says so: this
		// fixture exists to be refused, and a failure message reading a
		// bare 'F' would imply a scanned route that does not exist.
		archetype: 'F, as a guard fixture rather than an inventory row',
		url: `/practices/${practiceId}/settings/mfa`,
		h1: 'Multi-factor authentication'
	};

	// The h1 signal is satisfied here -- FormPage renders the title in its
	// loading branch -- so a `scan` that stopped at the h1 would have gone
	// on to run axe against the Skeleton and passed. It is the
	// loading-affordance wait that rejects, and it names what it saw.
	await expect(scan(page, held)).rejects.toThrow(/still had a loading affordance on screen/);
});

// The guard on LOAD_ERROR_SELECTOR (#1258), the same shape as the loading
// guard above: it holds a screen in its `loadError` branch on purpose and
// asserts that `scan` REFUSES it, rather than trusting the wait to keep
// working. Same subject and same endpoint as the loading guard, so
// `settings/mfa`'s own reasoning above for why that endpoint is this
// page's own read, not the shared layout session read, applies here too --
// only the route handler's answer differs: a refusal this time, not a
// hung request, so `loadImpact` catches and sets `loadError` rather than
// waiting forever.
test('the sweep refuses a screen whose Template is in its loadError branch', async ({
	page,
	request,
	context
}) => {
	const seeded = await seedPortalClient(request, 'Riverside Doulas');
	const { practiceId } = seeded;
	await enterPractice(context, page, seeded.staffHeaders, practiceId);

	await page.route('**/mfa-required/impact', (route) =>
		route.fulfill({ status: 500, contentType: 'application/json', body: '{}' })
	);

	const held: Route = {
		key: 'practices/[practiceId]/settings/mfa (held loadError)',
		// Not a row in the inventory above, and the label says so, matching
		// the loading guard's own fixture above.
		archetype: 'F, as a guard fixture rather than an inventory row',
		url: `/practices/${practiceId}/settings/mfa`,
		h1: 'Multi-factor authentication'
	};

	// Both the h1 and loading-affordance waits are satisfied here -- FormPage
	// renders the title and no Skeleton in its loadError branch -- so a
	// `scan` that stopped at either would have gone on to run axe against
	// the one-Notice error screen and passed. It is the load-error wait
	// that rejects, and it names what it saw.
	await expect(scan(page, held)).rejects.toThrow(/failed to load/);
});
