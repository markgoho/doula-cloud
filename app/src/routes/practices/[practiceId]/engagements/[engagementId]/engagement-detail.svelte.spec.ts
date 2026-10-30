import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { jsonResponse } from '#lib/testResponse.js';
import { findDuplicateIds } from '#lib/duplicateIds.js';
import Page from './EngagementScreen.svelte';
// Rendering `+page.svelte` directly bypasses `+layout.svelte`, the only
// place the real app calls this -- without it a layout primitive like
// `center-l`'s own `max` override sits unregistered and inert (matching
// clients-list.svelte.spec.ts's own reason for the same line). #486's own
// DataTable assertions below need the real table-view/record-view switch.
import '#lib/styles/app.css';
import { toApiResponder, toPageState } from '../../../../routeFixture.js';
import {
	contract as fixtureContract,
	contractorInvoices,
	detail as fixtureDetail,
	fixture,
	session
} from './page.fixture.js';
import type { Contract } from '#lib/contract.js';
import type { Rate } from '#lib/rates.js';
import { PREVIOUS_CONTRACT_PDF_NOTICE } from '#lib/components/molecules/ContractStatus.svelte';
if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The `page` this route reads comes from its own fixture (#596), so the
 * params this spec installs and the params the continuum sweep installs
 * are one description. `vi.mock` is hoisted above every import, so the
 * object is declared empty here and filled from the fixture once the
 * imports have run.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession
}));

// A real service-worker push subscription would fail in headless Chromium
// -- the summary this spec covers doesn't depend on it, so it's stood in
// with a no-op unsubscribe.
vi.mock('#lib/pushRefresh.js', () => ({ subscribeToThreadPushMessages: vi.fn(() => vi.fn()) }));

interface Detail {
	engagementId: string;
	clientId: string;
	clientName: string;
	clientDisplayName: string;
	status: string;
	createdAt: string;
	kind: string;
	dueDate?: string;
	statusMoves: string[];
	birthOutcome?: string;
	pregnancyEndedOn?: string;
	clientPortalInviteStatus?: string;
	clientEmailSuppressed?: boolean;
	clientHasEmail?: boolean;
	doulas: { staffId: string; name: string }[];
	canAttachSelf: boolean;
	yourFee?: { amountCents: number; acceptedOn: string };
}

// The Engagement is handed in as `data` rather than stubbed out of a
// fetch: it comes from +page.ts's load now (#695), and that load has its
// own spec. What remains mocked is the seven sections that still fill in
// after mount, every one answered with a refusal by default -- each
// section's own loader already treats that as "nothing to show" rather
// than throwing. `activityResponse` lets #486's own tests below answer
// just that one path differently, without a blanket mock stopping being
// useful to every other test in this file.
// `session` merges in from practices/[practiceId]/+layout.ts (#835) -- the
// generated `data` prop type requires it either way, since SvelteKit
// really does merge ancestor layout data into it at runtime. `roles: []`
// is fine for every test using `setup()` below: only the Contract PDF
// download (#302, its own describe block further down) reads
// `session.roles` at all, and it calls this with its own roles.
// `staffId` defaults to the roster's own first entry (#909), who holds
// `['owner', 'doula']` in the fixture -- the Doula-Owner this ticket is
// about, and the reader the picker opens on. A test about somebody the
// roster does not contain passes an id of its own.
// #885: a departure from the fixture's own session rather than a second
// description of one -- the Practice it names and the Practice the
// `respond` paths match on cannot drift apart if only one of them says it.
function sessionFor(roles: string[] = [], staffId = 'staff-1', isContractor = false) {
	return { ...session, staffId, roles, isContractor };
}

async function setup(detail: Detail, activityResponse?: Response, outcome?: string) {
	await testPage.viewport(3200, 900);
	if (activityResponse) {
		apiFetchWithSession.mockImplementation((path: string) =>
			Promise.resolve(path.includes('/activity') ? activityResponse : jsonResponse('not available', 403))
		);
	} else {
		apiFetchWithSession.mockResolvedValue(jsonResponse('not available', 403));
	}
	// No `params` prop: the screen reads `page.params`, which the
	// $app/state mock above supplies from the fixture, so the two cannot
	// disagree about which Engagement this is (#596).
	await render(Page, {
		data: { ...detail, session: sessionFor() },
		outcome
	});
}

// #2199: what the page and the BFF both say to completing an Engagement
// with no birth outcome.
const OUTCOME_FIRST = 'Record what happened to the pregnancy first. If the Practice never learned, that is an answer too.';

// Renders directly rather than through setup(): that helper's own
// activityResponse-less branch always overwrites apiFetchWithSession
// with a blanket 403 (its "everything refuses by default" default),
// which would erase the fixture responder a caller sets up itself -- the
// #841 test below takes the same direct-render path for the same reason.
async function renderWithFixtureResponder(
	respondOverride?: (path: string, init?: RequestInit) => Promise<Response> | undefined,
	detail: Detail = fixtureDetail
) {
	await testPage.viewport(3200, 900);
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		const overridden = respondOverride?.(path, init);
		if (overridden) return overridden;
		return respond(path);
	});
	await render(Page, {
		data: { ...detail, session: sessionFor() }
	});
}

// #268: the fixture responder, with the roster read answered separately
// -- a 200 is the Owner or Admin who may be offered a colleague to pick,
// a 403 is the plain Doula the endpoint refuses. At the outer scope
// because eslint's unicorn/consistent-function-scoping asks for it there.
//
// The read the pickers are drawn from is the Engagement-scoped one now
// (#911), not the Practice-wide roster, so that is the path a refusal or
// an outage is injected on.
async function setupWithRoster(
	rosterResponse?: Response,
	roles: string[] = ['owner', 'doula'],
	staffId = 'staff-1'
) {
	await testPage.viewport(3200, 900);
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string) => {
		if (rosterResponse && path.endsWith('/visit-assignees')) return Promise.resolve(rosterResponse);
		return respond(path);
	});
	await render(Page, {
		data: { ...fixtureDetail, session: sessionFor(roles, staffId) }
	});
}

// Shared by the #255, #258 and #1119 blocks below: each needs the
// Contract section actually enabled, which `setup()`'s own "everything
// refuses" default cannot give it, and each installs that differently
// (`prepareApi`, called after the viewport and before the render) --
// `mockContract`/`respondFromFixture`, defined further down this file,
// are what most callers reach for there.
async function setupWithContract(
	prepareApi: () => void,
	detail: Detail = fixtureDetail,
	contractSession: typeof session = sessionFor()
) {
	await testPage.viewport(3200, 900);
	prepareApi();
	await render(Page, {
		data: { ...detail, session: contractSession }
	});
}

/*
 * The Visits `<table>`: the first DataTable this page renders, and the
 * tree the wide viewport above actually shows. Scoping to it is the
 * querySelector exception's case 1 (.claude/rules/svelte-tests.md) --
 * two elements with the identical role and name, told apart only by
 * which one CSS hides -- and the same one the Staff roster's own spec
 * takes.
 * DataTable builds its record view alongside the table at every width, so
 * an unscoped count of a per-Visit control counts trees as well as
 * Visits.
 */
function visitsTable() {
	return testPage.elementLocator(document.querySelector('.table-view')!);
}

/*
 * #1999: a Visit row's one action, opening that Visit's change form under
 * the table. `row` is the row's place in the table, which lists the
 * fixture's scheduled Visit (Anne-Marie Ochieng-Whitfield) before its
 * unscheduled one (Jordan Reyes).
 */
async function changeVisit(row: number) {
	await visitsTable().getByRole('button', { name: 'Change' }).nth(row).click();
}

// A Visit as the BFF sends it, for the order case below (#1999).
function visitRow(visitId: string, staffName: string, createdAt: string, scheduledAt?: string) {
	return { visitId, staffId: 'staff-1', staffName, createdAt, ...(scheduledAt && { scheduledAt }), type: 'prenatal' };
}

// #1999: the add form opens only when asked, from its one opener.
async function openAddVisit() {
	await testPage.getByRole('button', { name: 'Add a Visit' }).click();
}

/*
 * #1998: the summary card's "Change" for one row. Every row's button reads
 * "Change", and the visually-hidden words it is described by name the row
 * (SummaryList), so that is how a test tells them apart, the same way a
 * screen reader does.
 */
function changeButton(changes: string): HTMLButtonElement | undefined {
	return [...document.querySelectorAll('button')].find(
		(button) =>
			button.textContent?.trim() === 'Change' &&
			document.querySelector(`#${CSS.escape(button.getAttribute('aria-describedby') ?? '')}`)?.textContent ===
				changes
	);
}

async function openChange(changes: string) {
	await vi.waitFor(() => expect(changeButton(changes)).toBeDefined());
	await testPage.elementLocator(changeButton(changes)!).click();
}

// #2118: a birth Engagement with no due date and no birth outcome yet --
// one approved without a date, or moved from postpartum to birth.
const noDueDateDetail: Detail = {
	...fixtureDetail,
	dueDate: undefined,
	birthOutcome: undefined,
	pregnancyEndedOn: undefined
};

describe('Staff Engagement detail summary', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	// #2004: her full name, the title her own Client record carries, not
	// the first name the page's sentences use.
	it("titles the page with the Client's full name", async () => {
		await setup(fixtureDetail);

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: fixtureDetail.clientDisplayName }))
			.toBeVisible();
	});

	// #1998: the drawing's summary card -- Care, Due, Care phase, Doula --
	// with the two rows the drawing does not have after them. The Client's
	// name is the h1, so it is no longer a row.
	it('lists the rows in the drawn order, in one bordered card', async () => {
		await setup(fixtureDetail);

		await expect.element(testPage.getByText('Care phase', { exact: true })).toBeVisible();
		const list = document.querySelector('dl.bordered')!;
		expect([...list.querySelectorAll(':scope dt')].map((node) => node.textContent)).toEqual([
			'Care',
			'Due',
			'Care phase',
			'Doulas',
			'Portal invite',
			'Created'
		]);
	});

	it('gives an Owner who is a Doula a Change on each row she may change, Due among them (#2013)', async () => {
		await testPage.viewport(3200, 900);
		apiFetchWithSession.mockResolvedValue(jsonResponse('not available', 403));
		await render(Page, { data: { ...fixtureDetail, session: sessionFor(['owner', 'doula']) } });

		await vi.waitFor(() => expect(changeButton('care')).toBeDefined());
		expect(changeButton('care phase')).toBeDefined();
		expect(changeButton('due date')).toBeDefined();
	});

	// A contractor Doula may change none of these facts, and the BFF
	// answers her with no status moves and no canAttachSelf, so her card
	// has no Change at all: an action she may not take is not shown.
	it('gives a contractor Doula no Change on any row', async () => {
		await testPage.viewport(3200, 900);
		apiFetchWithSession.mockResolvedValue(jsonResponse('not available', 403));
		await render(Page, {
			data: {
				...fixtureDetail,
				statusMoves: [],
				canAttachSelf: false,
				session: { ...sessionFor(['doula']), isContractor: true }
			}
		});

		await expect.element(testPage.getByText('Care phase', { exact: true })).toBeVisible();
		expect(document.querySelectorAll('dl.bordered button')).toHaveLength(0);
		await expect.element(testPage.getByRole('link', { name: 'Message the Client' })).toBeVisible();
	});

	it('draws the "← Clients" back link and the header actions above the summary', async () => {
		await setup(fixtureDetail);

		await expect
			.element(testPage.getByRole('link', { name: 'Clients', exact: true }))
			.toHaveAttribute('href', '/practices/practice-1/clients');
		await expect
			.element(testPage.getByRole('link', { name: 'Message the Client' }))
			.toHaveAttribute('href', '#messages');
	});

	it('shows the due date under its own label, alongside Created (#538)', async () => {
		// The fixture's own Engagement (#596) already carries a due date.
		await setup(fixtureDetail);

		await expect.element(testPage.getByText('Due', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText('Mar 1, 2027')).toBeVisible();
		await expect.element(testPage.getByText('Created')).toBeVisible();
	});

	// ADR-0017: a postpartum-only Engagement has no due date. #538 asks for
	// nothing to show, not a blank row and not a placeholder -- the row is
	// left out of the DescriptionList's own items entirely. The fixture's
	// Engagement always carries a due date, so a Detail with none is this
	// test's own -- the fixture has no way to hold the absence (#596).
	it('shows nothing for a postpartum Engagement with no due date -- no blank label, no placeholder', async () => {
		await setup({ ...fixtureDetail, kind: 'postpartum', dueDate: undefined });

		await expect.element(testPage.getByText('Active', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText('Due', { exact: true })).not.toBeInTheDocument();
	});

	// #2118: once a birth outcome is recorded the card does not ask for a
	// first due date. After a loss, or when the Practice never learned what
	// happened, ADR-0015 standing rule 1 forbids presuming an expected
	// baby; after a live birth the pregnancy has ended, the On-call window
	// ends on the date it ended, and a first date would drive nothing (the
	// reason canChangeKind stops offering postpartum to birth).
	it.each(['live_birth', 'loss', 'unknown'])('shows no Due row on a birth Engagement with no due date after %s', async (outcome) => {
		await setup({ ...fixtureDetail, dueDate: undefined, birthOutcome: outcome });

		await expect.element(testPage.getByText('Active', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText('Due', { exact: true })).not.toBeInTheDocument();
	});

	// A contractor Doula reads the gap, and has no Change to fill it.
	it('shows a contractor Doula the missing due date as not given, with no Change', async () => {
		await testPage.viewport(3200, 900);
		apiFetchWithSession.mockResolvedValue(jsonResponse('not available', 403));
		await render(Page, {
			data: {
				...noDueDateDetail,
				statusMoves: [],
				canAttachSelf: false,
				session: { ...sessionFor(['doula']), isContractor: true }
			}
		});

		await expect.element(testPage.getByText('Not given', { exact: true })).toBeVisible();
		expect(document.querySelectorAll('dl.bordered button')).toHaveLength(0);
	});
});

describe('a birth Engagement with no due date (#2118)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('shows the Due row as not given, in its drawn place', async () => {
		await setupDueDate(['owner'], undefined, noDueDateDetail);

		await expect.element(testPage.getByText('Not given', { exact: true })).toBeVisible();
		const list = document.querySelector('dl.bordered')!;
		expect([...list.querySelectorAll(':scope dt')].map((node) => node.textContent).slice(0, 3)).toEqual([
			'Care',
			'Due',
			'Care phase'
		]);
	});

	it('opens the date form with three empty boxes', async () => {
		await setupDueDate(['doula'], undefined, noDueDateDetail);
		await openChange('due date');

		await expect.element(dueDateGroup().getByLabelText('Month')).toHaveValue('');
		await expect.element(dueDateGroup().getByLabelText('Day')).toHaveValue('');
		await expect.element(dueDateGroup().getByLabelText('Year')).toHaveValue('');
	});

	it('saves a first date and shows it on the card', async () => {
		await setupDueDate(['owner'], undefined, noDueDateDetail);
		await openChange('due date');
		await dueDateGroup().getByLabelText('Month').fill('3');
		await dueDateGroup().getByLabelText('Day').fill('15');
		await dueDateGroup().getByLabelText('Year').fill('2027');

		await testPage.getByRole('button', { name: 'Save the due date' }).click();

		await expect.element(testPage.getByText('The due date is now Mar 15, 2027.')).toBeVisible();
		await expect.element(testPage.getByText('Mar 15, 2027', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText('Not given', { exact: true })).not.toBeInTheDocument();
		expect(callsTo('/due-date').at(-1)?.[1]).toEqual({
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ dueDate: '2027-03-15' })
		});
	});
});

// #2013: the page as a reader with roles sees it, every read answered by
// the fixture and the due-date write answered by `dueResponse`.
async function setupDueDate(
	roles: string[],
	dueResponse: Response = jsonResponse({ engagementId: 'x', dueDate: '2027-03-15' }),
	detail: Detail = fixtureDetail
) {
	await testPage.viewport(3200, 900);
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string) =>
		path.endsWith('/due-date') ? Promise.resolve(dueResponse) : respond(path)
	);
	await render(Page, { data: { ...detail, session: sessionFor(roles) } });
}

function dueDateGroup() {
	return testPage.getByRole('group', { name: 'New due date' });
}

function callsTo(suffix: string) {
	return apiFetchWithSession.mock.calls.filter(([path]) => String(path).endsWith(suffix));
}

describe("the due date's Change (#2013)", () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('gives an employee Doula the Change too, the same role table as the other facts', async () => {
		await setupDueDate(['doula']);

		await vi.waitFor(() => expect(changeButton('due date')).toBeDefined());
	});

	it('opens the date in three boxes holding the date there now', async () => {
		await setupDueDate(['owner']);
		await openChange('due date');

		await expect.element(dueDateGroup()).toBeVisible();
		await expect.element(dueDateGroup().getByLabelText('Month')).toHaveValue('03');
		await expect.element(dueDateGroup().getByLabelText('Day')).toHaveValue('01');
		await expect.element(dueDateGroup().getByLabelText('Year')).toHaveValue('2027');
		await expect.element(testPage.getByRole('button', { name: 'Save the due date' })).toBeVisible();
	});

	it('saves the new date, closes the form and shows the new date on the card', async () => {
		await setupDueDate(['owner']);
		await openChange('due date');
		await dueDateGroup().getByLabelText('Day').fill('15');
		const onCallReads = callsTo('/on-call').length;
		const activityReads = apiFetchWithSession.mock.calls.filter(([path]) => String(path).includes('/activity')).length;

		await testPage.getByRole('button', { name: 'Save the due date' }).click();

		await expect.element(testPage.getByText('The due date is now Mar 15, 2027.')).toBeVisible();
		await expect.element(testPage.getByText('Mar 15, 2027', { exact: true })).toBeVisible();
		await expect.element(dueDateGroup()).not.toBeInTheDocument();
		expect(callsTo('/due-date').at(-1)?.[1]).toEqual({
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ dueDate: '2027-03-15' })
		});
		// The On-call window and the ledger both follow the new date.
		await vi.waitFor(() => expect(callsTo('/on-call').length).toBeGreaterThan(onCallReads));
		await vi.waitFor(() =>
			expect(apiFetchWithSession.mock.calls.filter(([path]) => String(path).includes('/activity')).length).toBeGreaterThan(
				activityReads
			)
		);
	});

	it('refuses a date with a box left empty before anything is sent', async () => {
		await setupDueDate(['owner']);
		await openChange('due date');
		await dueDateGroup().getByLabelText('Year').fill('');

		await testPage.getByRole('button', { name: 'Save the due date' }).click();

		await expect.element(testPage.getByRole('link', { name: 'The due date must include a year' })).toBeVisible();
		expect(callsTo('/due-date')).toHaveLength(0);
	});

	it('asks for a date when all three boxes are emptied', async () => {
		await setupDueDate(['owner']);
		await openChange('due date');
		await dueDateGroup().getByLabelText('Month').fill('');
		await dueDateGroup().getByLabelText('Day').fill('');
		await dueDateGroup().getByLabelText('Year').fill('');

		await testPage.getByRole('button', { name: 'Save the due date' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Enter the due date' })).toBeVisible();
		expect(callsTo('/due-date')).toHaveLength(0);
	});

	it("puts the BFF's own refusal on the boxes and leaves the form open", async () => {
		const refusal = jsonResponse(
			{
				code: 'INVALID_ARGUMENT',
				message: 'The due date must be a calendar day, YYYY-MM-DD',
				details: { dueDate: 'Enter the due date as a real date, like 2027-04-23' }
			},
			400
		);
		await setupDueDate(['owner'], refusal);
		await openChange('due date');

		await testPage.getByRole('button', { name: 'Save the due date' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Enter the due date as a real date, like 2027-04-23' }))
			.toBeVisible();
		await expect.element(dueDateGroup()).toBeVisible();
	});

	it('closes the form on Cancel, leaving the date as it was', async () => {
		await setupDueDate(['owner']);
		await openChange('due date');

		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(dueDateGroup()).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Mar 1, 2027')).toBeVisible();
	});
});

// #1611, #1710: a save that leads here says what it did, such as an
// approved start at the end of a new Client's flow. +page.svelte takes
// the message while the save's navigation runs and hands it over
// (`streamed-pages.svelte.spec.ts`).
describe('the status message after a save that leads here (#1611, #1710)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('says what the save did, as a status, and takes focus', async () => {
		await setup(fixtureDetail, undefined, `Work with ${fixtureDetail.clientName} started.`);

		const message = testPage.getByRole('status').filter({ hasText: `Work with ${fixtureDetail.clientName} started.` });
		await expect.element(message).toBeVisible();
		await expect.element(message).toHaveFocus();
	});

	it('says nothing on a visit no save sent, such as a reload', async () => {
		await setup(fixtureDetail);

		await expect.element(testPage.getByRole('heading', { level: 1 })).toBeVisible();
		await expect.element(testPage.getByText(/started\.$/)).not.toBeInTheDocument();
	});
});

// #253: the status-move controls -- exactly what Detail.statusMoves
// names, nothing hand-decided in the component.
describe('the status-move controls (#253)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('shows no status control at all when statusMoves is empty', async () => {
		await setup({ ...fixtureDetail, statusMoves: [] });

		await expect.element(testPage.getByRole('button', { name: 'Mark care complete' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: /reopen|mark care as active/i })).not.toBeInTheDocument();
		// #1998: and the Care phase row has no Change to open.
		expect(changeButton('care phase')).toBeUndefined();
	});

	it('moves status directly for a move that is not completing, and reflects it without a reload', async () => {
		const requests: { path: string; body: unknown }[] = [];
		await renderWithFixtureResponder(
			(path, init) => {
				if (!init || init.method !== 'PATCH' || !path.endsWith('/status')) return;
				requests.push({ path, body: init.body ? JSON.parse(init.body as string) : undefined });
				return Promise.resolve(jsonResponse({ status: 'active', statusMoves: ['completed'] }));
			},
			{ ...fixtureDetail, status: 'completed', statusMoves: ['active'] }
		);

		await openChange('care phase');
		await testPage.getByRole('button', { name: 'Reopen (correction)' }).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.body).toEqual({ status: 'active' });
		await expect.element(testPage.getByText('Active', { exact: true })).toBeVisible();
		// #1998: the save closes the row's form and says what changed, in
		// the focus the pressed button had.
		await expect.element(testPage.getByText('The care phase is now Active.')).toHaveFocus();
		expect(changeButton('care phase')).toHaveAttribute('aria-expanded', 'false');
		expect(testPage.getByRole('button', { name: 'Mark care complete' }).elements()).toHaveLength(0);
	});

	it('asks for a reason before completing, and refuses to submit without one', async () => {
		const requests: unknown[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'PATCH' || !path.endsWith('/status')) return;
			requests.push(init.body);
			return Promise.resolve(jsonResponse({ status: 'completed', statusMoves: ['active'] }));
		});

		await openChange('care phase');
		await testPage.getByRole('button', { name: 'Mark care complete' }).click();
		await testPage.getByRole('button', { name: 'Confirm completion' }).click();

		// GOV.UK's own pattern says the message twice -- once in the error
		// summary's link, once against the radio group itself. Both are
		// role="alert" (the same disambiguation the sibling
		// engagement-requests/new spec uses for its own refusal).
		await expect
			.element(testPage.getByRole('link', { name: 'Select why this Engagement is ending' }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('alert').last())
			.toHaveTextContent('Select why this Engagement is ending');
		expect(requests).toHaveLength(0);
	});

	it('submits the chosen reason and note, then reflects the new status', async () => {
		const requests: { path: string; body: unknown }[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'PATCH' || !path.endsWith('/status')) return;
			requests.push({ path, body: init.body ? JSON.parse(init.body as string) : undefined });
			return Promise.resolve(jsonResponse({ status: 'completed', statusMoves: ['active'] }));
		});

		await openChange('care phase');
		await testPage.getByRole('button', { name: 'Mark care complete' }).click();
		await testPage.getByLabelText('The work finished as agreed').click();
		await testPage.getByLabelText('Note (optional)').fill('Baby arrived safely.');
		await testPage.getByRole('button', { name: 'Confirm completion' }).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.body).toEqual({
			status: 'completed',
			endingReason: 'care_complete',
			endingNote: 'Baby arrived safely.'
		});
		await expect.element(testPage.getByText('Completed', { exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Confirm completion' })).not.toBeInTheDocument();
		await expect.element(testPage.getByText('The care phase is now Completed.')).toHaveFocus();
	});

	// #1998: the completion question now sits inside the summary card's
	// Care phase row, the narrowest room it has had. No sweep presses a
	// Change, so this is where that open form is measured at 320px, with
	// its error summary on screen.
	it('never scrolls the document sideways at 320px with the completion question open', async () => {
		await renderWithFixtureResponder();
		await testPage.viewport(320, 800);

		await openChange('care phase');
		await testPage.getByRole('button', { name: 'Mark care complete' }).click();
		await testPage.getByRole('button', { name: 'Confirm completion' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Select why this Engagement is ending' }))
			.toBeVisible();
		expect(document.documentElement.scrollWidth).toBe(document.documentElement.clientWidth);
	});

	it('shows the refusal when the BFF rejects the move', async () => {
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'PATCH' || !path.endsWith('/status')) return;
			return Promise.resolve(jsonResponse('endingReason is required to complete an Engagement', 400));
		});

		await openChange('care phase');
		await testPage.getByRole('button', { name: 'Mark care complete' }).click();
		await testPage.getByLabelText('The work finished as agreed').click();
		await testPage.getByRole('button', { name: 'Confirm completion' }).click();

		await expect
			.element(testPage.getByText('endingReason is required to complete an Engagement'))
			.toBeVisible();
	});

	// #2199: ADR-0015 will not let an Engagement reach 'completed' while
	// its birth outcome is null, so the page says so before it asks why
	// the Engagement is ending, in the order the rule needs, and links to
	// the section that answers it.
	it('asks for the birth outcome before the ending reason when none is recorded', async () => {
		const requests: unknown[] = [];
		await renderWithFixtureResponder(
			(path, init) => {
				if (!init || init.method !== 'PATCH' || !path.endsWith('/status')) return;
				requests.push(init.body);
				return Promise.resolve(jsonResponse({ status: 'completed', statusMoves: ['active'] }));
			},
			{ ...fixtureDetail, birthOutcome: undefined, pregnancyEndedOn: undefined }
		);

		await openChange('care phase');
		await testPage.getByRole('button', { name: 'Mark care complete' }).click();

		await expect.element(testPage.getByText(OUTCOME_FIRST)).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Record what happened' }))
			.toHaveAttribute('href', '#birth-outcome');
		expect(testPage.getByRole('radiogroup', { name: 'Why is this Engagement ending?' }).elements()).toHaveLength(0);
		expect(testPage.getByRole('button', { name: 'Confirm completion' }).elements()).toHaveLength(0);
		expect(requests).toHaveLength(0);
	});

	// #940: the BFF still refuses with a named 409 where the page's own
	// reading was stale (an outcome removed in another tab), and its words
	// are written for this screen (#2199).
	it('shows the BFF refusal when the Engagement has no birth outcome recorded', async () => {
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'PATCH' || !path.endsWith('/status')) return;
			return Promise.resolve(jsonResponse({ code: 'BIRTH_OUTCOME_REQUIRED', message: OUTCOME_FIRST }, 409));
		});

		await openChange('care phase');
		await testPage.getByRole('button', { name: 'Mark care complete' }).click();
		await testPage.getByLabelText('The work finished as agreed').click();
		await testPage.getByRole('button', { name: 'Confirm completion' }).click();

		await expect.element(testPage.getByText(OUTCOME_FIRST)).toBeVisible();
		// The completion form stays open behind the refusal: the reader's
		// reason and note survive while she goes and records the outcome,
		// rather than being thrown away by a page that reset itself.
		await expect.element(testPage.getByRole('button', { name: 'Confirm completion' })).toBeVisible();
	});
});

// #2105: `count` ledger entries told apart by their actor, "Doula 1" up.
function ledgerEntries(count: number, firstActor = 1) {
	return Array.from({ length: count }, (_, index) => ({
		action: 'visit_logged',
		actorKind: 'staff',
		actorName: `Doula ${firstActor + index}`,
		createdAt: new Date().toISOString()
	}));
}

// #486 AC4: the same ledger treatment reused on the staff Engagement page,
// through the record-scoped read engagement.ListActivityHandler already
// exposes at /activity -- last in the page's own sections.
describe('the Activity ledger section (#486)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('renders the ledger under an Activity heading, low on the page', async () => {
		await setup(
			fixtureDetail,
			jsonResponse({
				items: [
					{
						subjectKind: 'engagement',
						subjectId: fixtureDetail.engagementId,
						action: 'visit_logged',
						actorKind: 'staff',
						actorName: 'Maya Torres',
						createdAt: new Date().toISOString()
					}
				],
				hasMore: false
			})
		);

		await expect.element(testPage.getByRole('heading', { name: 'Activity' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Visit added for the Client' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Maya Torres' })).toBeVisible();
	});

	it('shows who changed a fact, from what, to what (#1423)', async () => {
		await setup(
			fixtureDetail,
			jsonResponse({
				items: [
					{
						action: 'kind_changed',
						diff: { kindBefore: 'birth', kindAfter: 'postpartum' },
						actorKind: 'staff',
						actorName: 'Maya Torres',
						createdAt: new Date().toISOString()
					}
				],
				hasMore: false
			})
		);

		await expect.element(testPage.getByRole('cell', { name: 'Type of care changed for the Client: Birth to Postpartum' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Maya Torres' })).toBeVisible();
	});

	it('shows the empty-ledger message when the Engagement has no activity yet', async () => {
		await setup(fixtureDetail, jsonResponse({ items: [], hasMore: false }));

		await expect.element(testPage.getByText('Nothing has happened yet.')).toBeVisible();
	});

	it('says so when the ledger cannot be read', async () => {
		await setup(fixtureDetail, jsonResponse('nope', 403));

		await expect.element(testPage.getByText('nope')).toBeVisible();
	});

	// #2105: the newest eight, as the Overview's Recent activity shows, and
	// "Load more" for the rest in place.
	it('shows the newest eight entries, and the rest after "Load more"', async () => {
		await setup(fixtureDetail, jsonResponse({ items: ledgerEntries(9), hasMore: false }));

		await expect.element(testPage.getByRole('cell', { name: 'Doula 8', exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Doula 9', exact: true })).not.toBeInTheDocument();

		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('cell', { name: 'Doula 9', exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Load more' })).not.toBeInTheDocument();
	});

	it('reads the next page once every entry already read is shown', async () => {
		apiFetchWithSession.mockImplementation((path: string) => {
			if (path.includes('/activity?cursor=next')) {
				return Promise.resolve(jsonResponse({ items: ledgerEntries(1, 10), hasMore: false }));
			}
			if (path.includes('/activity')) {
				return Promise.resolve(jsonResponse({ items: ledgerEntries(9), hasMore: true, nextCursor: 'next' }));
			}
			return Promise.resolve(jsonResponse('not available', 403));
		});
		await testPage.viewport(3200, 900);
		await render(Page, { data: { ...fixtureDetail, session: sessionFor() } });

		await testPage.getByRole('button', { name: 'Load more' }).click();
		await expect.element(testPage.getByRole('cell', { name: 'Doula 9', exact: true })).toBeVisible();

		await testPage.getByRole('button', { name: 'Load more' }).click();
		await expect.element(testPage.getByRole('cell', { name: 'Doula 10', exact: true })).toBeVisible();
	});
});

// #250: a Visit's own scheduled date, distinct from createdAt.
describe('the Visits section Date column and schedule control (#250)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('shows a formatted date for a scheduled Visit and "Not yet scheduled" for one without', async () => {
		await renderWithFixtureResponder();

		// The fixture's own two rows (page.fixture.ts): visit-1 carries
		// scheduledAt, visit-2 carries none. The expected cell text is built
		// from the same Date methods formatScheduledVisit itself uses
		// (dates.spec.ts's own reason) rather than a literal UTC string, so
		// this passes regardless of the runner's own time zone.
		const scheduled = new Date('2027-03-15T14:30:00Z');
		const hour12 = scheduled.getHours() % 12 === 0 ? 12 : scheduled.getHours() % 12;
		const minute = scheduled.getMinutes().toString().padStart(2, '0');
		const suffix = scheduled.getHours() < 12 ? 'am' : 'pm';
		const expected = `${scheduled.toLocaleDateString('en-US', { month: 'short' })} ${scheduled.getDate()}, ${scheduled.getFullYear()}, ${hour12}:${minute}${suffix}`;
		await expect.element(testPage.getByRole('cell', { name: expected, exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Not yet scheduled', exact: true })).toBeVisible();
	});

	it("sets an unscheduled Visit's schedule from its Change, and sends nothing for the fields left alone", async () => {
		const requests: { path: string; method?: string; body: unknown }[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (!init?.method || init.method === 'GET') return;
			requests.push({ path, method: init.method, body: init.body ? JSON.parse(init.body as string) : undefined });
			return Promise.resolve(jsonResponse({ visitId: 'visit-2', scheduledAt: '2027-04-01T09:00:00Z' }));
		});

		// Jordan Reyes is visit-2, the fixture's unscheduled row (#250).
		await changeVisit(1);
		await expect
			.element(testPage.getByRole('heading', { name: 'Change the Visit for Jordan Reyes, not yet scheduled' }))
			.toHaveFocus();
		await testPage.getByLabelText('Scheduled date and time', { exact: true }).fill('2027-04-01T09:00');
		await testPage.getByRole('button', { name: 'Save changes' }).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.path).toContain('/visits/visit-2/schedule');
		expect(requests[0]!.body).toEqual({ scheduledAt: new Date('2027-04-01T09:00').toISOString() });
		// #1999: a save returns to the rows, and says what it did.
		await expect.element(testPage.getByText('The Visit for Jordan Reyes is saved.')).toHaveFocus();
		await expect.element(testPage.getByRole('button', { name: 'Save changes' })).not.toBeInTheDocument();
	});

	it("clears a scheduled Visit's schedule when its date is emptied", async () => {
		const requests: { path: string; body: unknown }[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (init?.method !== 'PATCH') return;
			requests.push({ path, body: init.body ? JSON.parse(init.body as string) : undefined });
			return Promise.resolve(jsonResponse({ visitId: 'visit-1' }));
		});

		await changeVisit(0);
		await testPage.getByLabelText('Scheduled date and time', { exact: true }).fill('');
		await testPage.getByRole('button', { name: 'Save changes' }).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.path).toContain('/visits/visit-1/schedule');
		expect(requests[0]!.body).toEqual({});
	});

	it('closes a change that changed nothing without sending anything', async () => {
		const writes: string[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (init?.method && init.method !== 'GET') writes.push(path);
			return;
		});

		await changeVisit(0);
		await testPage.getByRole('button', { name: 'Save changes' }).click();

		await expect.element(testPage.getByRole('button', { name: 'Save changes' })).not.toBeInTheDocument();
		expect(writes).toEqual([]);
	});

	it('holds one change open at a time, and the row action closes its own', async () => {
		await renderWithFixtureResponder();

		await changeVisit(0);
		await expect.element(visitsTable().getByRole('button', { name: 'Change' }).nth(0)).toHaveAttribute('aria-expanded', 'true');
		await changeVisit(1);
		await expect
			.element(testPage.getByRole('heading', { name: 'Change the Visit for Jordan Reyes, not yet scheduled' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('heading', { name: /Change the Visit for Anne-Marie/ })).not.toBeInTheDocument();
		await changeVisit(1);
		await expect.element(testPage.getByRole('button', { name: 'Save changes' })).not.toBeInTheDocument();
	});

	it('keeps the Visit as it was when a change is canceled', async () => {
		await renderWithFixtureResponder();

		await changeVisit(0);
		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(testPage.getByRole('button', { name: 'Save changes' })).not.toBeInTheDocument();
		// #2103: back on the row's Change that opened it, not on the page.
		await expect.element(visitsTable().getByRole('button', { name: 'Change' }).nth(0)).toHaveFocus();
	});

	it('says what was refused, keeps the form open, and reads the rows again for what was already written', async () => {
		let visitReads = 0;
		await renderWithFixtureResponder((path, init) => {
			if (!init && path.includes('/visits')) visitReads += 1;
			if (init?.method === 'PATCH' && path.endsWith('/schedule')) return Promise.resolve(jsonResponse({}));
			if (init?.method === 'PATCH' && path.endsWith('/notes')) {
				return Promise.resolve(jsonResponse('notes are down', 500));
			}
			return;
		});
		await expect.poll(() => visitReads).toBe(1);

		await changeVisit(1);
		await testPage.getByLabelText('Scheduled date and time', { exact: true }).fill('2027-04-01T09:00');
		await testPage.getByLabelText('Notes').fill('First check-in went well.');
		await testPage.getByRole('button', { name: 'Save changes' }).click();

		await expect.element(testPage.getByText('notes are down')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Save changes' })).toBeVisible();
		await expect.poll(() => visitReads).toBe(2);
	});

	it('shows the automatic intake -> active move after a Visit is scheduled, without a reload', async () => {
		// #895 / ADR-0015: the BFF activates the Engagement itself in the
		// same transaction as the scheduling write, and neither Visit
		// endpoint answers with the new status -- so the hub re-reads the
		// Engagement rather than leaving "Getting started" on screen.
		await renderWithFixtureResponder(
			(path, init) => {
				if (init?.method === 'PATCH' && path.endsWith('/schedule')) {
					return Promise.resolve(
						jsonResponse({ visitId: 'visit-2', scheduledAt: '2027-04-01T09:00:00Z' })
					);
				}
				if (!init && path.endsWith(`/engagements/${fixture.params.engagementId}`)) {
					return Promise.resolve(
						jsonResponse({ ...fixtureDetail, status: 'active', statusMoves: ['completed'] })
					);
				}
				return;
			},
			{ ...fixtureDetail, status: 'intake', statusMoves: ['active', 'completed'] }
		);

		await expect.element(testPage.getByText('Intake', { exact: true })).toBeVisible();

		await changeVisit(1);
		await testPage.getByLabelText('Scheduled date and time', { exact: true }).fill('2027-04-01T09:00');
		await testPage.getByRole('button', { name: 'Save changes' }).click();

		await expect.element(testPage.getByText('Active', { exact: true })).toBeVisible();
	});

	it('creates a Visit already scheduled from the Add a Visit form', async () => {
		const requests: { path: string; body: unknown }[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'POST' || !path.endsWith('/visits')) return;
			requests.push({ path, body: init.body ? JSON.parse(init.body as string) : undefined });
			return Promise.resolve(jsonResponse({ visitId: 'visit-3', staffId: 'staff-1' }, 201));
		});

		await testPage.getByRole('button', { name: 'Add a Visit' }).click();
		await expect.element(testPage.getByRole('heading', { name: 'Add a Visit' })).toHaveFocus();
		// #1999: the opener goes while its form is open, so the section
		// offers the form's one primary and nothing that opens it again.
		await expect.element(testPage.getByRole('button', { name: 'Add a Visit' })).not.toBeInTheDocument();
		await testPage.getByLabelText('Who is this Visit for?').selectOptions('Jordan Reyes');
		await testPage.getByLabelText('Scheduled date and time (optional)').fill('2027-05-20T10:15');
		await testPage.getByRole('button', { name: 'Save Visit' }).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.path).toContain('/visits');
		expect(requests[0]!.body).toEqual({
			scheduledAt: new Date('2027-05-20T10:15').toISOString(),
			staffId: 'staff-2'
		});
		await expect.element(testPage.getByText('The Visit for Jordan Reyes is added.')).toHaveFocus();
		await expect.element(testPage.getByRole('button', { name: 'Add a Visit' })).toBeVisible();
	});

	it('says why an add was refused, in the add form', async () => {
		await renderWithFixtureResponder((path, init) =>
			init?.method === 'POST' && path.endsWith('/visits')
				? Promise.resolve(jsonResponse('visits are down', 500))
				: undefined
		);

		await testPage.getByRole('button', { name: 'Add a Visit' }).click();
		await testPage.getByRole('button', { name: 'Save Visit' }).click();

		await expect.element(testPage.getByText('visits are down')).toBeVisible();
		await testPage.getByRole('button', { name: 'Cancel' }).click();
		await expect.element(testPage.getByText('visits are down')).not.toBeInTheDocument();
		// #2103: back on the opener, not on the page.
		await expect.element(testPage.getByRole('button', { name: 'Add a Visit' })).toHaveFocus();
	});

	it('lists the scheduled Visits by date, newest first, then the unscheduled ones (#1999)', async () => {
		await renderWithFixtureResponder((path, init) => {
			if (init || !path.includes('/visits')) return;
			// Newest logged first, the order the BFF pages in (#446).
			return Promise.resolve(
				jsonResponse({
					items: [
						visitRow('v-3', 'Logged Last', '2026-09-01T15:00:00Z'),
						visitRow('v-2', 'Aug Eighteen', '2026-08-03T15:00:00Z', '2026-08-18T15:00:00Z'),
						visitRow('v-1', 'Nov Fifth', '2026-08-01T15:00:00Z', '2026-11-05T15:00:00Z')
					],
					hasMore: false
				})
			);
		});

		await expect.element(testPage.getByRole('cell', { name: 'Logged Last', exact: true })).toBeVisible();
		const rows = visitsTable()
			.getByRole('row')
			.elements()
			.slice(1)
			.map((row) => ['Nov Fifth', 'Aug Eighteen', 'Logged Last'].find((name) => row.textContent?.includes(name)));
		expect(rows).toEqual(['Nov Fifth', 'Aug Eighteen', 'Logged Last']);
	});
});

// #251: a Visit's own free-text notes, staff-writable in place.
describe('the Visits section notes control (#251)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it("shows a Visit's own notes, and the empty-state text for one that has never had any", async () => {
		await renderWithFixtureResponder();

		await expect
			.element(
				testPage.getByRole('cell', {
					name: 'She asked a lot of questions about pain management options and wants to keep her options open rather than commit to an unmedicated birth ahead of time. Her partner is nervous about the hospital transfer distance and would like a practice run of the drive before the due date. Follow up next visit on the birth plan draft she is writing.',
					exact: true
				})
			)
			.toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'No notes yet.', exact: true })).toBeVisible();
	});

	it('saves notes for a Visit without any from its Change', async () => {
		const requests: { path: string; body: unknown }[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'PATCH' || !path.endsWith('/notes')) return;
			requests.push({ path, body: init.body ? JSON.parse(init.body as string) : undefined });
			return Promise.resolve(jsonResponse({ visitId: 'visit-2', notes: 'First check-in went well.' }));
		});

		// Jordan Reyes is visit-2, the fixture's row with no prior notes.
		await changeVisit(1);
		await testPage.getByLabelText('Notes').fill('First check-in went well.');
		await testPage.getByRole('button', { name: 'Save changes' }).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.path).toContain('/visits/visit-2/notes');
		expect(requests[0]!.body).toEqual({ notes: 'First check-in went well.' });
	});
});

// #281: a Visit's derived type, rendered as its own column with the
// human label rather than the raw enum value the BFF sends (#262).
describe('the Visits section Type column (#281)', () => {
	it("renders each Visit's derived type as its human label", async () => {
		await renderWithFixtureResponder();

		// The fixture's own two rows (page.fixture.ts): visit-1 is
		// 'postpartum', visit-2 is 'prenatal'. Neither raw enum value
		// ('postpartum'/'prenatal' as sent) collides with its own label
		// here, but the label is what a Doula reads, not the wire value.
		await expect.element(testPage.getByRole('cell', { name: 'Postpartum', exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Prenatal', exact: true })).toBeVisible();
	});
});

// #841: each SectionState is its own instance, so one section's failure
// must not touch another's -- proved here rather than by the default
// "everything answers 403" mock every other test in this file uses, since
// that shows every section failing at once, not one failing independently.
describe('a section fails on its own, independent of the others (#841)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it("shows the Contract section's own failure while Visits still renders its fixture row", async () => {
		await testPage.viewport(3200, 900);
		const respond = toApiResponder(fixture);
		apiFetchWithSession.mockImplementation((path: string) =>
			path.endsWith('/contract') ? Promise.resolve(jsonResponse('contract is down', 500)) : respond(path)
		);

		await render(Page, {
			data: { ...fixtureDetail, session: sessionFor() }
		});

		await expect.element(testPage.getByText('contract is down')).toBeVisible();
		// Scoped to the Visits section by its own label: the fixture's
		// Visit's staffName also appears in the reassign row's
		// visually-hidden name and, once Activity's own fixture entry
		// loads, in that section too -- an unscoped query is ambiguous.
		await expect
			.element(
				testPage.getByLabelText('Visits').getByRole('cell', { name: 'Anne-Marie Ochieng-Whitfield', exact: true })
			)
			.toBeVisible();
	});
});

function pdfBlobResponse(): Response {
	return new Response(new Blob(['%PDF-1.4'], { type: 'application/pdf' }), { status: 200 });
}

// #302: the Practice-side download rides ContractStatus.svelte's own
// onDownloadPdf prop (its own spec covers the click/error mechanics), so
// what this page owns is deciding whether that prop is passed at all --
// Owner/Admin per the endpoint's own OwnerAndAdmin gate (ADR-0008's money
// row), never rendered for a role the endpoint would 403.
// #255: the Client's portal-invite state, shown as standing information
// on the summary, and the Contract section's own block while she has
// never been invited.
/*
 * The fixture's Contract, back in draft (#885). A draft is not this
 * route's happy path -- the fixture ships the signed Contract, the one
 * status that renders the section's own controls -- so it is written as a
 * departure from the fixture rather than as a second Contract of its own
 * (`.claude/rules/svelte-tests.md`). The two fields only a signed
 * Contract can carry are dropped rather than overwritten: a draft has no
 * price change to report and no void to have been asked for. hasSignedPdf
 * goes back to false with them: an Engagement whose only Contract is this
 * Draft has no signed PDF anywhere, and leaving the fixture's true would
 * put a download control into every test below that means a plain Draft.
 */
function draftOf(overrides: Partial<Contract> = {}): Contract {
	const draft: Contract = { ...fixtureContract, status: 'draft', hasSignedPdf: false };
	delete draft.amountChangedAt;
	delete draft.voidRequests;
	return { ...draft, ...overrides };
}

describe("the Client's portal-invite state and the Contract section's block (#255)", () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	// Every merge field already filled, so #258's own completeness block
	// never engages here -- this block is only about #255's precondition.
	const draftContract = draftOf();

	it('reads "Never invited" on the summary when the Client has never been invited', async () => {
		await setup(fixtureDetail);

		await expect.element(testPage.getByText('Portal invite', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText('Never invited')).toBeVisible();
	});

	it('reads the pending word once the fixture reports a pending invite', async () => {
		await setup({ ...fixtureDetail, clientPortalInviteStatus: 'pending' });

		await expect.element(testPage.getByText('Invite pending')).toBeVisible();
	});

	it('offers "Send portal invite" while never invited', async () => {
		await setup(fixtureDetail);

		await expect.element(testPage.getByRole('button', { name: 'Send portal invite' }).first()).toBeVisible();
	});

	it('withholds "Send portal invite" once the Client has accepted one', async () => {
		await setup({ ...fixtureDetail, clientPortalInviteStatus: 'accepted' });

		expect(testPage.getByRole('button', { name: 'Send portal invite' }).elements()).toHaveLength(0);
	});

	// A withheld send is absent, never disabled (govuk-alignment.md, the
	// Button row): the Notice is the reason (#2004).
	it('withholds Send Contract and names the ordering while the Client has never been invited', async () => {
		await setupWithContract(() => mockContract(draftContract));

		await expect
			.element(testPage.getByText(/send a portal invite to this client before sending the contract/i))
			.toBeVisible();
		expect(testPage.getByRole('button', { name: 'Send Contract' }).elements()).toHaveLength(0);
	});

	it('enables Send Contract once the Client has a pending or accepted invite', async () => {
		await setupWithContract(
			() => mockContract(draftContract),
			{ ...fixtureDetail, clientPortalInviteStatus: 'pending' }
		);

		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeEnabled();
		expect(
			testPage
				.getByText(/send a portal invite to this client before sending the contract/i)
				.elements()
		).toHaveLength(0);
	});

	// #270 converted this from a disabled button with a visually-hidden
	// hint into a Notice naming the missing thing, the same shape #275
	// established on InvoiceSection -- block over warn, never a control
	// silently withheld with only a hidden explanation.
	it('replaces "Send portal invite" with a Notice once the Client has no email on file', async () => {
		await setup({ ...fixtureDetail, clientHasEmail: false });

		await expect.element(testPage.getByRole('button', { name: 'Send portal invite' })).not.toBeInTheDocument();
		await expect
			.element(testPage.getByText('This Client has no email address on file. Add one before sending a portal invite.'))
			.toBeVisible();
		await expect
			.element(testPage.getByText(/no email on file, so the client cannot be invited yet/i))
			.toBeVisible();
	});

	it('lifts the Contract section\'s block in one click once a portal invite is sent, without a reload', async () => {
		await setupWithContract(() => {
			const respond = toApiResponder(fixture);
			apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
				if (path.endsWith('/contract')) return Promise.resolve(jsonResponse(draftContract));
				if (path.endsWith('/portal-invite') && init?.method === 'POST') {
					return Promise.resolve(jsonResponse({ inviteToken: 'a-fresh-token' }));
				}
				return respond(path);
			});
		});
		await expect
			.element(testPage.getByText(/send a portal invite to this client before sending the contract/i))
			.toBeVisible();

		await testPage.getByRole('button', { name: 'Send portal invite' }).first().click();

		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeEnabled();
		await expect.element(testPage.getByText('Invite pending')).toBeVisible();
		// #2198: the confirmation names her, never "them".
		await expect
			.element(
				testPage.getByText(
					`Invited. ${fixtureDetail.clientName} has an email with the link. To share the link yourself, here it is:`
				)
			)
			.toBeVisible();
		await expect.element(testPage.getByText(/a-fresh-token/)).toBeVisible();
	});
});

/*
 * The fixture answering every path it knows, with one path answered
 * differently (#885). `toApiResponder` beside its own
 * `mockImplementation` had been written out four times in this file by
 * the time the fixture grew a Contract of its own -- four chances for one
 * of them to consult the fixture first and shadow its own override.
 */
function respondFromFixture(override?: (path: string) => Response | undefined) {
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string) =>
		Promise.resolve(override?.(path) ?? respond(path))
	);
}

// Answers the Practice-side GET .../contract with `contract`, letting
// every other section respond from the fixture as normal -- shared by
// the #255, #258 and #1119 describe blocks, each through `setupWithContract`.
function mockContract(contract: Contract) {
	respondFromFixture((path) => (path.endsWith('/contract') ? jsonResponse(contract) : undefined));
}

// #2004: the Contract's text is behind a disclosure, closed.
async function openContractText() {
	await testPage.getByText('Read the Contract text').click();
}

describe("the Contract's merge-field completeness block and filled-text render (#258)", () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	// clientPortalInviteStatus: 'accepted' throughout, so #255's own block
	// never engages here -- this block is only about #258's own precondition.
	it('renders the Contract prose with merge values substituted, on the Staff side', async () => {
		await setupWithContract(
			() => mockContract(draftOf()),
			{ ...fixtureDetail, clientPortalInviteStatus: 'accepted' }
		);

		// The substitution is what is asserted, so the expected text is
		// built from the fixture's own values rather than restated: the
		// two placeholders either side of "between … and …" have to have
		// been replaced by the Practice's name and the Client's. The text is
		// behind its disclosure, closed (#2004).
		await openContractText();
		await expect
			.element(
				testPage.getByText(
					`between ${fixtureContract.values.practice_name} and ${fixtureContract.values.client_name}`,
					{ exact: false }
				)
			)
			.toBeVisible();
	});

	it('withholds Send Contract and names the blank fields while any merge field is unfilled', async () => {
		await setupWithContract(
			() => mockContract(draftOf({ values: { ...fixtureContract.values, practice_name: '' } })),
			{ ...fixtureDetail, clientPortalInviteStatus: 'accepted' }
		);

		await expect
			.element(
				testPage.getByText(/fill in every merge field before sending the contract\. missing: practice name\./i)
			)
			.toBeVisible();
		expect(testPage.getByRole('button', { name: 'Send Contract' }).elements()).toHaveLength(0);
	});

	it('enables Send Contract once every merge field has a value', async () => {
		await setupWithContract(
			() => mockContract(draftOf()),
			{ ...fixtureDetail, clientPortalInviteStatus: 'accepted' }
		);

		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeEnabled();
	});

	// #1776: a Practice's template can hold no merge field at all. The BFF
	// answers such a Contract with `mergeFields: []`, and the page must
	// render it with nothing to fill in rather than fail.
	it('renders a Contract whose prose holds no merge field, with Send Contract enabled', async () => {
		const prose = 'This agreement has no fields to fill in.';
		await setupWithContract(
			() => mockContract(draftOf({ prose, mergeFields: [], values: {} })),
			{ ...fixtureDetail, clientPortalInviteStatus: 'accepted' }
		);

		await openContractText();
		await expect.element(testPage.getByText(prose)).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeEnabled();
		expect(testPage.getByText(/fill in every merge field/i).elements()).toHaveLength(0);
	});
});

// #1876: Save sends only the merge fields ContractForm offers for editing,
// and price is never one of them, so a Draft with no other merge field has
// nothing to save. The page must not offer a Save control that does nothing.
describe('the Save Contract control on a Draft with no editable merge field (#1876)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('does not offer Save Contract when the Draft holds no merge field', async () => {
		await setupWithContract(
			() => mockContract(draftOf({ prose: 'This agreement has no fields to fill in.', mergeFields: [], values: {} })),
			{ ...fixtureDetail, clientPortalInviteStatus: 'accepted' }
		);

		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeEnabled();
		expect(testPage.getByRole('button', { name: 'Save Contract' }).elements()).toHaveLength(0);
	});

	it('does not offer Save Contract when price is the only merge field', async () => {
		await setupWithContract(
			() =>
				mockContract(
					draftOf({ prose: 'The fee is {{price}}.', mergeFields: ['price'], values: { price: '$1,200.00' } })
				),
			{ ...fixtureDetail, clientPortalInviteStatus: 'accepted' }
		);

		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeEnabled();
		expect(testPage.getByRole('button', { name: 'Save Contract' }).elements()).toHaveLength(0);
	});

	it('offers Save Contract when the Draft holds an editable merge field, once its change is opened', async () => {
		await setupWithContract(
			() => mockContract(draftOf()),
			{ ...fixtureDetail, clientPortalInviteStatus: 'accepted' }
		);

		await testPage.getByRole('button', { name: 'Change the Draft Contract' }).click();

		await expect.element(testPage.getByRole('button', { name: 'Save Contract' })).toBeVisible();
	});
});

// #2004: the Contract is facts first; a Draft's merge fields are a form
// only when asked, which edits a copy, and a save or a send returns to
// the facts with a status that takes focus.
describe('the Contract read first, and its change only when asked (#2004)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	const accepted = { ...fixtureDetail, clientPortalInviteStatus: 'accepted' };

	it('shows the status and fee as rows and no field, with the text behind a closed disclosure', async () => {
		await setupWithContract(() => mockContract(draftOf()), accepted);

		await expect.element(testPage.getByText('Draft, not sent yet')).toBeVisible();
		await expect
			.element(testPage.getByRole('definition').filter({ hasText: fixtureContract.values.price! }))
			.toBeVisible();
		expect(testPage.getByRole('textbox', { name: 'Scope of service' }).elements()).toHaveLength(0);
		await expect.element(testPage.getByText('Read the Contract text')).toBeVisible();
	});

	it('opens the merge fields with the form heading focused, and Cancel keeps what was saved', async () => {
		await setupWithContract(() => mockContract(draftOf()), accepted);

		await testPage.getByRole('button', { name: 'Change the Draft Contract' }).click();
		await expect.element(testPage.getByRole('heading', { level: 3, name: 'Change the Draft Contract' })).toHaveFocus();
		await testPage.getByLabelText('Scope of service').fill('Something else');
		await testPage.getByRole('button', { name: 'Cancel' }).click();
		// #2103: back on the opener, not on the page.
		await expect.element(testPage.getByRole('button', { name: 'Change the Draft Contract' })).toHaveFocus();
		await testPage.getByRole('button', { name: 'Change the Draft Contract' }).click();

		await expect
			.element(testPage.getByLabelText('Scope of service'))
			.toHaveValue(fixtureContract.values.scope_of_service!);
	});

	it('saves only the edited fields, returns to the facts and says so, focused', async () => {
		await setupWithContract(() => {
			const respond = toApiResponder(fixture);
			apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
				if (path.endsWith('/contract') && init?.method === 'PUT') {
					return Promise.resolve(jsonResponse(draftOf({ values: { ...fixtureContract.values, scope_of_service: 'Two visits' } })));
				}
				if (path.endsWith('/contract')) return Promise.resolve(jsonResponse(draftOf()));
				return respond(path);
			});
		}, accepted);

		await testPage.getByRole('button', { name: 'Change the Draft Contract' }).click();
		await testPage.getByLabelText('Scope of service').fill('Two visits');
		await testPage.getByRole('button', { name: 'Save Contract' }).click();

		await expect.element(testPage.getByRole('status').filter({ hasText: 'Contract saved.' })).toHaveFocus();
		expect(testPage.getByRole('button', { name: 'Save Contract' }).elements()).toHaveLength(0);
		const put = apiFetchWithSession.mock.calls.find(([, init]) => (init as RequestInit | undefined)?.method === 'PUT');
		const sent = JSON.parse((put![1] as RequestInit).body as string) as { values: Record<string, string> };
		expect(sent.values.scope_of_service).toBe('Two visits');
		expect(sent.values).not.toHaveProperty('price');
	});

	it('keeps the form open beside the refusal when a save is refused', async () => {
		await setupWithContract(() => {
			const respond = toApiResponder(fixture);
			apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
				if (path.endsWith('/contract') && init?.method === 'PUT') {
					return Promise.resolve(jsonResponse('contract is no longer a draft', 409));
				}
				if (path.endsWith('/contract')) return Promise.resolve(jsonResponse(draftOf()));
				return respond(path);
			});
		}, accepted);

		await testPage.getByRole('button', { name: 'Change the Draft Contract' }).click();
		await testPage.getByRole('button', { name: 'Save Contract' }).click();

		await expect.element(testPage.getByText('contract is no longer a draft')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Save Contract' })).toBeVisible();
	});

	it('sends the Contract and says so, focused', async () => {
		await setupWithContract(() => {
			const respond = toApiResponder(fixture);
			apiFetchWithSession.mockImplementation((path: string) => {
				if (path.endsWith('/contract/send')) return Promise.resolve(jsonResponse(draftOf({ status: 'sent' })));
				if (path.endsWith('/contract')) return Promise.resolve(jsonResponse(draftOf()));
				return respond(path);
			});
		}, accepted);

		await testPage.getByRole('button', { name: 'Send Contract' }).click();

		await expect.element(testPage.getByRole('status').filter({ hasText: 'Contract sent.' })).toHaveFocus();
		await expect.element(testPage.getByText('Sent, waiting for the Client to sign')).toBeVisible();
	});

	it('opens a new Draft at its merge fields', async () => {
		await setupWithContract(() => {
			mockNoContract(birthRateOnly);
			const fromFixture = apiFetchWithSession.getMockImplementation()!;
			apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) =>
				path.endsWith('/contract') && init?.method === 'POST'
					? Promise.resolve(jsonResponse(draftOf()))
					: fromFixture(path, init)
			);
		}, fixtureDetail, ownerSession);

		await testPage.getByRole('button', { name: 'Create a Draft Contract' }).click();

		await expect.element(testPage.getByRole('heading', { level: 3, name: 'Change the Draft Contract' })).toHaveFocus();
	});
});

// #1885: no Contract yet, and the rates the page reads to decide whether
// "Create Draft Contract" is live. `rates` is the `/rates` answer, or a
// Response for a refusal; every other path is the fixture's.
function mockNoContract(rates: Rate[] | Response) {
	respondFromFixture((path) => {
		if (path.endsWith('/contract')) return jsonResponse('not found', 404);
		if (path.endsWith('/rates')) return Array.isArray(rates) ? jsonResponse({ rates }) : rates;
		return;
	});
}

const ownerSession = sessionFor(['owner']);
const birthRateOnly: Rate[] = [
	{ kind: 'birth', amountCents: 150_000 },
	// eslint-disable-next-line unicorn/no-null -- a real API response's JSON null, for a kind with no rate set
	{ kind: 'postpartum', amountCents: null }
];
const noRates: Rate[] = [
	// eslint-disable-next-line unicorn/no-null -- a real API response's JSON null, for a kind with no rate set
	{ kind: 'birth', amountCents: null },
	// eslint-disable-next-line unicorn/no-null -- a real API response's JSON null, for a kind with no rate set
	{ kind: 'postpartum', amountCents: null }
];

describe('Create Draft Contract with no rate for the Engagement kind (#1885)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('withholds Create Draft Contract and links an Owner to the Rates page', async () => {
		await setupWithContract(() => mockNoContract(noRates), fixtureDetail, ownerSession);

		await expect
			.element(testPage.getByText('Your Practice has no birth rate yet, so you cannot create a Contract.'))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Set the birth rate' }))
			.toHaveAttribute('href', '/practices/practice-1/settings/rates');
		expect(testPage.getByRole('button', { name: 'Create a Draft Contract' }).elements()).toHaveLength(0);
	});

	it('tells a Doula who can set the rate, with no link to a page the Doula cannot use', async () => {
		await setupWithContract(() => mockNoContract(noRates), fixtureDetail, sessionFor(['doula']));

		await expect
			.element(
				testPage.getByText(
					'Your Practice has no birth rate yet, so you cannot create a Contract. Ask an owner or admin to set one.'
				)
			)
			.toBeVisible();
		expect(testPage.getByRole('link', { name: 'Set the birth rate' }).elements()).toHaveLength(0);
		expect(testPage.getByRole('button', { name: 'Create a Draft Contract' }).elements()).toHaveLength(0);
	});

	it('offers Create Draft Contract when the Engagement kind has a rate', async () => {
		await setupWithContract(() => mockNoContract(birthRateOnly), fixtureDetail, ownerSession);

		await expect.element(testPage.getByRole('button', { name: 'Create a Draft Contract' })).toBeVisible();
		expect(testPage.getByText(/no birth rate yet/i).elements()).toHaveLength(0);
	});

	it('moves the block when the kind changes to one with no rate, with no second read', async () => {
		await setupWithContract(
			() => {
				mockNoContract(birthRateOnly);
				const fromFixture = apiFetchWithSession.getMockImplementation()!;
				apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) =>
					path.endsWith('/kind') && init?.method === 'PUT'
						? Promise.resolve(jsonResponse({ kind: 'postpartum' }))
						: fromFixture(path, init)
				);
			},
			fixtureDetail,
			ownerSession
		);
		await expect.element(testPage.getByRole('button', { name: 'Create a Draft Contract' })).toBeVisible();

		await openChange('care');
		await testPage.getByRole('button', { name: 'Change to Postpartum' }).click();

		await expect
			.element(testPage.getByText('Your Practice has no postpartum rate yet, so you cannot create a Contract.'))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Set the postpartum rate' })).toBeVisible();
		expect(testPage.getByRole('button', { name: 'Create a Draft Contract' }).elements()).toHaveLength(0);
	});

	it('withholds Create Draft Contract and shows the refusal when the rates do not load', async () => {
		await setupWithContract(
			() => mockNoContract(jsonResponse('rates are down', 500)),
			fixtureDetail,
			ownerSession
		);

		await expect.element(testPage.getByText('rates are down')).toBeVisible();
		expect(testPage.getByRole('button', { name: 'Create a Draft Contract' }).elements()).toHaveLength(0);
	});
});

function contractRegion() {
	return testPage.getByRole('region', { name: 'Contract', exact: true });
}

// #2146: before there is a Contract, the page says what signing needs and
// where the money will appear.
describe('an Engagement with no Contract yet (#2146)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	const inviteBeforeContract =
		'Send a portal invite to this Client before sending the Contract — the Contract can only be viewed and signed once portal access exists.';

	it('shows the Invoices section with one line and no action', async () => {
		await setupWithContract(() => mockNoContract(birthRateOnly), fixtureDetail, ownerSession);

		const invoices = testPage.getByRole('region', { name: 'Invoices', exact: true });
		await expect.element(invoices.getByText('An Invoice is raised against a signed Contract.')).toBeVisible();
		expect(invoices.getByRole('button').elements()).toHaveLength(0);
		await expect.element(testPage.getByRole('link', { name: 'Invoices' }).first()).toHaveAttribute('href', '#invoices');
	});

	it('says a never-invited Client signs in the portal, and offers the invite beside Create', async () => {
		await setupWithContract(() => mockNoContract(birthRateOnly), fixtureDetail, ownerSession);

		await expect.element(contractRegion().getByText(inviteBeforeContract)).toBeVisible();
		await expect.element(contractRegion().getByRole('button', { name: 'Send portal invite' })).toBeVisible();
		await expect.element(contractRegion().getByRole('button', { name: 'Create a Draft Contract' })).toBeVisible();
	});

	it('says why the invite cannot be sent when the Client has no email address', async () => {
		await setupWithContract(
			() => mockNoContract(birthRateOnly),
			{ ...fixtureDetail, clientHasEmail: false },
			ownerSession
		);

		await expect.element(contractRegion().getByText(inviteBeforeContract)).toBeVisible();
		await expect
			.element(
				contractRegion().getByText('This Client has no email address on file. Add one before sending a portal invite.')
			)
			.toBeVisible();
		expect(contractRegion().getByRole('button', { name: 'Send portal invite' }).elements()).toHaveLength(0);
		await expect.element(contractRegion().getByRole('button', { name: 'Create a Draft Contract' })).toBeVisible();
	});

	it('names the acceptance signing waits on once an invite is out', async () => {
		await setupWithContract(
			() => mockNoContract(birthRateOnly),
			{ ...fixtureDetail, clientPortalInviteStatus: 'sent' },
			ownerSession
		);

		await expect
			.element(
				contractRegion().getByText(
					'The Contract can only be viewed and signed once this Client has accepted the portal invite.'
				)
			)
			.toBeVisible();
		await expect.element(contractRegion().getByRole('button', { name: 'Send portal invite' })).toBeVisible();
	});

	it('offers neither line nor invite to a Client who has a portal account', async () => {
		await setupWithContract(
			() => mockNoContract(birthRateOnly),
			{ ...fixtureDetail, clientPortalInviteStatus: 'accepted' },
			ownerSession
		);

		await expect.element(contractRegion().getByRole('button', { name: 'Create a Draft Contract' })).toBeVisible();
		expect(contractRegion().getByRole('button', { name: 'Send portal invite' }).elements()).toHaveLength(0);
		expect(contractRegion().getByText(/portal/).elements()).toHaveLength(0);
	});

	it('shows no action row with no email address and no rate', async () => {
		await setupWithContract(() => mockNoContract(noRates), { ...fixtureDetail, clientHasEmail: false }, ownerSession);

		await expect.element(contractRegion().getByText(inviteBeforeContract)).toBeVisible();
		expect(contractRegion().getByRole('button').elements()).toHaveLength(0);
	});
});

// #2146: the order a Doula works in, Contract before the Visits.
describe('the order of the sections (#2146)', () => {
	it('runs On call, Birth outcome, Contract, Visits, Care Plan, Birth Plan, Invoices, Offers, Messages, Activity', async () => {
		await setupWithContract(() => respondFromFixture(), fixtureDetail, ownerSession);
		// Offers is an Owner's or an Admin's, drawn once its read lands.
		await expect.element(testPage.getByRole('heading', { level: 2, name: 'Offers' })).toBeVisible();

		const headings = testPage
			.getByRole('heading', { level: 2 })
			.all()
			.map((heading) => heading.element().textContent?.trim());
		expect(headings).toEqual([
			'On call',
			'Birth outcome',
			'Contract',
			'Visits',
			'Care Plan',
			'Birth Plan',
			'Invoices',
			'Offers',
			'Messages',
			'Activity'
		]);
	});
});

/*
 * #885: the fixture's own Contract section, mounted the way the continuum
 * sweep mounts it -- the fixture's `respond` and the fixture's own
 * session, nothing overridden. The sweep asserts only that nothing needs
 * more room than it is given, so it stays green on a section that drew no
 * controls at all, which is what this route's fixture did while its
 * `/contract` answered a 404. This is the assertion that the subject the
 * sweep measures really does carry the section's controls.
 */
// At the conformance commitment itself (ADR-0024), not at a desk width:
// the criterion asks that these controls be there at 320px, and a screen
// that draws them only when it has room would satisfy a wider mount.
async function mountFixture() {
	await testPage.viewport(320, 900);
	respondFromFixture();
	await render(Page, { data: { ...fixtureDetail, session } });
}

describe("the fixture's own Contract section, as the sweep mounts it (#885)", () => {
	it("offers the section's Void and signed-PDF download controls", async () => {
		await mountFixture();

		await expect
			.element(testPage.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Void this Contract' })).toBeVisible();
	});

	it("reads back the Contract's price change and both of its void requests", async () => {
		await mountFixture();

		await expect.element(testPage.getByText('Price changed', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText(/^Void request declined:/)).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Decline' })).toBeVisible();
	});
});

// The signed Contract is the fixture's own now (#885), so nothing here
// describes one: only the PDF path, which the fixture cannot answer
// because each test needs a different response from it.
function mockSignedContract(pdfResponse: Response) {
	respondFromFixture((path) => (path.endsWith('/contract/pdf') ? pdfResponse : undefined));
}

async function setupWithRoles(roles: string[], pdfResponse: Response, isContractor = false) {
	await testPage.viewport(3200, 900);
	mockSignedContract(pdfResponse);
	await render(Page, {
		data: { ...fixtureDetail, session: sessionFor(roles, 'staff-1', isContractor) }
	});
}

describe('the Contract PDF download is gated on the page (#302, amended by #969)', () => {
	it('offers the download to an Owner', async () => {
		await setupWithRoles(['owner'], pdfBlobResponse());

		await expect
			.element(testPage.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.toBeVisible();
	});

	it('offers the download to an employed Doula (#969)', async () => {
		await setupWithRoles(['doula'], pdfBlobResponse());

		await expect
			.element(testPage.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.toBeVisible();
	});

	it('does not render the download for a contractor Doula, the role the endpoint still refuses', async () => {
		await setupWithRoles(['doula'], pdfBlobResponse(), true);

		await expect.element(testPage.getByText('Signed on Aug 14, 2026', { exact: true })).toBeVisible();
		expect(testPage.getByRole('button', { name: 'Download signed Contract (PDF)' }).elements()).toHaveLength(0);
	});

	it('fetches the pdf path when an Owner clicks download', async () => {
		await setupWithRoles(['owner'], pdfBlobResponse());
		await testPage.getByRole('button', { name: 'Download signed Contract (PDF)' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/practices/practice-1/engagements/engagement-1/contract/pdf');
	});

	it('reports a failed pdf fetch in words (#305 is what fails this locally/in CI)', async () => {
		await setupWithRoles(['owner'], new Response('signed PDF not found', { status: 500 }));
		await testPage.getByRole('button', { name: 'Download signed Contract (PDF)' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('signed PDF not found');
	});
});

// #1119: the page passes the Contract's own hasSignedPdf through to
// ContractStatus, so the control follows the PDF rather than the status.
// Voiding a Contract used to take the Practice's copy of it off the
// screen, while the endpoint went on serving it (#299).
describe('the Contract PDF download survives a void (#1119)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('offers the download on a voided Contract whose signed PDF still exists', async () => {
		await setupWithContract(
			() => mockContract({ ...fixtureContract, status: 'voided', hasSignedPdf: true }),
			fixtureDetail,
			session
		);

		await expect.element(testPage.getByText('Voided, no longer active')).toBeVisible();
		await expect
			.element(testPage.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.toBeVisible();
	});

	it('offers no download on a voided Contract that was never signed', async () => {
		await setupWithContract(
			() => mockContract({ ...fixtureContract, status: 'voided', hasSignedPdf: false }),
			fixtureDetail,
			session
		);

		await expect.element(testPage.getByText('Voided, no longer active')).toBeVisible();
		expect(testPage.getByRole('button', { name: 'Download signed Contract (PDF)' }).elements()).toHaveLength(0);
	});
});

// #1229: a fresh Draft (created after #72's recreate-after-void flow)
// beside an older signed-and-voided Contract used to read "Status: draft"
// next to the download control with nothing saying the PDF is the earlier
// Contract's, not the Draft's -- a reader could only conclude the Draft
// had somehow been signed.
describe('a recreated Draft names the PDF as a previous Contract\'s (#1229)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('names the earlier Contract on a fresh Draft beside an older signed-and-voided one', async () => {
		await setupWithContract(
			() => mockContract(draftOf({ hasSignedPdf: true })),
			fixtureDetail,
			session
		);

		await expect.element(testPage.getByText('Draft, not sent yet')).toBeVisible();
		await expect
			.element(testPage.getByText(PREVIOUS_CONTRACT_PDF_NOTICE))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.toBeVisible();
	});

	it('adds no notice on a plain Draft with no signed PDF anywhere on the Engagement', async () => {
		await setupWithContract(() => mockContract(draftOf()), fixtureDetail, session);

		await expect.element(testPage.getByText('Draft, not sent yet')).toBeVisible();
		await expect
			.element(testPage.getByText(PREVIOUS_CONTRACT_PDF_NOTICE))
			.not.toBeInTheDocument();
	});
});

/*
 * #2004: the Care Plan and the Birth Plan are facts first -- how far each
 * is answered and, for the Birth Plan, whether the Client has read it --
 * with one action that opens its questions, and the Birth Plan's own
 * address (#280) beside it, where printing and the PDF live. A save
 * returns to the facts with a status that takes focus.
 */
const planFields = [
	{ id: 'header', type: 'section_header' as const, label: 'Labor', order: 0 },
	{ id: 'music', type: 'short_text' as const, label: 'Music', order: 1 },
	{ id: 'lights', type: 'short_text' as const, label: 'Lighting', order: 2 }
];

function planOf(planType: string, overrides: Record<string, unknown> = {}) {
	return {
		engagementId: fixture.params.engagementId,
		planType,
		fields: planFields,
		answers: { music: 'Quiet' },
		...overrides
	};
}

// Each Plan answered from `plans`, a Plan type with no entry a 404 (not
// started); `onPut`, where given, answers the save.
async function setupPlans(
	plans: Partial<Record<'care_plan' | 'birth_plan', ReturnType<typeof planOf>>>,
	onPut?: (planType: string, body: { answers: Record<string, unknown> }) => Response,
	onPost?: (planType: string) => Response
) {
	await testPage.viewport(3200, 900);
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		const planType = /\/plans\/(care_plan|birth_plan)$/.exec(path)?.[1] as 'care_plan' | 'birth_plan' | undefined;
		if (onPut && planType && init?.method === 'PUT') {
			return Promise.resolve(onPut(planType, JSON.parse(init.body as string)));
		}
		if (onPost && planType && init?.method === 'POST') return Promise.resolve(onPost(planType));
		if (planType) {
			const plan = plans[planType];
			return Promise.resolve(plan ? jsonResponse(plan) : jsonResponse('not found', 404));
		}
		return respond(path);
	});
	await render(Page, { data: { ...fixtureDetail, session: sessionFor() } });
}

function planSection(heading: string) {
	return testPage.getByRole('region', { name: heading, exact: true });
}

describe('the Care Plan and Birth Plan sections (#2004)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('shows how far a Plan is answered, and no question until asked', async () => {
		await setupPlans({ care_plan: planOf('care_plan') });

		await expect.element(planSection('Care Plan').getByText('1 of 2 questions answered')).toBeVisible();
		expect(testPage.getByLabelText('Music').elements()).toHaveLength(0);
		await expect.element(planSection('Care Plan').getByRole('button', { name: 'Change the Care Plan' })).toBeVisible();
	});

	it("says by the Client's name whether the Birth Plan has been read", async () => {
		await setupPlans({
			birth_plan: planOf('birth_plan'),
			care_plan: planOf('care_plan')
		});

		await expect
			.element(planSection('Birth Plan').getByText(`${fixtureDetail.clientName} has not read this yet`))
			.toBeVisible();
		expect(planSection('Care Plan').getByText(/has not read this yet/).elements()).toHaveLength(0);
	});

	it('says when the Client read the Birth Plan', async () => {
		await setupPlans({ birth_plan: planOf('birth_plan', { clientAcknowledgedAt: '2026-10-03T15:00:00Z' }) });

		await expect
			.element(planSection('Birth Plan').getByText(`${fixtureDetail.clientName} read this on Oct 3, 2026`))
			.toBeVisible();
	});

	it('links the Birth Plan to its own page, for printing it for the hospital (#280)', async () => {
		await setupPlans({ birth_plan: planOf('birth_plan') });

		await expect
			.element(testPage.getByRole('link', { name: 'Print for the hospital' }))
			.toHaveAttribute(
				'href',
				`/practices/${fixture.params.practiceId}/engagements/${fixture.params.engagementId}/birth-plan`
			);
		expect(testPage.getByRole('button', { name: 'Download Birth Plan (PDF)' }).elements()).toHaveLength(0);
	});

	it('says a Plan is not started, and starts it at its questions', async () => {
		await setupPlans({}, undefined, (planType) => jsonResponse(planOf(planType, { answers: {} })));

		await expect.element(planSection('Care Plan').getByText('The Care Plan is not started yet.')).toBeVisible();
		await planSection('Care Plan').getByRole('button', { name: 'Start the Care Plan' }).click();

		await expect.element(testPage.getByRole('heading', { level: 3, name: 'Change the Care Plan' })).toHaveFocus();
		// #2106: the Plan's own groups sit one level under the form's heading.
		await expect.element(testPage.getByRole('heading', { level: 4, name: 'Labor' })).toBeVisible();
		await expect.element(testPage.getByLabelText('Music')).toHaveValue('');
	});

	// #2133: a Plan started and left unsaved still says who started it.
	it('says who started a Plan, from the start itself, until its first save', async () => {
		const started = { at: '2026-10-26T15:00:00Z', by: 'Renata Alvarez' };
		await setupPlans({}, undefined, (planType) => jsonResponse(planOf(planType, { answers: {}, started })));

		await planSection('Care Plan').getByRole('button', { name: 'Start the Care Plan' }).click();
		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(planSection('Care Plan').getByText('Started', { exact: true })).toBeVisible();
		await expect.element(planSection('Care Plan').getByText('Oct 26, 2026 by Renata Alvarez')).toBeVisible();
		expect(planSection('Care Plan').getByText('Last edited').elements()).toHaveLength(0);
	});

	it('names the last save, not the start, once a Plan is saved', async () => {
		await setupPlans({
			care_plan: planOf('care_plan', {
				started: { at: '2026-10-20T15:00:00Z', by: 'Renata Alvarez' },
				lastEdited: { at: '2026-10-24T15:00:00Z', by: 'Priya Raman' }
			})
		});

		await expect.element(planSection('Care Plan').getByText('Oct 24, 2026 by Priya Raman')).toBeVisible();
		expect(planSection('Care Plan').getByText('Started', { exact: true }).elements()).toHaveLength(0);
	});

	it('saves what was typed, returns to the facts, and says so, focused', async () => {
		let saved: Record<string, unknown> = {};
		await setupPlans({ birth_plan: planOf('birth_plan', { clientAcknowledgedAt: '2026-10-03T15:00:00Z' }) }, (planType, body) => {
			saved = body.answers;
			return jsonResponse(planOf(planType, { answers: body.answers }));
		});

		await planSection('Birth Plan').getByRole('button', { name: 'Change the Birth Plan' }).click();
		await testPage.getByLabelText('Lighting').fill('Low');
		await testPage.getByRole('button', { name: 'Save Birth Plan' }).click();

		await expect.element(testPage.getByRole('status').filter({ hasText: 'Birth Plan saved.' })).toHaveFocus();
		expect(saved).toEqual({ music: 'Quiet', lights: 'Low' });
		await expect.element(planSection('Birth Plan').getByText('Complete')).toBeVisible();
		// The save's answer carries no acknowledgment: a Staff edit clears it.
		await expect
			.element(planSection('Birth Plan').getByText(`${fixtureDetail.clientName} has not read this yet`))
			.toBeVisible();
	});

	it('keeps the Plan as it was on Cancel', async () => {
		await setupPlans({ care_plan: planOf('care_plan') });

		await planSection('Care Plan').getByRole('button', { name: 'Change the Care Plan' }).click();
		await testPage.getByLabelText('Lighting').fill('Low');
		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(planSection('Care Plan').getByText('1 of 2 questions answered')).toBeVisible();
		// #2103: back on the opener, not on the page.
		await expect.element(planSection('Care Plan').getByRole('button', { name: 'Change the Care Plan' })).toHaveFocus();
		await planSection('Care Plan').getByRole('button', { name: 'Change the Care Plan' }).click();
		await expect.element(testPage.getByLabelText('Lighting')).toHaveValue('');
	});

	it('keeps the form open beside the refusal when a save is refused', async () => {
		await setupPlans({ care_plan: planOf('care_plan') }, () => jsonResponse('answers do not match the plan', 400));

		await planSection('Care Plan').getByRole('button', { name: 'Change the Care Plan' }).click();
		await testPage.getByRole('button', { name: 'Save Care Plan' }).click();

		await expect.element(testPage.getByText('answers do not match the plan')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Save Care Plan' })).toBeVisible();
	});
});

describe("the link to the Client's details journey (#1610)", () => {
	const clientPath = `/api/practices/${fixture.params.practiceId}/clients/${fixtureDetail.clientId}`;

	it('offers to add her details, returning here', async () => {
		await renderWithFixtureResponder();

		await expect
			.element(testPage.getByRole('link', { name: `Add ${fixtureDetail.clientName}'s details`, exact: true }))
			.toHaveAttribute(
				'href',
				`/practices/${fixture.params.practiceId}/clients/${fixtureDetail.clientId}/details?engagement=${fixture.params.engagementId}`
			);
	});

	it('offers none for an erased record', async () => {
		await renderWithFixtureResponder((path) =>
			path === clientPath
				? Promise.resolve(jsonResponse({ id: fixtureDetail.clientId, erasedAt: '2026-03-01T00:00:00Z' }))
				: undefined
		);

		await expect.element(testPage.getByRole('link', { name: 'View Client' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: /details$/ })).not.toBeInTheDocument();
	});

	// Best-effort: a lost read costs the link, never the page.
	it('offers none when her record could not be read', async () => {
		await renderWithFixtureResponder((path) =>
			path === clientPath ? Promise.resolve(jsonResponse('forbidden', 403)) : undefined
		);

		await expect.element(testPage.getByRole('link', { name: 'View Client' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: /details$/ })).not.toBeInTheDocument();
	});
});

/*
 * #268/#274: choosing who a Visit is for is picking a person by name, at
 * create and at reassign alike, and the control is only ever drawn for a
 * reader whose role can actually complete it.
 *
 * The roster read is what decides that, so these tests drive it directly:
 * a 200 is an Owner or Admin, a 403 is the plain Doula the endpoint
 * refuses. The BFF refuses the write on its own either way -- see
 * api/internal/visit/assign_test.go, which asserts the refusal by role
 * with no screen involved at all.
 */
describe('choosing who a Visit is for (#268, #274)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('offers the Practice\'s Doulas by name when adding a Visit', async () => {
		await setupWithRoster();
		await openAddVisit();

		const picker = testPage.getByLabelText('Who is this Visit for?');
		await expect.element(picker).toBeVisible();
		// The option is asserted by its presence in the picker's own option
		// list, not with `toBeVisible`: an <option> inside a closed <select>
		// is not visible in the sense the matcher means, and this repo's
		// `toBeVisible`-over-`toBeInTheDocument` rule is about a positive
		// case that *can* be seen. What matters here is that her name is a
		// choice the picker offers.
		expect(
			picker.getByRole('option', { name: 'Kanyakumari Balasubramanian' }).elements()
		).toHaveLength(1);
	});

	// The bookkeeper holds the Admin role and no Doula role, so she can
	// never be named on a Visit -- offering her would be offering a choice
	// the BFF refuses.
	it('leaves a Staff member who holds no Doula role out of the picker', async () => {
		await setupWithRoster();
		await openAddVisit();

		await expect.element(testPage.getByLabelText('Who is this Visit for?')).toBeVisible();
		expect(
			testPage.getByRole('option', { name: 'Winifred Abernathy-Castellano' }).elements()
		).toHaveLength(0);
	});

	it('offers a select, not a free-text staff id, for reassigning a Visit', async () => {
		await setupWithRoster();
		await changeVisit(0);

		const picker = testPage.getByLabelText('Who is this Visit for?');
		await expect.element(picker).toBeVisible();
		expect(picker.element().tagName).toBe('SELECT');
	});

	// The defect this replaces: no screen in the product prints a staff
	// id, so a field asking for one could not be filled in.
	it('offers no staff-id text field anywhere on the page', async () => {
		await setupWithRoster();
		await openAddVisit();

		await expect.element(testPage.getByLabelText('Who is this Visit for?')).toBeVisible();
		expect(testPage.getByLabelText(/staff id/i).elements()).toHaveLength(0);
	});

	// #274's own criterion: absent, not broken and not erroring. A plain
	// Doula may not read the roster (ADR-0008), so she is offered no
	// assign control at all -- and still logs her own Visit, which needs
	// no roster.
	it('leaves both pickers out for a reader the roster read refuses, and still lets her log her own Visit', async () => {
		await setupWithRoster(jsonResponse('not permitted to read this', 403), ['doula']);

		await openAddVisit();
		await expect.element(testPage.getByRole('button', { name: 'Save Visit' })).toBeVisible();
		expect(testPage.getByLabelText('Who is this Visit for?').elements()).toHaveLength(0);
		await changeVisit(0);
		await expect.element(testPage.getByLabelText('Notes')).toBeVisible();
		expect(testPage.getByLabelText('Who is this Visit for?').elements()).toHaveLength(0);
		expect(testPage.getByText('There is nobody else to reassign this Visit to.').elements()).toHaveLength(0);
		expect(testPage.getByRole('alert').elements()).toHaveLength(0);
	});

	// The other side of that criterion. "Absent, not erroring" is the
	// answer to a *refusal*; a roster read that fell over is an outage, and
	// answering it with the same silence hands an Owner a screen whose
	// Add-a-Visit form and both pickers have vanished with no reason given.
	// She is told, and the pickers still stay out because there is nobody
	// to put in them.
	it('says so when the roster read fails, rather than hiding the pickers silently', async () => {
		await setupWithRoster(jsonResponse('the roster is unavailable', 500), ['owner']);

		await expect.element(testPage.getByRole('alert').first()).toBeVisible();
		await expect.element(testPage.getByText('the roster is unavailable')).toBeVisible();
		expect(testPage.getByLabelText('Who is this Visit for?').elements()).toHaveLength(0);
	});

	// An Owner or Admin who is not a Doula has no self to log, so the
	// picker is the only way in for her -- and it is there.
	it('offers the picker to an Admin who holds no Doula role', async () => {
		await setupWithRoster(undefined, ['admin']);
		await openAddVisit();

		await expect.element(testPage.getByLabelText('Who is this Visit for?')).toBeVisible();
	});

	/*
	 * #909's three-way split, and the reassign picker's own rule. The
	 * fixture roster's first entry -- staff-1, Anne-Marie
	 * Ochieng-Whitfield -- is the only one holding both Owner and Doula,
	 * so she is the Doula-Owner this ticket is about.
	 */
	describe('a Doula who owns her Practice, logging her own Visit (#909)', () => {
		it('opens the picker on the caller, marked as the signed-in person and first in the list', async () => {
			await setupWithRoster();
			await openAddVisit();

			const picker = testPage.getByLabelText('Who is this Visit for?');
			await expect.element(picker).toHaveValue('staff-1');
			// First non-placeholder option, so a standing answer is visible
			// as an answer rather than as an arbitrary top of the list.
			const options = [...picker.element().querySelectorAll('option')].map((o) => o.textContent);
			expect(options[1]).toBe('Anne-Marie Ochieng-Whitfield (you)');
		});

		it('logs the Visit for the caller without her touching the picker', async () => {
			await setupWithRoster();
			await openAddVisit();
			await expect.element(testPage.getByLabelText('Who is this Visit for?')).toHaveValue('staff-1');
			await testPage.getByRole('button', { name: 'Save Visit' }).click();

			expect(apiFetchWithSession).toHaveBeenCalledWith(
				'/api/practices/practice-1/engagements/engagement-1/visits',
				expect.objectContaining({
					body: JSON.stringify({ scheduledAt: undefined, staffId: 'staff-1' })
				})
			);
			// Her own Visit is hers, however the request named her (#1999).
			await expect.element(testPage.getByText('Your Visit is added.')).toHaveFocus();
		});

		// The other two thirds of the split. An Admin who is not on the
		// Doula roster has no self to fall back on, so she is asked, and
		// nothing is chosen for her.
		it('opens on the placeholder for an Admin who holds no Doula role', async () => {
			await setupWithRoster(undefined, ['admin'], 'staff-bookkeeper');
			await openAddVisit();

			await expect.element(testPage.getByLabelText('Who is this Visit for?')).toHaveValue('');
		});

		// #1228: the placeholder above is empty, not pre-selected, so this
		// picker carried no browser refusal to lose in the first place --
		// only the required Selects in this file did. Refused here rather
		// than sent as an empty staffId for the BFF to 400 on.
		it('refuses an Add a Visit submit left on the placeholder, without calling the API', async () => {
			await setupWithRoster(undefined, ['admin'], 'staff-bookkeeper');
			await openAddVisit();
			await expect.element(testPage.getByLabelText('Who is this Visit for?')).toHaveValue('');
			apiFetchWithSession.mockClear();

			await testPage.getByRole('button', { name: 'Save Visit' }).click();

			expect(apiFetchWithSession).not.toHaveBeenCalled();
			await expect.element(testPage.getByText('Select who this Visit is for').first()).toBeVisible();
		});

		// And a plain Doula is never asked at all -- the roster read
		// refuses her, and an absent assignee already means her.
		it('asks a plain Doula nothing, and still lets her log her own Visit', async () => {
			await setupWithRoster(jsonResponse('not permitted to read this', 403), ['doula'], 'staff-2');

			await openAddVisit();
			await expect.element(testPage.getByRole('button', { name: 'Save Visit' })).toBeVisible();
			expect(testPage.getByLabelText('Who is this Visit for?').elements()).toHaveLength(0);
		});

		// The standing answer is a default, not a lock: the same Owner also
		// books her colleagues' Visits, and naming one has to stick.
		it('keeps the colleague she names, rather than reverting to her own name', async () => {
			await setupWithRoster();
			await openAddVisit();
			const picker = testPage.getByLabelText('Who is this Visit for?');
			await expect.element(picker).toHaveValue('staff-1');

			await picker.selectOptions('Jordan Reyes');

			await expect.element(picker).toHaveValue('staff-2');
		});

		// visit-1 is assigned to staff-1, visit-2 to staff-2 -- offering
		// either her own Visit back to her is a move that did not happen.
		it("leaves a Visit's current assignee out of its own reassign picker", async () => {
			await setupWithRoster();
			const picker = testPage.getByLabelText('Who is this Visit for?');

			// #1999: she is kept by the first option, which opens chosen and
			// sends nothing, rather than offered as a move.
			await changeVisit(0);
			await expect.element(picker).toHaveValue('');
			expect(picker.getByRole('option', { name: 'Keep Anne-Marie Ochieng-Whitfield' }).elements()).toHaveLength(1);
			expect(
				picker.getByRole('option', { name: 'Anne-Marie Ochieng-Whitfield (you)' }).elements()
			).toHaveLength(0);
			// She is still offered on a Visit that is not already hers.
			await changeVisit(1);
			await expect
				.poll(() => picker.getByRole('option', { name: 'Anne-Marie Ochieng-Whitfield (you)' }).elements())
				.toHaveLength(1);
			expect(picker.getByRole('option', { name: 'Jordan Reyes', exact: true }).elements()).toHaveLength(0);
		});

		// ADR-0024's floor, on the real route rather than on a style-guide
		// demo: the fixture roster is fourteen Doulas with long real-world
		// names, and the marked option is the longest of the lot.
		it('never scrolls the document sideways at 320px with a fourteen-name roster', async () => {
			await setupWithRoster();
			await testPage.viewport(320, 800);
			await openAddVisit();

			await expect.element(testPage.getByLabelText('Who is this Visit for?')).toBeVisible();
			expect(document.documentElement.scrollWidth).toBe(document.documentElement.clientWidth);
		});

		// A Practice whose only Doula is the person already holding the
		// Visit: an empty picker with a placeholder and no explanation is
		// the defect, so the control says what is true instead.
		it('says there is nobody to reassign to rather than showing an empty picker', async () => {
			const soleDoula = {
				items: [
					{
						staffId: 'staff-1',
						name: 'Anne-Marie Ochieng-Whitfield',
						employmentType: 'employee',
						nameable: true
					}
				]
			};
			await setupWithRoster(jsonResponse(soleDoula));
			const nobodyElse = testPage.getByText('There is nobody else to reassign this Visit to.');

			// #1999: said once, in the open change, where the question would
			// be -- never as a line in every row.
			await expect.element(visitsTable().getByRole('button', { name: 'Change' }).first()).toBeVisible();
			expect(nobodyElse.elements()).toHaveLength(0);
			await changeVisit(0);
			await expect.element(nobodyElse).toBeVisible();
			expect(testPage.getByLabelText('Who is this Visit for?').elements()).toHaveLength(0);
			// visit-2 is Jordan Reyes's, and she is not on this roster, so
			// its own change still offers the sole Doula.
			await changeVisit(1);
			await expect.element(testPage.getByLabelText('Who is this Visit for?')).toBeVisible();
			expect(nobodyElse.elements()).toHaveLength(0);
		});
	});

	// A reader who can neither pick a colleague nor log her own Visit is
	// offered no create control at all, rather than a button that 403s.
	it('offers no add control at all to a reader who can do neither', async () => {
		await setupWithRoster(jsonResponse('not permitted to read this', 403), ['admin']);

		await expect.element(testPage.getByRole('heading', { name: 'Visits' })).toBeVisible();
		expect(testPage.getByRole('button', { name: 'Add a Visit' }).elements()).toHaveLength(0);
	});

	it('sends the chosen colleague on the create request', async () => {
		await setupWithRoster();
		await openAddVisit();
		await testPage.getByLabelText('Who is this Visit for?').selectOptions('Jordan Reyes');
		await testPage.getByRole('button', { name: 'Save Visit' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/engagement-1/visits',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ scheduledAt: undefined, staffId: 'staff-2' })
			})
		);
	});

	it('sends the chosen colleague on the reassign request, and says whose the Visit is now', async () => {
		await setupWithRoster();
		await changeVisit(0);
		await testPage.getByLabelText('Who is this Visit for?').selectOptions('Jordan Reyes');
		await testPage.getByRole('button', { name: 'Save changes' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/engagement-1/visits/visit-1',
			expect.objectContaining({ method: 'PATCH', body: JSON.stringify({ staffId: 'staff-2' }) })
		);
		await expect.element(testPage.getByText('The Visit for Jordan Reyes is saved.')).toBeVisible();
	});
});

// #2004: the Offers form opens only when asked, from its one opener.
async function openOfferForm() {
	await testPage.getByRole('button', { name: 'Offer this work' }).click();
}

// #1432: the Offers form reads the same roster the Visit pickers do, and
// "nobody is there" and "we could not find out" are two facts. A failed
// read must not be told to an Owner as a Practice with no Doula on it.
describe('what the Offers form says when the roster holds nobody (#1432)', () => {
	it('does not tell an Owner that there is no one to offer the work to when the roster read fails', async () => {
		await setupWithRoster(jsonResponse('the roster is unavailable', 500), ['owner']);
		await openOfferForm();

		await expect.element(testPage.getByText(/^We could not load who is at this Practice/)).toBeVisible();
		expect(testPage.getByText(/^There is no one at this Practice to offer this work to/).elements()).toHaveLength(0);
	});

	it('says that there is no one to offer the work to when the roster holds nobody, and asks for an email address', async () => {
		await setupWithRoster(jsonResponse({ items: [] }), ['owner']);
		await openOfferForm();

		await expect.element(testPage.getByText(/^There is no one at this Practice to offer this work to/)).toBeVisible();
		expect(testPage.getByRole('group', { name: 'Offer this work to' }).elements()).toHaveLength(0);
		await expect.element(testPage.getByLabelText('Email address')).toBeVisible();
	});
});

// #1598: nobody offers herself work. The Offers form reads the roster the
// Visit pickers read, minus the person at the screen; the pickers keep her.
describe('the Offers form does not list the person at the screen (#1598)', () => {
	it('leaves the reader out of the Doula pick, and keeps her in the Visit picker', async () => {
		// The fixture's reader is `staff-1`, the roster's first Doula.
		await setupWithRoster();
		await openOfferForm();
		// *Offer this work to* opens with nothing chosen (#1914).
		await testPage.getByLabelText('Someone already at this Practice').click();

		const doulaPick = testPage.getByRole('group', { name: 'Doula', exact: true });
		await expect.element(doulaPick.getByLabelText('Jordan Reyes')).toBeVisible();
		expect(doulaPick.getByLabelText('Anne-Marie Ochieng-Whitfield').elements()).toHaveLength(0);

		await openAddVisit();
		const visitPicker = testPage.getByLabelText('Who is this Visit for?');
		expect(visitPicker.getByRole('option', { name: 'Anne-Marie Ochieng-Whitfield (you)' }).elements()).toHaveLength(1);
	});

	it('says there is no one to offer the work to when the reader is the only Doula at the Practice', async () => {
		await setupWithRoster(
			jsonResponse({
				items: [{ staffId: 'staff-1', name: 'Anne-Marie Ochieng-Whitfield', employmentType: 'employee', nameable: true }]
			})
		);
		await openOfferForm();

		await expect.element(testPage.getByText(/^There is no one at this Practice to offer this work to/)).toBeVisible();
		expect(testPage.getByRole('group', { name: 'Offer this work to' }).elements()).toHaveLength(0);
		await expect.element(testPage.getByLabelText('Email address')).toBeVisible();
		// The same roster still gives her own Visit its standing answer.
		await openAddVisit();
		await expect.element(testPage.getByLabelText('Who is this Visit for?')).toHaveValue('staff-1');
	});

	it('still says the roster could not be read, and not that nobody is there, when the read fails', async () => {
		await setupWithRoster(jsonResponse('the roster is unavailable', 500));
		await openOfferForm();

		await expect.element(testPage.getByText(/^We could not load who is at this Practice/)).toBeVisible();
		expect(testPage.getByText(/^There is no one at this Practice to offer this work to/).elements()).toHaveLength(0);
	});
});

// #1598: who is on the Engagement, and the one press that puts a Doula
// on it. The BFF decides who is offered the control
// (Detail.canAttachSelf); this page draws what it is told. An employee
// Doula is offered it, and so is a Doula who holds Owner or Admin
// whatever her Employment type is (#1625).
const attachSelfPath = '/api/practices/practice-1/engagements/engagement-1/attachments/me';
const putMeOnIt = () => testPage.getByRole('button', { name: 'Put me on this Engagement' });

async function setupWhoIsOn(detail: Detail, attachResponse?: () => Response) {
	await testPage.viewport(3200, 900);
	apiFetchWithSession.mockImplementation((path: string) =>
		Promise.resolve(
			attachResponse && path.endsWith('/attachments/me') ? attachResponse() : jsonResponse('not available', 403)
		)
	);
	await render(Page, {
		data: { ...detail, session: sessionFor(['owner', 'doula']) }
	});
}

describe('who is on the Engagement, and putting yourself on it (#1598)', () => {
	const nobodyOnIt: Detail = { ...fixtureDetail, doulas: [], canAttachSelf: true };

	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('names each Doula on the Engagement in the summary, and offers no control to a reader the BFF does not offer it to', async () => {
		await setupWhoIsOn(fixtureDetail);

		await expect.element(testPage.getByText('Doulas', { exact: true })).toBeVisible();
		await expect
			.element(testPage.getByText('Persephone Vandermeulen-Achterberg, CD(DONA), Bo Ng', { exact: true }))
			.toBeVisible();
		expect(putMeOnIt().elements()).toHaveLength(0);
		expect(changeButton('Doula')).toBeUndefined();
	});

	it('says "No Doula yet" and offers the control where the BFF says the press would be accepted', async () => {
		await setupWhoIsOn(nobodyOnIt);

		await expect.element(testPage.getByText('No Doula yet', { exact: true })).toBeVisible();
		await openChange('Doula');
		await expect.element(putMeOnIt()).toBeVisible();
	});

	// #1625: the fixture's own reader, an Owner who is a contractor Doula.
	// The BFF offers her the control, and the page holds no rule of its
	// own that takes it away for her Employment type.
	it('offers the control to an Owner who is a contractor Doula, where the BFF says the press would be accepted', async () => {
		await testPage.viewport(3200, 900);
		apiFetchWithSession.mockImplementation(() => Promise.resolve(jsonResponse('not available', 403)));
		expect(session).toMatchObject({ roles: ['owner', 'doula'], isContractor: true });
		await render(Page, { data: { ...nobodyOnIt, session } });

		await expect.element(testPage.getByText('No Doula yet', { exact: true })).toBeVisible();
		await openChange('Doula');
		await expect.element(putMeOnIt()).toBeVisible();
	});

	// A contractor Doula who holds neither Owner nor Admin, and an Owner or
	// an Admin who holds no Doula role, read `canAttachSelf: false` from
	// the BFF. The control is drawn off
	// that one field, so nobody is on it and there is still no button.
	it('offers no control with nobody on the Engagement, where the BFF says the reader may not attach herself', async () => {
		await setupWhoIsOn({ ...nobodyOnIt, canAttachSelf: false });

		await expect.element(testPage.getByText('No Doula yet', { exact: true })).toBeVisible();
		expect(putMeOnIt().elements()).toHaveLength(0);
		expect(changeButton('Doula')).toBeUndefined();
	});

	it('puts the reader on the Engagement with one press, names her, says so, and takes the control away', async () => {
		await setupWhoIsOn(nobodyOnIt, () =>
			jsonResponse({
				engagementId: 'engagement-1',
				doulas: [{ staffId: 'staff-1', name: 'Anne-Marie Ochieng-Whitfield' }],
				canAttachSelf: false
			})
		);
		await openChange('Doula');
		await expect.element(putMeOnIt()).toBeVisible();
		apiFetchWithSession.mockClear();

		await putMeOnIt().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(attachSelfPath, { method: 'PUT' });
		// The result takes focus: the button she pressed is gone.
		await expect.element(testPage.getByText('You are on this Engagement.')).toHaveFocus();
		expect(testPage.getByText('No Doula yet', { exact: true }).elements()).toHaveLength(0);
		expect(putMeOnIt().elements()).toHaveLength(0);
		expect(changeButton('Doula')).toBeUndefined();
		// The ledger has a new entry and the birth can now have an On-call
		// window, so both sections are read again.
		await vi.waitFor(() => {
			expect(apiFetchWithSession).toHaveBeenCalledWith('/api/practices/practice-1/engagements/engagement-1/on-call');
			expect(apiFetchWithSession).toHaveBeenCalledWith(
				'/api/practices/practice-1/engagements/engagement-1/activity'
			);
		});
	});

	it('shows the refusal and keeps the control when the BFF refuses the press', async () => {
		await setupWhoIsOn(nobodyOnIt, () => jsonResponse('This Engagement has completed, so nobody can be put on it.', 409));
		await openChange('Doula');
		await expect.element(putMeOnIt()).toBeVisible();

		await putMeOnIt().click();

		await expect.element(testPage.getByText('This Engagement has completed, so nobody can be put on it.')).toBeVisible();
		await expect.element(putMeOnIt()).toBeVisible();
		await expect.element(testPage.getByText('No Doula yet', { exact: true })).toBeVisible();
	});

	// A Visit that names an employee grants her an Attachment, so the
	// Engagement is read again after the write: the reader who logs her own
	// Visit is on the Engagement, and the control goes.
	it('reads who is on the Engagement again after a Visit is added', async () => {
		await testPage.viewport(3200, 900);
		const respond = toApiResponder(fixture);
		let isVisitAdded = false;
		apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
			if (path.endsWith('/visits') && init?.method === 'POST') {
				isVisitAdded = true;
				return Promise.resolve(jsonResponse({ visitId: 'visit-3' }, 201));
			}
			if (/\/engagements\/engagement-1$/.test(path)) {
				return Promise.resolve(
					jsonResponse(
						isVisitAdded
							? {
									...nobodyOnIt,
									doulas: [{ staffId: 'staff-1', name: 'Anne-Marie Ochieng-Whitfield' }],
									canAttachSelf: false
								}
							: nobodyOnIt
					)
				);
			}
			return respond(path);
		});
		await render(Page, {
			data: { ...nobodyOnIt, session: sessionFor(['owner', 'doula']) }
		});
		await openChange('Doula');
		await expect.element(putMeOnIt()).toBeVisible();
		await openAddVisit();
		await expect.element(testPage.getByLabelText('Who is this Visit for?')).toHaveValue('staff-1');

		await testPage.getByRole('button', { name: 'Save Visit' }).click();

		await expect.element(putMeOnIt()).not.toBeInTheDocument();
		expect(testPage.getByText('No Doula yet', { exact: true }).elements()).toHaveLength(0);
	});
});

/*
 * #911: the pickers were drawn from the Practice-wide roster, which
 * cannot know anything about *this* Engagement -- so an Owner or Admin
 * saw a contractor's name, picked it, and met a `400` she had no way to
 * anticipate. The fixture's roster makes every third member a contractor
 * and attaches none of them here, so `staff-4` is exactly that person.
 * The BFF still refuses the write whatever this screen drew: see
 * api/internal/visit/nameable_test.go.
 */
describe('who can be named on a Visit at this Engagement (#911)', () => {
	// The unattached contractor the fixture's own roster produces.
	const unattached = 'Guadalupe Fernández-Castellanos';
	const unattachedOption = `${unattached} (cannot be named yet)`;

	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	// She is marked, not dropped: a contractor who has not accepted an
	// Offer is precisely the person an Admin is trying to get onto the
	// birth, and losing her name would read as "she is not on the roster".
	// And she stays choosable -- a bare `disabled` option carries no
	// reason, so choosing her is what produces one.
	it('lists an unattached contractor by name, marked, and still choosable', async () => {
		await setupWithRoster();
		await openAddVisit();

		const picker = testPage.getByLabelText('Who is this Visit for?');
		await expect.element(picker).toBeVisible();
		const option = picker.getByRole('option', { name: unattachedOption });
		expect(option.elements()).toHaveLength(1);
		expect(option.element().hasAttribute('disabled')).toBe(false);
	});

	// One derived list, both pickers -- two independently filtered lists on
	// one page is the failure this ticket exists to close.
	it('marks her the same way in the reassign picker', async () => {
		await setupWithRoster();
		await changeVisit(0);

		const picker = testPage.getByLabelText('Who is this Visit for?');
		await expect.element(picker).toBeVisible();
		expect(picker.getByRole('option', { name: unattachedOption }).elements()).toHaveLength(1);
	});

	// What the marker means and what changes it, in text, before anybody
	// has to choose a name to find out -- not conveyed by color and not by
	// a bare `disabled` attribute.
	it('says in text what the marker means and what makes her nameable', async () => {
		await setupWithRoster();
		await openAddVisit();

		await expect
			.element(testPage.getByText(/an Offer, then name .+ on a Visit/).first())
			.toBeVisible();
	});

	// #1228: ErrorSummary's own contract ("a clean form never reaches
	// here... this cannot steal focus from someone who has not submitted")
	// assumes its errors only ever arrive from a submit. #911's own live
	// block does not -- it fires the instant an ineligible name is picked
	// -- so the summary has to wait for an actual submit attempt even
	// though the inline field error (unchanged since #911) shows at once.
	it('marks an ineligible pick on the field at once, without mounting the summary or moving focus', async () => {
		await setupWithRoster();
		await openAddVisit();
		const picker = testPage.getByLabelText('Who is this Visit for?');
		await expect.element(picker).toBeVisible();

		await picker.selectOptions(unattachedOption);

		await expect.element(picker).toHaveAttribute('aria-invalid', 'true');
		await expect.element(testPage.getByRole('heading', { name: 'There is a problem' })).not.toBeInTheDocument();
	});

	// The hard block: the request is never sent, and the refusal names the
	// act that changes it (GOV.UK's error-message rule).
	it('blocks the create submit rather than letting the BFF 400 be the first news', async () => {
		await setupWithRoster();
		await openAddVisit();
		await testPage.getByLabelText('Who is this Visit for?').selectOptions(unattachedOption);
		apiFetchWithSession.mockClear();
		await testPage.getByRole('button', { name: 'Save Visit' }).click();

		expect(apiFetchWithSession).not.toHaveBeenCalled();
		// .first() -- #1228's ErrorSummary repeats the message as a link, so
		// it now appears twice on screen (the summary and the field).
		await expect
			.element(
				testPage.getByText(new RegExp(`^${unattached} is a contractor who has not accepted`)).first()
			)
			.toBeVisible();
	});

	it('blocks the reassign submit the same way, with the summary once she has tried', async () => {
		await setupWithRoster();
		await changeVisit(0);
		await testPage.getByLabelText('Who is this Visit for?').selectOptions(unattachedOption);
		await expect.element(testPage.getByRole('heading', { name: 'There is a problem' })).not.toBeInTheDocument();
		apiFetchWithSession.mockClear();
		await testPage.getByRole('button', { name: 'Save changes' }).click();

		expect(apiFetchWithSession).not.toHaveBeenCalled();
		await expect.element(testPage.getByRole('heading', { name: 'There is a problem' })).toBeVisible();
	});

	// The caller's own row takes the self rule, not the named-colleague
	// one: `assignee` (api/internal/visit/roles.go) takes the self path
	// whenever the requested id equals the caller's own, so the fixture's
	// Owner -- herself a contractor Doula with no attachment here -- is
	// accepted by the write when she names herself, and the picker must
	// not block a write the BFF allows.
	it('offers the caller herself without the marker, though she is an unattached contractor', async () => {
		await setupWithRoster();
		await openAddVisit();

		const picker = testPage.getByLabelText('Who is this Visit for?');
		await expect.element(picker).toBeVisible();
		// "(you)" is #909's own marking of her row; what matters here is
		// that "(cannot be named yet)" is not on it.
		expect(
			picker.getByRole('option', { name: 'Anne-Marie Ochieng-Whitfield (you)' }).elements()
		).toHaveLength(1);
	});
});

/*
 * #943: the Engagement hub is the only screen that records ADR-0015's
 * birth outcome. The section's own states are BirthOutcomeSection's
 * spec; what is asserted here is the wiring only -- that the hub reads
 * the two fields off `engagement.Detail`, draws the control for a reader
 * who may record and never for a contractor Doula, and puts to #293's
 * own endpoint.
 */
describe('the birth outcome section', () => {
	const unrecorded: Detail = { ...fixtureDetail, birthOutcome: undefined, pregnancyEndedOn: undefined };

	interface OutcomeOptions {
		detail?: Detail;
		roles?: string[];
		isContractor?: boolean;
	}

	async function setupOutcome({
		detail = fixtureDetail,
		roles = ['owner', 'doula'],
		isContractor = false
	}: OutcomeOptions = {}) {
		await testPage.viewport(3200, 900);
		apiFetchWithSession.mockResolvedValue(jsonResponse('not available', 403));
		await render(Page, {
			data: { ...detail, session: { ...sessionFor(roles), isContractor } }
		});
	}

	it("reads a recorded outcome off the Engagement's own read", async () => {
		await setupOutcome();

		await expect.element(testPage.getByRole('heading', { name: 'Birth outcome' })).toBeVisible();
		await expect.element(testPage.getByText('The baby was born alive')).toBeVisible();
	});

	it("offers a contractor Doula no way to record it, matching the BFF's own refusal", async () => {
		await setupOutcome({ detail: unrecorded, roles: ['doula'], isContractor: true });

		await expect
			.element(testPage.getByRole('button', { name: 'Record what happened' }))
			.not.toBeInTheDocument();
	});

	it('puts what was entered to the birth-outcome endpoint, and reads it back', async () => {
		await setupOutcome({ detail: unrecorded });
		await testPage.getByRole('button', { name: 'Record what happened' }).click();
		await testPage.getByLabelText('The Practice never learned what happened').click();
		apiFetchWithSession.mockClear();
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({ engagementId: 'engagement-1', birthOutcome: 'unknown' })
		);
		await testPage.getByRole('button', { name: 'Record this outcome' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/engagement-1/birth-outcome',
			expect.objectContaining({ method: 'PUT' })
		);
		await expect
			.element(testPage.getByText('The Practice never learned what happened').first())
			.toBeVisible();
	});
});

// #874: ADR-0015's kind is mutable in both directions while the
// pregnancy is still expected, so unlike the birth outcome above there is
// no correction dialog -- a single named button offering the one other
// value, the same bare-command shape the status section's own
// reopen/activate moves use. The one exception is the upgrade,
// postpartum -> birth, once a birth outcome is recorded ("attending a
// birth that has already happened means nothing", ADR-0015) -- the
// fixture's own Engagement already carries one (`birthOutcome:
// 'live_birth'`), so every test below that offers 'Change to Postpartum'
// is exercising the downgrade, which carries no such caveat.
describe('the Kind section (#874)', () => {
	interface KindOptions {
		detail?: Detail;
		roles?: string[];
		isContractor?: boolean;
	}

	async function setupKind({
		detail = fixtureDetail,
		roles = ['owner', 'doula'],
		isContractor = false
	}: KindOptions = {}) {
		await testPage.viewport(3200, 900);
		apiFetchWithSession.mockResolvedValue(jsonResponse('not available', 403));
		await render(Page, {
			data: { ...detail, session: { ...sessionFor(roles), isContractor } }
		});
	}

	it("reads what the Practice sold off the Engagement's own read, and offers the other kind", async () => {
		await setupKind();

		await expect.element(testPage.getByText('Care', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText('Birth', { exact: true })).toBeVisible();
		// #1998: behind the Care row's Change, not in a loose stack.
		expect(testPage.getByRole('button', { name: 'Change to Postpartum' }).elements()).toHaveLength(0);
		await openChange('care');
		await expect.element(testPage.getByRole('button', { name: 'Change to Postpartum' })).toBeVisible();
		expect(changeButton('care')).toHaveAttribute('aria-expanded', 'true');
	});

	it("offers a contractor Doula no way to change it, matching the BFF's own refusal", async () => {
		await setupKind({ roles: ['doula'], isContractor: true });

		await expect
			.element(testPage.getByRole('button', { name: 'Change to Postpartum' }))
			.not.toBeInTheDocument();
		// #1998: and so no Change on the Care row.
		expect(changeButton('care')).toBeUndefined();
	});

	// ADR-0015: "the product stops offering a postpartum -> birth change
	// once the birth outcome is recorded." The fixture's own Engagement
	// carries a recorded outcome, so a postpartum Engagement built from it
	// is exactly this state.
	it("does not offer the upgrade once a birth outcome is recorded, matching the BFF's own refusal", async () => {
		await setupKind({ detail: { ...fixtureDetail, kind: 'postpartum' } });

		await expect.element(testPage.getByText('Postpartum', { exact: true })).toBeVisible();
		expect(changeButton('care')).toBeUndefined();
	});

	// The other side of the same rule: a postpartum Engagement whose
	// pregnancy is still expected -- #874's own motivating case, a Client
	// who decides she wants a birth doula too -- still offers the upgrade.
	it('offers the upgrade while the pregnancy is still expected (no birth outcome recorded)', async () => {
		await setupKind({
			detail: { ...fixtureDetail, kind: 'postpartum', birthOutcome: undefined, pregnancyEndedOn: undefined }
		});

		await openChange('care');
		await expect.element(testPage.getByRole('button', { name: 'Change to Birth' })).toBeVisible();
	});

	it('puts the other kind to the kind endpoint, reads it back and announces the change', async () => {
		await setupKind();
		apiFetchWithSession.mockClear();
		// Scoped to the `/kind` path rather than a blanket
		// `mockResolvedValue`: every other section on this page is still
		// mid-fetch behind its own "not available" 403, and overwriting the
		// default for all of them breaks whichever one settles next (e.g.
		// InvoiceSection reading a kind-shaped body as its own Invoice list).
		apiFetchWithSession.mockImplementation((path: string) =>
			Promise.resolve(
				path.endsWith('/kind')
					? jsonResponse({ engagementId: 'engagement-1', kind: 'postpartum' })
					: jsonResponse('not available', 403)
			)
		);

		await openChange('care');
		await testPage.getByRole('button', { name: 'Change to Postpartum' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/engagement-1/kind',
			expect.objectContaining({ method: 'PUT', body: JSON.stringify({ kind: 'postpartum' }) })
		);
		await expect.element(testPage.getByText('Postpartum', { exact: true })).toBeVisible();
		// The upgrade back to 'birth' is not offered: the fixture's own
		// birth outcome ('live_birth') rides along unchanged -- a kind
		// change moves nothing else -- so this Engagement is now exactly
		// the state ADR-0015's upgrade caveat describes.
		await expect
			.element(testPage.getByRole('button', { name: 'Change to Birth' }))
			.not.toBeInTheDocument();
		// #874's own live-region requirement -- announces the result, not
		// only the visible value change. Scoped by its own text rather than
		// getByRole('status'), which also matches InvoiceSection's unrelated
		// "Invoicing is unavailable" notice on this same fixture.
		await expect.element(testPage.getByText('Now set to Postpartum.')).toBeVisible();
		// #1998: the save closes the Care row's form, so the message takes
		// the focus the pressed button had, and the row has no Change left.
		await expect.element(testPage.getByText('Now set to Postpartum.')).toHaveFocus();
		expect(changeButton('care')).toBeUndefined();
	});

	it('shows the refusal when the BFF rejects the change', async () => {
		await setupKind();
		apiFetchWithSession.mockClear();
		// Same scoping reason as above.
		apiFetchWithSession.mockImplementation((path: string) =>
			Promise.resolve(
				jsonResponse(
					path.endsWith('/kind') ? 'only a Practice Owner, Admin or Doula can do that' : 'not available',
					403
				)
			)
		);

		await openChange('care');
		await testPage.getByRole('button', { name: 'Change to Postpartum' }).click();

		// Scoped by its own text rather than getByRole('alert'), which also
		// matches the page's other sections, each already refusing with the
		// same 403 mock this test installs.
		await expect
			.element(testPage.getByText('only a Practice Owner, Admin or Doula can do that').first())
			.toBeVisible();
	});
});

/*
 * #666: this page's Visits DataTable renders its per-Visit actions into
 * both of the component's trees, so every id in that snippet -- the
 * visually-hidden name span each button is described by, and the reassign,
 * schedule and notes LabeledField ids, which the hint and error ids under
 * them are derived from -- was emitted twice per Visit. Nothing asserted
 * over this page with a non-empty Visits list before, which is why it
 * survived: an id under a `display: none` ancestor is invisible to axe's
 * own duplicate-id rules.
 */
describe('id uniqueness across DataTable two trees (#666)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('emits no duplicate id with a non-empty Visits list on screen', async () => {
		await setupWithRoster();

		// Each row's Change is described by a name span in both trees, and
		// the open change form's ids sit beside them (#1999), so the
		// assertion covers both.
		await changeVisit(0);
		await expect.element(testPage.getByLabelText('Who is this Visit for?')).toBeVisible();

		expect(findDuplicateIds(document)).toEqual([]);
	});
});

/*
 * #2002 and #2003: what a reader sees about money on this page. A
 * contractor Doula who holds neither Owner nor Admin reads her own agreed
 * fee and never the Practice's Invoices (ADR-0008, as amended on #282);
 * everyone else reads the Invoice list as before. `respond` is the
 * fixture's; a test that needs the create to answer overrides that one
 * path.
 */
async function setupMoney(
	reader: typeof session,
	detail: Detail = fixtureDetail,
	respondOverride?: (path: string, init?: RequestInit) => Response | undefined
) {
	await testPage.viewport(3200, 900);
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) =>
		Promise.resolve(respondOverride?.(path, init) ?? respond(path))
	);
	await render(Page, { data: { ...detail, session: reader } });
}

const contractorReader = sessionFor(['doula'], 'staff-9', true);

function invoiceListReads(): unknown[] {
	return apiFetchWithSession.mock.calls.filter(
		([path, init]) => String(path).endsWith('/contract/invoices') && (init as RequestInit | undefined)?.method !== 'POST'
	);
}

describe('what a reader sees about money (#2002, #2003)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('shows a contractor her own fee in the Contract section, and the day she accepted it', async () => {
		await setupMoney(contractorReader);

		await expect.element(testPage.getByText('Your fee')).toBeVisible();
		await expect.element(testPage.getByText('$12,620.00, the fee you accepted on Jul 22, 2026')).toBeVisible();
	});

	it('draws no fee row for a reader the BFF sends no fee', async () => {
		await setupMoney(sessionFor(['owner']), { ...fixtureDetail, yourFee: undefined });

		await expect.element(testPage.getByText('Signed on Aug 14, 2026', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText('Your fee')).not.toBeInTheDocument();
	});

	it("shows a contractor each Invoice's date and status, and says what she does not see, as information (#2094)", async () => {
		await setupMoney(contractorReader, fixtureDetail, (path, init) =>
			path.endsWith('/contract/invoices') && init?.method !== 'POST'
				? jsonResponse({ items: contractorInvoices, hasMore: false })
				: undefined
		);

		const invoices = testPage.getByRole('region', { name: 'Invoices', exact: true });
		await expect.element(invoices.getByRole('cell', { name: 'Aug 1, 2026' })).toBeVisible();
		await expect.element(invoices.getByRole('cell', { name: 'Overdue' })).toBeVisible();
		await expect
			.element(testPage.getByText(/You see when each Invoice was raised and where it stands, not what the Client pays/))
			.toBeVisible();
		expect(invoiceListReads()).toHaveLength(1);
	});

	it('lets a contractor raise an Invoice, tells her it worked, and reads her rows again', async () => {
		await setupMoney(contractorReader, fixtureDetail, (path, init) =>
			path.endsWith('/contract/invoices') && init?.method === 'POST'
				? jsonResponse({ id: 'invoice-9', status: 'open', billingMode: 'stripe' }, 201)
				: undefined
		);

		await testPage.getByRole('button', { name: 'Create Invoice' }).click();

		await expect.element(testPage.getByText('Invoice created.')).toBeVisible();
		expect(invoiceListReads()).toHaveLength(2);
	});

	it('reads and lists the Invoices for an Owner, with no contractor line', async () => {
		await setupMoney(sessionFor(['owner']));

		await expect.element(testPage.getByText('No Invoices yet.')).toBeVisible();
		await expect.element(testPage.getByText(/You contract for this Practice/)).not.toBeInTheDocument();
		expect(invoiceListReads()).toHaveLength(1);
	});
});

// #2004: the thread is read first and its form opens only when asked;
// a Client with no portal account cannot read a Message, so the form is
// not offered and the section says why and what to do.
function messagesSection() {
	return testPage.getByRole('region', { name: 'Messages', exact: true });
}

describe('the Messages section (#2004)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('offers no form to a Client never invited, says why, and offers the invite', async () => {
		await setupMoney(sessionFor(['owner']));

		await expect
			.element(messagesSection().getByText(/reads messages in the portal, so send the portal invite first/))
			.toBeVisible();
		expect(messagesSection().getByRole('button', { name: 'Write a message' }).elements()).toHaveLength(0);
		await expect.element(messagesSection().getByRole('button', { name: 'Send portal invite' })).toBeVisible();
	});

	it('says the thread opens once a pending invite is accepted', async () => {
		await setupMoney(sessionFor(['owner']), { ...fixtureDetail, clientPortalInviteStatus: 'pending' });

		await expect
			.element(
				messagesSection().getByText(`The thread opens once ${fixtureDetail.clientName} has accepted the portal invite.`)
			)
			.toBeVisible();
		expect(messagesSection().getByRole('button', { name: 'Send portal invite' }).elements()).toHaveLength(0);
	});

	it('says an email address comes first when the Client has none', async () => {
		await setupMoney(sessionFor(['owner']), { ...fixtureDetail, clientHasEmail: false });

		await expect.element(messagesSection().getByText(/has no email address on file/)).toBeVisible();
		expect(messagesSection().getByRole('button', { name: 'Send portal invite' }).elements()).toHaveLength(0);
	});

	it('opens the form only when asked, for a Client with a portal account, and sends', async () => {
		await setupMoney(
			sessionFor(['owner']),
			{ ...fixtureDetail, clientPortalInviteStatus: 'accepted' },
			(path, init) =>
				path.endsWith('/messages') && init?.method === 'POST'
					? jsonResponse(
							{
								messageId: 'message-9',
								senderType: 'staff',
								senderId: 'staff-1',
								senderName: 'Renata Alvarez',
								body: 'See you Thursday.',
								createdAt: '2026-10-26T15:00:00Z'
							},
							201
						)
					: undefined
		);

		expect(messagesSection().getByLabelText('Message').elements()).toHaveLength(0);
		await messagesSection().getByRole('button', { name: 'Write a message' }).click();
		await messagesSection().getByLabelText('Message').fill('See you Thursday.');
		await messagesSection().getByRole('button', { name: 'Send', exact: true }).click();

		await expect.element(messagesSection().getByRole('status').filter({ hasText: 'Message sent.' })).toHaveFocus();
		await expect.element(messagesSection().getByText('See you Thursday.', { exact: true })).toBeVisible();
	});

	it('reads the newest message, and whether the Client is waiting on a reply (#2104)', async () => {
		const fromClient = {
			messageId: 'message-2',
			senderType: 'client',
			senderId: 'client-1',
			senderName: 'Camille',
			body: 'Thursday works. See you at 2.',
			createdAt: '2026-10-24T15:00:00Z'
		};
		const fromStaff = {
			...fromClient,
			messageId: 'message-1',
			senderType: 'staff',
			senderName: 'Priya Raman',
			body: 'Thursday at 2?',
			createdAt: '2026-10-23T15:00:00Z'
		};
		await setupMoney(sessionFor(['owner']), fixtureDetail, (path) =>
			path.includes('/messages') ? jsonResponse({ items: [fromClient, fromStaff], hasMore: false }) : undefined
		);

		const latest = messagesSection().getByText('“Thursday works. See you at 2.” — Camille, Oct 24, 2026');
		await expect.element(latest).toBeVisible();
		await expect.element(messagesSection().getByText('Since Oct 24, 2026')).toBeVisible();
	});

	it('draws no Unanswered row once Staff has answered (#2104)', async () => {
		await setupMoney(sessionFor(['owner']), fixtureDetail, (path) =>
			path.includes('/messages')
				? jsonResponse({
						items: [
							{
								messageId: 'message-1',
								senderType: 'staff',
								senderId: 'staff-1',
								senderName: 'Priya Raman',
								body: 'Thursday at 2?',
								createdAt: '2026-10-23T15:00:00Z'
							}
						],
						hasMore: false
					})
				: undefined
		);

		await expect.element(messagesSection().getByText('“Thursday at 2?” — Priya Raman, Oct 23, 2026')).toBeVisible();
		expect(messagesSection().getByText('Unanswered').elements()).toHaveLength(0);
	});
});

describe('how the Contract and the Plans came to be (#2104)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('says when the Contract was signed and sent, and by whom, and when the Birth Plan was last edited', async () => {
		await setupMoney(sessionFor(['owner']));

		await expect.element(testPage.getByText('Signed on Aug 14, 2026', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText('Oluwaseun Adebayo-Montgomery Fitzwilliam')).toBeVisible();
		await expect.element(testPage.getByText('Aug 12, 2026 by Anne-Marie Ochieng-Whitfield')).toBeVisible();
		await expect.element(testPage.getByText('Aug 26, 2026 by Priyanka Venkataraman-Solberg')).toBeVisible();
	});
});
