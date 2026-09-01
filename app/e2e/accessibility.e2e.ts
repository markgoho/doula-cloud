import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT } from './ports';
import {
	seedClient,
	seedContractorDoula,
	seedPortalClient,
	PORTAL_CLIENT_PASSWORD
} from './portalClient';
import { seedEngagement, seedEngagementRequest } from './stack';

const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

/**
 * The automated half of the accessibility gate (#447). Everything about
 * why this exists, what it owns, and what it deliberately cannot see is
 * in docs/testing.md -- read that before adding an assertion here.
 */

// WCAG 2.2 AA, which is the bar GDS holds its own services to, and this
// repo has already adopted the GOV.UK Design System as its reference for
// service patterns (ADR-0021) -- so the level is pre-argued rather than
// picked here. axe's `best-practice` tag is deliberately off: those are
// opinions, not conformance failures, and a gate that blocks on an
// opinion is a gate people learn to route around.
const WCAG_TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

/*
 * Not scanned, and each is a decision rather than an oversight:
 *
 * - `style-guide/*` -- sixty-odd component demo pages. They are not
 *   archetype routes, and scanning them would roughly triple the run for
 *   no additional archetype coverage. Every component on them is already
 *   scanned in the place a person meets it.
 * - `demo/*` -- SvelteKit's own scaffolding, not a screen in the product.
 * - `/` and `/account` -- both sit outside every route group that carries
 *   chrome, so they render with no shell, no `<main>` and no skip link at
 *   all. Filed as #484; scanning them here would only re-find it.
 */

/**
 * One route to scan. `key` is the stable name a KNOWN entry points at --
 * the route pattern, not the provisioned URL, so an allowance survives a
 * new fixture. `h1` is the ready signal: this is a client-rendered SPA,
 * so `goto` resolves long before the data lands, and axe run against a
 * half-loaded page finds a different set of violations every time --
 * which CI's `retries: 2` would then quietly launder into green.
 */
interface Route {
	key: string;
	archetype: string;
	url: string;
	h1: string | RegExp;
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

async function scan(page: Page, route: Route) {
	await page.goto(route.url);
	await expect(
		page.getByRole('heading', { level: 1, name: route.h1 }),
		`${route.key} never finished loading -- axe would have scanned a skeleton`
	).toBeVisible();

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

// Archetype A, and the only batch that needs no fixtures at all: every
// one of these is what a person meets before there is an account. The
// two token-bearing screens are scanned in their no-token entry state,
// which is the state a person actually arrives in when the link is
// stale -- the form is the page.
test('Archetype A -- the screens a person meets signed out', async ({ page }) => {
	const routes: Route[] = [
		{ key: 'login', archetype: 'A', url: '/login', h1: 'Log in' },
		{ key: 'signup', archetype: 'A', url: '/signup', h1: 'Sign up your Practice' },
		{
			key: 'accept-invite',
			archetype: 'A',
			url: '/accept-invite',
			h1: 'Accept your Staff invite'
		},
		{
			key: 'offers/[offerId]',
			archetype: 'A',
			url: '/offers/00000000-0000-0000-0000-000000000000',
			h1: 'An offer of work'
		},
		{ key: 'portal/login', archetype: 'A', url: '/portal/login', h1: 'Log in' }
	];

	for (const route of routes) {
		await scan(page, route);
	}
});

// Archetypes B, C, D, E and F, all behind one Staff session. Provisioned
// once and looped rather than a test per route: signup + login is ~4s of
// the same work every time, and none of these scans depends on any other
// having run.
test('Archetypes B, C, D, E, F -- the Staff side', async ({ page, request }) => {
	const seeded = await seedPortalClient(request, 'Riverside Doulas');
	const { practiceId, staffId, engagementId } = seeded;

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
	const pendingRequestClientId = await seedClient(request, practiceId, seeded.staffHeaders, {
		givenName: 'Casey',
		familyName: 'Pending'
	});
	seedEngagementRequest(pendingRequestClientId, practiceId, staffId);

	await page.goto('/login');
	await page.getByLabel('Email').fill(seeded.staffEmail);
	await page.getByLabel('Password').fill(PORTAL_CLIENT_PASSWORD);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	const routes: Route[] = [
		{
			key: 'practices/[practiceId]',
			archetype: 'B',
			url: `/practices/${practiceId}`,
			h1: 'Welcome to Riverside Doulas'
		},
		{
			key: 'practices/[practiceId]/clients',
			archetype: 'C',
			url: `/practices/${practiceId}/clients`,
			h1: 'Clients'
		},
		{
			key: 'practices/[practiceId]/invoices',
			archetype: 'C',
			url: `/practices/${practiceId}/invoices`,
			h1: 'Invoices'
		},
		{
			key: 'practices/[practiceId]/billing',
			archetype: 'C',
			url: `/practices/${practiceId}/billing`,
			h1: 'Billing'
		},
		{
			key: 'practices/[practiceId]/staff',
			archetype: 'C',
			url: `/practices/${practiceId}/staff`,
			h1: 'Staff'
		},
		{
			key: 'practices/[practiceId]/offers',
			archetype: 'C',
			url: `/practices/${practiceId}/offers`,
			h1: 'Your offers'
		},
		{
			key: 'practices/[practiceId]/engagement-requests',
			archetype: 'C',
			url: `/practices/${practiceId}/engagement-requests`,
			h1: 'Requests awaiting approval'
		},
		{
			key: 'practices/[practiceId]/engagements/[engagementId]',
			archetype: 'D',
			url: `/practices/${practiceId}/engagements/${engagementId}`,
			h1: 'Pat'
		},
		{
			key: 'practices/[practiceId]/clients/[clientId]',
			archetype: 'D',
			url: `/practices/${practiceId}/clients/${clientId}`,
			h1: 'Jane Smith'
		},
		{
			// #535: the same hub, scanned a second time for a Client whose
			// pending Engagement Request was requested by this signed-in
			// session -- the only state that puts the Withdraw Button and
			// its surrounding pending-Request Notice in the DOM.
			key: 'practices/[practiceId]/clients/[clientId] (pending request)',
			archetype: 'D',
			url: `/practices/${practiceId}/clients/${pendingRequestClientId}`,
			h1: 'Casey Pending'
		},
		{
			key: 'practices/[practiceId]/clients/[clientId]/edit',
			archetype: 'E',
			url: `/practices/${practiceId}/clients/${clientId}/edit`,
			h1: 'Edit Jane Smith'
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
			url: `/practices/${practiceId}/clients/${clientId}/engagement-requests/new`,
			h1: /^Start work with Jane Smith$/
		},
		{
			key: 'practices/[practiceId]/clients/search',
			archetype: 'E',
			url: `/practices/${practiceId}/clients/search`,
			h1: 'Find a Client'
		},
		{
			key: 'practices/[practiceId]/clients/new',
			archetype: 'E',
			url: `/practices/${practiceId}/clients/new`,
			h1: "What is the Client's name?"
		},
		{
			key: 'practices/[practiceId]/invite',
			archetype: 'E',
			url: `/practices/${practiceId}/invite`,
			h1: 'Invite a Staff member'
		},
		{
			key: 'practices/[practiceId]/settings',
			archetype: 'F',
			url: `/practices/${practiceId}/settings`,
			h1: 'Settings'
		},
		{
			key: 'practices/[practiceId]/settings/payments',
			archetype: 'F',
			url: `/practices/${practiceId}/settings/payments`,
			h1: 'Payments'
		},
		{
			key: 'practices/[practiceId]/settings/contract-template',
			archetype: 'F',
			url: `/practices/${practiceId}/settings/contract-template`,
			h1: 'Contract Template'
		},
		{
			key: 'practices/[practiceId]/settings/plan-templates',
			archetype: 'F',
			url: `/practices/${practiceId}/settings/plan-templates`,
			h1: 'Plan Templates'
		},
		// The last two settings screens postdate #405's A-G survey -- they
		// arrived with #452's Settings hub -- so they are archetype F by
		// shape rather than by that table.
		{
			key: 'practices/[practiceId]/settings/website',
			archetype: 'F',
			url: `/practices/${practiceId}/settings/website`,
			h1: 'Your website'
		},
		{
			key: 'practices/[practiceId]/settings/client-fields',
			archetype: 'F',
			url: `/practices/${practiceId}/settings/client-fields`,
			h1: 'Client Fields'
		}
	];

	for (const route of routes) {
		await scan(page, route);
	}
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
	await page.getByLabel('Email').fill(contractor.email);
	await page.getByLabel('Password').fill(PORTAL_CLIENT_PASSWORD);
	await page.getByRole('button', { name: 'Log in' }).click();
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

// Archetypes D and G, behind a Client-portal session. Both G routes
// render nothing but a "none yet" line until the record exists, so the
// Birth Plan and the Contract are provisioned through the API first --
// scanning the empty state would prove nothing about the document view,
// which is the whole point of archetype G.
test('Archetypes D, G -- the Client portal', async ({ page, request }) => {
	const seeded = await seedPortalClient(request, 'Riverside Doulas');
	const { practiceId, engagementId, staffHeaders } = seeded;
	const engagementURL = `${API_URL}/api/practices/${practiceId}/engagements/${engagementId}`;

	const plan = await request.post(`${engagementURL}/plans/birth_plan`, { headers: staffHeaders });
	expect(plan.ok(), `create birth plan failed: ${plan.status()} ${await plan.text()}`).toBe(true);

	const contract = await request.post(`${engagementURL}/contract`, { headers: staffHeaders });
	expect(
		contract.ok(),
		`create contract failed: ${contract.status()} ${await contract.text()}`
	).toBe(true);
	const sent = await request.post(`${engagementURL}/contract/send`, { headers: staffHeaders });
	expect(sent.ok(), `send contract failed: ${sent.status()} ${await sent.text()}`).toBe(true);

	await page.goto('/portal/login');
	await page.getByLabel('Email').fill(seeded.clientEmail);
	await page.getByLabel('Password').fill(PORTAL_CLIENT_PASSWORD);
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	const routes: Route[] = [
		{
			key: 'portal/engagements/[engagementId]',
			archetype: 'D',
			url: `/portal/engagements/${engagementId}`,
			h1: 'Welcome to Riverside Doulas'
		},
		{
			key: 'portal/engagements/[engagementId]/birth-plan',
			archetype: 'G',
			url: `/portal/engagements/${engagementId}/birth-plan`,
			h1: 'Birth Plan'
		},
		{
			key: 'portal/engagements/[engagementId]/contract',
			archetype: 'G',
			url: `/portal/engagements/${engagementId}/contract`,
			h1: 'Contract'
		},
		// Also younger than the archetype survey: Messages joined the
		// portal nav on #431, and it is a RecordDetail like the hub above.
		{
			key: 'portal/engagements/[engagementId]/messages',
			archetype: 'D',
			url: `/portal/engagements/${engagementId}/messages`,
			h1: 'Messages'
		}
	];

	for (const route of routes) {
		await scan(page, route);
	}
});
