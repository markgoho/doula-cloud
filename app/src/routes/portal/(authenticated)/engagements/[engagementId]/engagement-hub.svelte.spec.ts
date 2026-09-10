import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import Hub from './+page.svelte';
// Rendering `+page.svelte` directly bypasses `+layout.svelte`, the only
// place the real app calls this -- without it a layout primitive sits
// unregistered and inert (clients-list.svelte.spec.ts's own reason for the
// same line). #486's own DataTable assertions below need the real
// table-view/record-view switch.
import '#lib/styles/app.css';
import { toApiResponder, toPageState } from '../../../../routeFixture.js';
import { createdAt, detail, fixture, practiceName, visits } from './page.fixture.js';
import { engagementLabel } from '#lib/clientRegister.js';
if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The Engagement this screen shows, and the `page` it reads, both come
 * from the route's own fixture (#596) -- so the screen this spec asserts
 * on and the screen the continuum sweep measures are one description.
 * `vi.mock` is hoisted above every import, so `pageState` is declared
 * empty and filled from the fixture once the imports have run; the hub
 * reads `page.params.engagementId` for the fetch and its two Link hrefs,
 * and `page.data.practiceName` for the bar, inside its own functions
 * rather than at module scope, so the later write is seen. Same
 * installation, through the same `toPageState`, as
 * `route-continuum.svelte.spec.ts`.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
}));

function jsonResponse(body: unknown) {
	return { ok: true, json: () => Promise.resolve(body) } as Response;
}

// The Engagement detail read, #486's own /activity read and #478's
// /visits read share this one mock, branched by path -- a blanket single
// response would hand either list loader the detail body instead of a
// { items, hasMore } page.
function mockFetch(body: unknown, activityItems: unknown[] = [], visitItems: unknown[] = visits) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.includes('/activity')) return Promise.resolve(jsonResponse({ items: activityItems, hasMore: false }));
		if (path.includes('/visits')) return Promise.resolve(jsonResponse({ items: visitItems, hasMore: false }));
		return Promise.resolve(jsonResponse(body));
	});
}

/** One of the two list reads refusing rather than answering, which no
 * list content can express: the named read fails, the other list answers
 * empty, and the Engagement detail answers as ever. Both the Activity
 * block and the Your visits block need this and neither can express it
 * through `mockFetch`, so it is written once and told which read fails. */
function mockRefusedList(failingPath: '/activity' | '/visits', refusal: string) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.includes(failingPath))
			return Promise.resolve({ ok: false, text: () => Promise.resolve(refusal) } as Response);
		if (path.includes('/activity') || path.includes('/visits'))
			return Promise.resolve(jsonResponse({ items: [], hasMore: false }));
		return Promise.resolve(jsonResponse(detail));
	});
}

/*
 * One `setup()` per `describe` below, per `.claude/rules/svelte-tests.md`
 * -- each named for the block it serves rather than all four named
 * `setup`, because they live out here rather than inside their blocks.
 * `unicorn/consistent-function-scoping` refuses a nested function that
 * closes over nothing its enclosing scope owns, and none of these does:
 * every one of them reaches only for this file's own mocks, the fixture,
 * and the route.
 */

/** The `Client portal Engagement hub` block's setup. The happy path is
 * the fixture answering every read itself, which is also what the
 * continuum sweep installs -- so `record` is left out for a test that
 * asserts on the fixture's own Engagement. A test that needs a state the
 * fixture does not hold passes a spread of `detail` here, which routes
 * through `mockFetch` instead: the fixture's `respond` answers from its
 * own content and cannot be handed a departure from it. Nothing in that
 * block reads the render result. */
async function setupHub({ record }: { record?: unknown } = {}) {
	if (record === undefined) {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
	} else {
		mockFetch(record);
	}
	await render(Hub);
}

/** The `#296` heading block's setup. The fixture's own Engagement is the
 * happy path; a state that ticket has to hold for is a spread of it,
 * never a second record. Nothing in that block reads the render result:
 * the heading is asserted through the role tree. */
async function setupHeading({ record = detail }: { record?: unknown } = {}) {
	mockFetch(record);
	await render(Hub);
}

describe('Client portal Engagement hub', () => {
	// The happy path: the fixture's own detail already carries a due date
	// (2027-03-01), so this is the fixture's content unmodified.
	it("shows the due date under its own label, not 'Created' (#505)", async () => {
		await setupHub();

		await expect.element(page.getByText('Due date')).toBeVisible();
		await expect.element(page.getByText('Mar 1, 2027')).toBeVisible();
		await expect.element(page.getByText('Created')).not.toBeInTheDocument();
	});

	// #310's own "a change of Engagement is announced" AC: SvelteKit's
	// built-in navigation announcer reads `document.title` aloud after a
	// client-side route change, so the title has to carry the same
	// distinguishing fact the chrome's switcher does -- two Engagements at
	// the same Practice must not announce the same words. `serviceName`
	// builds `engagementLabel` from the ancestor layout's `practiceName`
	// and `createdAt` rather than the bare Practice name.
	it('titles the tab with the same distinguishing label the switcher uses (#310)', async () => {
		await setupHub();

		await expect.element(page.getByText('Due date')).toBeVisible();
		expect(document.title).toContain(engagementLabel({ practiceName, createdAt }));
	});

	// ADR-0017: a postpartum-only Engagement has no due date. #505 asks for
	// nothing to show, not a blank row and not a placeholder -- the row is
	// left out of the DescriptionList's own items entirely, so there is no
	// "Due date" label sitting over an empty value. Not the happy path --
	// the fixture's own detail has a due date (see the previous test) --
	// so this is a departure from it, spread rather than restated.
	it('shows nothing for a null due date -- no blank label, no placeholder', async () => {
		await setupHub({ record: { ...detail, dueDate: undefined } });

		// #212: the register's own label ("Ongoing" for `active`), not the
		// raw enum value -- this is also this suite's own assertion for
		// #212's AC4 (the portal shows the register's word for a status
		// value).
		await expect.element(page.getByText('Ongoing')).toBeVisible();
		await expect.element(page.getByText(/due date/i)).not.toBeInTheDocument();
	});

	// #311: a postpartum-only Engagement offers no Birth Plan link on the
	// hub, alongside the layout's own nav-item test. Not the happy path --
	// the fixture's own detail offers one (see the first test above) -- so
	// this is a departure from it, spread rather than restated.
	it('offers no Birth plan link when the Engagement does not call for one', async () => {
		await setupHub({ record: { ...detail, offersBirthPlan: false } });

		await expect.element(page.getByText('Ongoing')).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Birth plan' })).not.toBeInTheDocument();
		await expect.element(page.getByRole('link', { name: 'Contract' })).toBeVisible();
	});
});

/*
 * #296. The hub used to head itself `Welcome to {practiceName}` -- a
 * first-visit greeting rendered on every visit for the life of the
 * Engagement, which on the loss journey is the first thing the screen
 * says to a woman coming back three weeks after her pregnancy ended.
 *
 * It is now CONTEXT.md's own Client word for an Engagement: the entry's
 * `_Client says_:` line reads `my care ("Your care" as a heading)`.
 *
 * There is deliberately no first-visit-versus-returning test here,
 * because there is deliberately no such branch to test: the heading is a
 * literal in `+page.svelte` and consults nothing -- not the record, not a
 * visit count, not an outcome. What these tests prove instead is that
 * claim from the outside -- the same words across every Engagement state
 * the DTO can be in, including the shape a Client comes back to after a
 * loss -- so a conditional reintroduced later fails here rather than
 * passing quietly.
 */
describe("the hub's heading (#296)", () => {
	it("names the page with the register's own word, and greets nobody", async () => {
		await setupHeading();

		await expect.element(page.getByRole('heading', { name: 'Your care', level: 1 })).toBeVisible();
		// Still the page's only <h1>, so the document outline and where a
		// screen reader lands are unchanged by the rewording. A heading is
		// announced, so `level: 1` says this through the role tree rather
		// than reaching past it into the DOM for a tag name.
		expect(await page.getByRole('heading', { level: 1 }).all()).toHaveLength(1);
		await expect.element(page.getByText(/welcome/i)).not.toBeInTheDocument();
		// The Practice's name left the heading and not the product: it is
		// still what `<title>` is built from, which is also what SvelteKit's
		// navigation announcer reads aloud. The other half of that -- the
		// name a Client *sees* -- belongs to the portal shell's own top bar
		// and is asserted in `portal-authenticated-layout.svelte.spec.ts`,
		// since rendering `+page.svelte` alone has no shell around it.
		expect(document.title).toContain(practiceName);
	});

	// The three values CONTEXT.md's Engagement entry defines, plus the
	// record a Client comes back to after a loss. That last one is a
	// `completed` Engagement with no due date left to speak of and a Birth
	// Plan she still owns -- shaped from the DTO alone, because this ticket
	// must not read `birth_outcome` (#293 adds the column and #294 owns the
	// surface that reads it), and the point here is precisely that the
	// heading reads no such fact.
	const records = [
		{ name: 'an Engagement in intake', detail: { ...detail, status: 'intake' } },
		{ name: 'an active Engagement', detail: { ...detail, status: 'active' } },
		{ name: 'a completed Engagement', detail: { ...detail, status: 'completed' } },
		{
			name: 'a Client returning after a loss',
			detail: { ...detail, status: 'completed', dueDate: undefined }
		}
	];

	it.each(records)('says the same words to $name', async ({ detail: record }) => {
		await setupHeading({ record });

		await expect.element(page.getByRole('heading', { name: 'Your care', level: 1 })).toBeVisible();
		await expect.element(page.getByText(/welcome/i)).not.toBeInTheDocument();
	});
});

// #486 AC5: CONTEXT.md's own vocabulary for this to a Client -- "Everything
// that has happened" -- behind a closed disclosure, per the design brief's
// own #433 amendment for the Client portal.
/** One ledger entry, named by the action whose wording the test is about
 * -- the only field any of the Activity tests varies. The actor is a
 * generic name, not a person's: portal.ActivityHandler (Go) already
 * replaces a staff actor's name before this response ever reaches the
 * browser (CONTEXT.md's Activity entry: "never who inside the Practice
 * did what"). This spec only proves the frontend renders whatever name it
 * is given, muted; the redaction itself has its own Go test. */
function activityEntry(action: string) {
	return {
		subjectKind: 'engagement',
		subjectId: detail.engagementId,
		action,
		actorKind: 'staff',
		actorName: 'Your practice',
		createdAt: new Date().toISOString()
	};
}

/** The `#486` Activity block's setup. The happy path is the fixture
 * answering every read itself, the same installation the continuum sweep
 * makes -- so a test asserting on the fixture's own ledger passes
 * nothing. `activity` is the ledger this screen is handed instead, and
 * `refusal` is the ledger read failing rather than answering, which no
 * list content can express. `container` is returned because a closed
 * `<details>` is outside the accessibility tree, so those tests have to
 * reach past it -- the reason each of them restates at its own scoping
 * line. */
async function setupActivity({ activity, refusal }: { activity?: unknown[]; refusal?: string } = {}) {
	if (refusal !== undefined) {
		mockRefusedList('/activity', refusal);
	} else if (activity === undefined) {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
	} else {
		mockFetch(detail, activity);
	}
	const { container } = await render(Hub);
	return { container };
}

describe('the Activity disclosure (#486)', () => {
	// DataTable renders both a <table> and a record view at once (#508,
	// ADR-0024), so a bare page-wide getByText matches both -- scoped here
	// to `.table-view` the same way DataTable.svelte.spec.ts's own record-
	// view tests do, rather than asserting on ambiguous duplicate text.
	// `.frame` has no accessible signal of its own for open/closed --
	// svelte-tests.md's rule 1, the same exception DataTable's own
	// disclosure test already relies on: `<details>` without `open`
	// removes its content from the accessibility tree entirely rather than
	// merely marking it not-visible, so `getByRole`/`getByText` can never
	// resolve anything inside it to assert against.
	it('renders closed under its own heading, with a descriptive toggle', async () => {
		await page.viewport(1440, 900);

		const { container } = await setupActivity({ activity: [activityEntry('contract_sent')] });

		await expect
			.element(page.getByRole('heading', { name: 'Everything that has happened' }))
			.toBeVisible();
		await expect.element(page.getByText('Show what has happened')).toBeVisible();
		expect(getComputedStyle(container.querySelector(':scope details .frame')!).display).toBe('none');

		await page.getByText('Show what has happened').click();
		expect(getComputedStyle(container.querySelector(':scope details .frame')!).display).not.toBe('none');
		// Scoped inside the disclosure specifically: #478 put a second
		// DataTable ("Your visits") on this page, above this one, so a bare
		// `.table-view` is that table rather than the ledger's.
		const tableView = page.elementLocator(container.querySelector(':scope details .table-view')!);
		// #708: the register's phrase for `contract_sent`, not the staff
		// surfaces' "Contract sent" -- which is the raw action string with
		// its underscore taken out, and a domain word ADR-0005 says a
		// Client never meets.
		await expect.element(tableView.getByText('Your Contract was sent to you.')).toBeVisible();
		await expect.element(tableView.getByText('Your practice')).toBeVisible();
	});

	// This describe once carried an overflow sweep of its own, opening the
	// disclosure by hand and then reusing `continuum.ts`'s instrument on
	// it. #486 needed it because a closed `<details>` takes no box at all,
	// so `route-continuum.svelte.spec.ts` measured a ledger it could not
	// see into -- and #710 fixed that where it belonged, in the sweep,
	// which now opens every closed disclosure under the frame before it
	// measures and closes it again after. A second copy of the instrument
	// beside one route is exactly what #570 says not to keep.
	//
	// What the copy is replaced by is the test below, not nothing. The
	// shared sweep waits on one signal -- the fixture's `readyText`, this
	// screen's `<h1>` -- and this route answers three requests, of which
	// the ledger's is neither the first nor the one the heading depends
	// on. So the thing that has to hold for #486's own AC to still be
	// checked is that the rows are already rendered when that signal is
	// met. Measured, and now asserted, rather than assumed: the deleted
	// test polled for the row text itself and so carried this guarantee
	// inside it, and dropping the test without keeping the guarantee is
	// how a sweep goes on passing over an empty table.
	it("has the ledger's rows rendered by the time the sweep's own signal is met", async () => {
		const { container } = await setupActivity();
		await expect
			.element(page.getByRole('heading', { name: fixture.readyText, level: 1 }))
			.toBeVisible();

		// Read synchronously off the DOM, with no poll of its own: a poll
		// here would wait for the rows the sweep does not wait for, and
		// would pass in exactly the case this exists to fail. Scoped
		// through `querySelector` for the reason every other Activity
		// test in this file is (`.claude/rules/svelte-tests.md` case 1):
		// a closed `<details>` puts its content outside the accessibility
		// tree entirely, so no accessible query can reach in.
		expect(container.querySelector(':scope details')?.textContent).toContain(
			'A payment recorded earlier was removed.'
		);
	});

	// #708's own acceptance criterion, on the row the ticket names: the
	// staff surfaces render `plan_instance_edited` as "Plan instance
	// edited", which puts an internal system noun in front of a Client.
	// Asserted through the opened disclosure's own table view, the same
	// scoping the two tests above take and for the same #508 reason.
	it("speaks the Client register, not the write side's action name", async () => {
		const { container } = await setupActivity({ activity: [activityEntry('plan_instance_edited')] });

		await page.getByText('Show what has happened').click();
		const tableView = page.elementLocator(container.querySelector(':scope details .table-view')!);
		await expect.element(tableView.getByText('Your Birth Plan was updated.')).toBeVisible();
		expect(container.querySelector(':scope details')!.textContent).not.toContain('Plan instance edited');
	});

	it('says so when the ledger cannot be read', async () => {
		await setupActivity({ refusal: 'nope' });

		await expect.element(page.getByText('nope')).toBeVisible();
	});
});

// DataTable renders both a <table> and a record view at once (#508,
// ADR-0024) with the same text in each, so a page-wide accessible query
// matches one of them twice over -- the documented exception this file's
// Activity tests already take. `:not(details *)` is what separates the
// "Your visits" table from the ledger's, which lives inside the
// disclosure.
//
// Re-read on every poll rather than captured once: `sections` is rebuilt
// when the Visit page arrives, so a node held from before that is
// detached and keeps the empty table's text forever.
function visitsTableText(container: HTMLElement) {
	return () => container.querySelector(':scope .table-view:not(details *)')?.textContent;
}

// #478: CONTEXT.md's Visit entry settles the Client register's word for
// this section -- "visits", "Your visits" as a heading -- and settles
// what a Client is told about one: when it is, and who is coming.
/** The `#478` Your visits block's setup: the fixture's two Visits are the
 * happy path, and a state it has to hold for is a departure passed here
 * rather than a second screen written out. `refusal` is the read failing
 * rather than answering, which no list content can express. */
async function setupVisits({ items = visits, refusal }: { items?: unknown[]; refusal?: string } = {}) {
	if (refusal === undefined) {
		mockFetch(detail, [], items);
	} else {
		mockRefusedList('/visits', refusal);
	}
	// `container` alone: `visitsTableText` reaches past the accessibility
	// tree for the reason its own comment gives, and nothing here needs
	// the rest of the render result.
	const { container } = await render(Hub);
	return { container };
}

describe('Your visits (#478)', () => {
	it("heads the section with the register's own word", async () => {
		await setupVisits();

		await expect.element(page.getByRole('heading', { name: 'Your visits' })).toBeVisible();
	});

	// The acceptance criterion in the ticket's own words: "Thursday at
	// 2pm" and "she came on 18 August" must render differently. The server
	// sends `hasHappened`; the two formats are what a Client sees, so
	// this asserts the strings rather than the flag. Both are read out of
	// the fixture in the runner's own zone, which is what a Client's
	// browser does with the same instant.
	it('renders a scheduled Visit and a past one differently, each naming who is coming', async () => {
		await page.viewport(1440, 900);

		const { container } = await setupVisits();
		const text = visitsTableText(container);

		// The upcoming one: weekday first, then the clock, because the
		// hour is the part she has to be ready for. Both instants are read
		// in the runner's own zone, which is what a Client's browser does
		// with the same value.
		const upcoming = new Date(visits[0]!.scheduledAt);
		const weekday = upcoming.toLocaleDateString('en-US', { weekday: 'short' });
		const month = upcoming.toLocaleDateString('en-US', { month: 'short' });
		await expect
			.poll(text)
			.toMatch(new RegExp(`${weekday} ${upcoming.getDate()} ${month}, ${String.raw`\d+:\d\d[ap]m`}`));
		expect(text()).toContain(visits[0]!.doulaName);

		// The past one: the calendar day it was on, no weekday and no
		// clock -- neither is what she is checking a month later.
		const happened = new Date(visits[1]!.scheduledAt);
		const happenedMonth = happened.toLocaleDateString('en-US', { month: 'short' });
		expect(text()).toContain(`${happened.getDate()} ${happenedMonth} ${happened.getFullYear()}`);
		expect(text()).toContain(visits[1]!.doulaName);
		// The past row carries no weekday and no clock of its own -- the
		// whole of what tells the two apart on the page.
		const pastDay = `${happened.getDate()} ${happenedMonth}`;
		expect(text()).not.toMatch(
			new RegExp(`${String.raw`\w{3} `}${pastDay}|${pastDay} ${String.raw`\d{4}, \d`}`)
		);
	});

	// A Visit's derived type is staff-only (CONTEXT.md's Visit entry), and
	// naming a bereavement Visit `postpartum` in Nadia's own portal is the
	// CB-G5 mistake this surface exists not to repeat. The server sends no
	// type at all; this proves the screen renders none either, from a
	// payload that carried one anyway.
	it('names no Visit type, even if the response carries one', async () => {
		await setupVisits({ items: [{ ...visits[0], type: 'postpartum' }] });

		await expect.element(page.getByRole('heading', { name: 'Your visits' })).toBeVisible();
		await expect.element(page.getByText(/prenatal|postpartum/i)).not.toBeInTheDocument();
	});

	// CB-G5: the empty state has to be true for a postpartum-only
	// Engagement, where nothing prenatal was ever coming, and for one
	// where the Practice has booked nothing yet -- so it reports the state
	// of her own list and promises no visit at all.
	it('promises nothing when nothing is booked', async () => {
		await page.viewport(1440, 900);

		const { container } = await setupVisits({ items: [] });

		await expect.poll(visitsTableText(container)).toContain('Nothing is booked yet.');
		await expect.element(page.getByText(/will be|soon|shortly|coming up/i)).not.toBeInTheDocument();
	});

	it('says so when the visits cannot be read', async () => {
		await setupVisits({ refusal: 'no visits for you' });

		await expect.element(page.getByText('no visits for you')).toBeVisible();
	});

	// The next Visit is the answer both journeys came for, so the
	// scheduled ones read soonest-first on the page even though the
	// response is ordered furthest-future first (inReadingOrder, and its
	// own doc comment). Two scheduled Visits are what makes that order
	// visible at all, which the fixture's single upcoming row cannot show
	// -- a departure from it, spread rather than restated.
	it('puts the soonest scheduled Visit above a later one', async () => {
		await page.viewport(1440, 900);
		const later = { ...visits[0]!, visitId: 'visit-later', scheduledAt: '2027-04-20T15:00:00Z' };

		const { container } = await setupVisits({ items: [later, ...visits] });
		const text = visitsTableText(container);

		const soonest = new Date(visits[0]!.scheduledAt);
		const soonestDay = `${soonest.getDate()} ${soonest.toLocaleDateString('en-US', { month: 'short' })}`;
		const furthest = new Date(later.scheduledAt);
		const furthestDay = `${furthest.getDate()} ${furthest.toLocaleDateString('en-US', { month: 'short' })}`;
		await expect.poll(text).toContain(furthestDay);
		expect(text()!.indexOf(soonestDay)).toBeLessThan(text()!.indexOf(furthestDay));
	});
});
