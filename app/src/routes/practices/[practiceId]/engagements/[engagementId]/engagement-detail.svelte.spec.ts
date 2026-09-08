import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
// Rendering `+page.svelte` directly bypasses `+layout.svelte`, the only
// place the real app calls this -- without it a layout primitive like
// `center-l`'s own `max` override sits unregistered and inert (matching
// clients-list.svelte.spec.ts's own reason for the same line). #486's own
// DataTable assertions below need the real table-view/record-view switch.
import '#lib/styles/app.css';
import { toApiResponder, toPageState } from '../../../../routeFixture.js';
import { detail as fixtureDetail, fixture } from './page.fixture.js';
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

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: vi.fn(async (response: Response) => await response.text())
}));

// A real service-worker push subscription would fail in headless Chromium
// -- the summary this spec covers doesn't depend on it, so it's stood in
// with a no-op unsubscribe.
vi.mock('#lib/pushRefresh.js', () => ({ subscribeToThreadPushMessages: vi.fn(() => vi.fn()) }));

interface Detail {
	engagementId: string;
	clientId: string;
	clientName: string;
	status: string;
	createdAt: string;
	dueDate?: string;
	statusMoves: string[];
	birthOutcome?: string;
	pregnancyEndedOn?: string;
	clientPortalInviteStatus?: string;
	clientEmailSuppressed?: boolean;
	clientHasEmail?: boolean;
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
function sessionFor(roles: string[] = [], staffId = 'staff-1') {
	return {
		practiceId: fixture.params.practiceId,
		staffId,
		practiceName: 'Riverside Doula Collective',
		roles,
		isContractor: false
	};
}

async function setup(detail: Detail, activityResponse?: Response) {
	await testPage.viewport(1440, 900);
	if (activityResponse) {
		apiFetchWithSession.mockImplementation((path: string) =>
			Promise.resolve(path.includes('/activity') ? activityResponse : jsonResponse('not available', 403))
		);
	} else {
		apiFetchWithSession.mockResolvedValue(jsonResponse('not available', 403));
	}
	// No `params` prop: the page reads `page.params`, which the $app/state
	// `params` rides along because PageProps requires it; the page itself
	// reads `page.params`, which the $app/state mock above supplies. Both
	// come from the fixture, so the two cannot disagree about which
	// Engagement this is (#596).
	await render(Page, {
		data: { ...detail, session: sessionFor() },
		params: fixture.params
	});
}

// Renders directly rather than through setup(): that helper's own
// activityResponse-less branch always overwrites apiFetchWithSession
// with a blanket 403 (its "everything refuses by default" default),
// which would erase the fixture responder a caller sets up itself -- the
// #841 test below takes the same direct-render path for the same reason.
async function renderWithFixtureResponder(
	respondOverride?: (path: string, init?: RequestInit) => Promise<Response> | undefined,
	detail: Detail = fixtureDetail
) {
	await testPage.viewport(1440, 900);
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		const overridden = respondOverride?.(path, init);
		if (overridden) return overridden;
		return respond(path);
	});
	await render(Page, {
		data: {
			...detail,
			session: {
				practiceId: fixture.params.practiceId,
				staffId: 'staff-1',
				practiceName: 'Riverside Doula Collective',
				roles: [],
				isContractor: false
			}
		},
		params: fixture.params
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
	await testPage.viewport(1440, 900);
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string) => {
		if (rosterResponse && path.endsWith('/visit-assignees')) return Promise.resolve(rosterResponse);
		return respond(path);
	});
	await render(Page, {
		data: { ...fixtureDetail, session: sessionFor(roles, staffId) },
		params: fixture.params
	});
}

describe('Staff Engagement detail summary', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('shows the due date under its own label, alongside Created (#538)', async () => {
		// The fixture's own Engagement (#596) already carries a due date.
		await setup(fixtureDetail);

		await expect.element(testPage.getByText('Due date')).toBeVisible();
		await expect.element(testPage.getByText('Mar 1, 2027')).toBeVisible();
		await expect.element(testPage.getByText('Created')).toBeVisible();
	});

	// ADR-0017: a postpartum-only Engagement has no due date. #538 asks for
	// nothing to show, not a blank row and not a placeholder -- the row is
	// left out of the DescriptionList's own items entirely. The fixture's
	// Engagement always carries a due date, so a Detail with none is this
	// test's own -- the fixture has no way to hold the absence (#596).
	it('shows nothing for a null due date -- no blank label, no placeholder', async () => {
		await setup({ ...fixtureDetail, dueDate: undefined });

		await expect.element(testPage.getByText('active')).toBeVisible();
		await expect.element(testPage.getByText(/due date/i)).not.toBeInTheDocument();
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

		await testPage.getByRole('button', { name: 'Reopen (correction)' }).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.body).toEqual({ status: 'active' });
		await expect.element(testPage.getByText('active', { exact: true })).toBeVisible();
	});

	it('asks for a reason before completing, and refuses to submit without one', async () => {
		const requests: unknown[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'PATCH' || !path.endsWith('/status')) return;
			requests.push(init.body);
			return Promise.resolve(jsonResponse({ status: 'completed', statusMoves: ['active'] }));
		});

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
		await expect.element(testPage.getByText('completed', { exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Confirm completion' })).not.toBeInTheDocument();
	});

	it('shows the refusal when the BFF rejects the move', async () => {
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'PATCH' || !path.endsWith('/status')) return;
			return Promise.resolve(jsonResponse('endingReason is required to complete an Engagement', 400));
		});

		await testPage.getByRole('button', { name: 'Mark care complete' }).click();
		await testPage.getByLabelText('The work finished as agreed').click();
		await testPage.getByRole('button', { name: 'Confirm completion' }).click();

		await expect
			.element(testPage.getByText('endingReason is required to complete an Engagement'))
			.toBeVisible();
	});

	// #940: ADR-0015 will not let an Engagement reach 'completed' while
	// its birth outcome is null, and the BFF says so as a named 409
	// rather than letting the CHECK surface as a service problem. The
	// words are the BFF's own -- it is the only thing that knows which of
	// the two facts is missing -- and the control that answers them is
	// this page's own birth-outcome section, a few hundred pixels down.
	it('shows the BFF refusal when the Engagement has no birth outcome recorded', async () => {
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'PATCH' || !path.endsWith('/status')) return;
			return Promise.resolve(
				jsonResponse(
					{
						code: 'BIRTH_OUTCOME_REQUIRED',
						message:
							'record what happened to the pregnancy before completing this Engagement; if the Practice never learned, say so, and that answer needs no date'
					},
					409
				)
			);
		});

		await testPage.getByRole('button', { name: 'Mark care complete' }).click();
		await testPage.getByLabelText('The work finished as agreed').click();
		await testPage.getByRole('button', { name: 'Confirm completion' }).click();

		await expect
			.element(testPage.getByText(/record what happened to the pregnancy before completing/i))
			.toBeVisible();
		// The completion form stays open behind the refusal: the reader's
		// reason and note survive while she goes and records the outcome,
		// rather than being thrown away by a page that reset itself.
		await expect.element(testPage.getByRole('button', { name: 'Confirm completion' })).toBeVisible();
	});
});

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
		await expect.element(testPage.getByRole('cell', { name: 'Visit logged' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Maya Torres' })).toBeVisible();
	});

	it('shows the empty-ledger message when the Engagement has no activity yet', async () => {
		await setup(fixtureDetail, jsonResponse({ items: [], hasMore: false }));

		await expect.element(testPage.getByRole('cell', { name: 'Nothing has happened yet.' })).toBeVisible();
	});

	it('says so when the ledger cannot be read', async () => {
		await setup(fixtureDetail, jsonResponse('nope', 403));

		await expect.element(testPage.getByText('nope')).toBeVisible();
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
		const expected = `${scheduled.getDate()} ${scheduled.toLocaleDateString('en-US', { month: 'short' })} ${scheduled.getFullYear()}, ${hour12}:${minute}${suffix}`;
		await expect.element(testPage.getByRole('cell', { name: expected, exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Not yet scheduled', exact: true })).toBeVisible();
	});

	it('sets an unscheduled Visit schedule from its own row control', async () => {
		const requests: { path: string; body: unknown }[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'PATCH' || !path.endsWith('/schedule')) return;
			requests.push({ path, body: init.body ? JSON.parse(init.body as string) : undefined });
			return Promise.resolve(jsonResponse({ visitId: 'visit-2', scheduledAt: '2027-04-01T09:00:00Z' }));
		});

		// Jordan Reyes is visit-2, the fixture's unscheduled row (#250) --
		// its own field is scoped by that row's unique label id. `exact`
		// excludes the "Add a Visit" form's own field just above the
		// table, whose label ("...(optional)") contains this one's as a
		// substring.
		const field = testPage.getByLabelText('Scheduled date and time', { exact: true }).nth(1);
		await field.fill('2027-04-01T09:00');
		await testPage.getByRole('button', { name: 'Update schedule' }).nth(1).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.path).toContain('/visits/visit-2/schedule');
		expect(requests[0]!.body).toEqual({ scheduledAt: new Date('2027-04-01T09:00').toISOString() });
	});

	it('creates a Visit already scheduled from the Add a Visit form', async () => {
		const requests: { path: string; body: unknown }[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'POST' || !path.endsWith('/visits')) return;
			requests.push({ path, body: init.body ? JSON.parse(init.body as string) : undefined });
			return Promise.resolve(jsonResponse({ visitId: 'visit-3', staffId: 'staff-1' }, 201));
		});

		await testPage.getByLabelText('Who is this Visit for?').selectOptions('Jordan Reyes');
		await testPage.getByLabelText('Scheduled date and time (optional)').fill('2027-05-20T10:15');
		await testPage.getByRole('button', { name: 'Add a Visit' }).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.path).toContain('/visits');
		expect(requests[0]!.body).toEqual({
			scheduledAt: new Date('2027-05-20T10:15').toISOString(),
			staffId: 'staff-2'
		});
	});
});

// #251: a Visit's own free-text notes, staff-writable in place.
describe('the Visits section notes control (#251)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it("shows a Visit's own notes, and the empty-state text for one that has never had any", async () => {
		await renderWithFixtureResponder();

		// `exact` matters here: the row's own notes control (visitActions)
		// carries this same text as its Textarea's current value, which
		// widens the Actions cell's own accessible name to a longer string
		// that contains this one as a substring -- an inexact or regex
		// match would resolve both cells and violate strict mode.
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

	it('saves a Visit without notes from its own row control', async () => {
		const requests: { path: string; body: unknown }[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'PATCH' || !path.endsWith('/notes')) return;
			requests.push({ path, body: init.body ? JSON.parse(init.body as string) : undefined });
			return Promise.resolve(jsonResponse({ visitId: 'visit-2', notes: 'First check-in went well.' }));
		});

		// Jordan Reyes is visit-2, the fixture's row with no prior notes.
		const field = testPage.getByLabelText('Notes').nth(1);
		await field.fill('First check-in went well.');
		await testPage.getByRole('button', { name: 'Save notes' }).nth(1).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.path).toContain('/visits/visit-2/notes');
		expect(requests[0]!.body).toEqual({ notes: 'First check-in went well.' });
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
		await testPage.viewport(1440, 900);
		const respond = toApiResponder(fixture);
		apiFetchWithSession.mockImplementation((path: string) =>
			path.endsWith('/contract') ? Promise.resolve(jsonResponse('contract is down', 500)) : respond(path)
		);

		await render(Page, {
			data: { ...fixtureDetail, session: sessionFor() },
			params: fixture.params
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
describe("the Client's portal-invite state and the Contract section's block (#255)", () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	const draftContract = {
		engagementId: 'engagement-1',
		status: 'draft',
		prose: 'This Contract is between {{practice_name}} and {{client_name}}.',
		mergeFields: [],
		values: {}
	};

	function mockDraftContract() {
		const respond = toApiResponder(fixture);
		apiFetchWithSession.mockImplementation((path: string) => {
			if (path.endsWith('/contract')) return Promise.resolve(jsonResponse(draftContract));
			return respond(path);
		});
	}

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

		await expect.element(testPage.getByRole('button', { name: 'Send portal invite' })).toBeVisible();
	});

	it('withholds "Send portal invite" once the Client has accepted one', async () => {
		await setup({ ...fixtureDetail, clientPortalInviteStatus: 'accepted' });

		expect(testPage.getByRole('button', { name: 'Send portal invite' }).elements()).toHaveLength(0);
	});

	it('disables Send Contract and names the ordering while the Client has never been invited', async () => {
		mockDraftContract();
		await render(Page, {
			data: { ...fixtureDetail, session: sessionFor() },
			params: fixture.params
		});

		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeDisabled();
		await expect
			.element(testPage.getByText(/send a portal invite to this client before sending the contract/i))
			.toBeVisible();
	});

	it('enables Send Contract once the Client has a pending or accepted invite', async () => {
		mockDraftContract();
		await render(Page, {
			data: { ...fixtureDetail, clientPortalInviteStatus: 'pending', session: sessionFor() },
			params: fixture.params
		});

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
		const respond = toApiResponder(fixture);
		apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
			if (path.endsWith('/contract')) return Promise.resolve(jsonResponse(draftContract));
			if (path.endsWith('/portal-invite') && init?.method === 'POST') {
				return Promise.resolve(jsonResponse({ inviteToken: 'a-fresh-token' }));
			}
			return respond(path);
		});
		await render(Page, {
			data: { ...fixtureDetail, session: sessionFor() },
			params: fixture.params
		});
		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeDisabled();

		await testPage.getByRole('button', { name: 'Send portal invite' }).click();

		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeEnabled();
		await expect.element(testPage.getByText('Invite pending')).toBeVisible();
	});
});

// Mocks the Practice-side GET .../contract response to contract, letting
// every other section respond from the fixture as normal -- shared by
// the #258 describe block below.
function mockContract(contract: Record<string, unknown>) {
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.endsWith('/contract')) return Promise.resolve(jsonResponse(contract));
		return respond(path);
	});
}

describe("the Contract's merge-field completeness block and filled-text render (#258)", () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	// clientPortalInviteStatus: 'accepted' throughout, so #255's own block
	// never engages here -- this block is only about #258's own precondition.
	it('renders the Contract prose with merge values substituted, on the Staff side', async () => {
		mockContract({
			engagementId: 'engagement-1',
			status: 'draft',
			prose: 'This Contract is between {{practice_name}} and {{client_name}}.',
			mergeFields: ['practice_name', 'client_name'],
			values: { practice_name: 'Riverside Doulas', client_name: 'Jamie Rivera' }
		});
		await render(Page, {
			data: { ...fixtureDetail, clientPortalInviteStatus: 'accepted', session: sessionFor() },
			params: fixture.params
		});

		await expect
			.element(testPage.getByText('This Contract is between Riverside Doulas and Jamie Rivera.'))
			.toBeVisible();
	});

	it('disables Send Contract and names the blank fields while any merge field is unfilled', async () => {
		mockContract({
			engagementId: 'engagement-1',
			status: 'draft',
			prose: 'This Contract is between {{practice_name}} and {{client_name}}.',
			mergeFields: ['practice_name', 'client_name'],
			values: { client_name: 'Jamie Rivera' }
		});
		await render(Page, {
			data: { ...fixtureDetail, clientPortalInviteStatus: 'accepted', session: sessionFor() },
			params: fixture.params
		});

		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeDisabled();
		await expect
			.element(
				testPage.getByText(/fill in every merge field before sending the contract\. missing: practice name\./i)
			)
			.toBeVisible();
	});

	it('enables Send Contract once every merge field has a value', async () => {
		mockContract({
			engagementId: 'engagement-1',
			status: 'draft',
			prose: 'This Contract is between {{practice_name}} and {{client_name}}.',
			mergeFields: ['practice_name', 'client_name'],
			values: { practice_name: 'Riverside Doulas', client_name: 'Jamie Rivera' }
		});
		await render(Page, {
			data: { ...fixtureDetail, clientPortalInviteStatus: 'accepted', session: sessionFor() },
			params: fixture.params
		});

		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeEnabled();
	});

	// #258: for an Owner/Admin reader, GetContractHandler's response splits
	// a money-tagged key (ADR-0008) into a separate moneyValues field
	// rather than including it in values -- reading values alone would
	// render this filled field as blank and flag it as missing.
	it('renders a filled money-tagged field substituted and does not count it as missing, for an Owner/Admin reader', async () => {
		mockContract({
			engagementId: 'engagement-1',
			status: 'draft',
			prose: 'This Contract is between {{practice_name}} and {{client_name}} for {{money_price}}.',
			mergeFields: ['practice_name', 'client_name', 'money_price'],
			values: { practice_name: 'Riverside Doulas', client_name: 'Jamie Rivera' },
			moneyValues: { money_price: '$1,200' }
		});
		await render(Page, {
			data: { ...fixtureDetail, clientPortalInviteStatus: 'accepted', session: sessionFor() },
			params: fixture.params
		});

		await expect
			.element(testPage.getByText('This Contract is between Riverside Doulas and Jamie Rivera for $1,200.'))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Send Contract' })).toBeEnabled();
	});
});

describe('the Contract PDF download is Owner/Admin-gated on the page (#302)', () => {
	const signedContract = {
		engagementId: 'engagement-1',
		status: 'signed',
		prose: 'This Contract is between {{practice_name}} and {{client_name}}.',
		mergeFields: [],
		values: {}
	};

	function mockSignedContract(pdfResponse: Response) {
		const respond = toApiResponder(fixture);
		apiFetchWithSession.mockImplementation((path: string) => {
			if (path.endsWith('/contract/pdf')) return Promise.resolve(pdfResponse);
			if (path.endsWith('/contract')) return Promise.resolve(jsonResponse(signedContract));
			return respond(path);
		});
	}

	async function setupWithRoles(roles: string[], pdfResponse: Response) {
		await testPage.viewport(1440, 900);
		mockSignedContract(pdfResponse);
		await render(Page, {
			data: { ...fixtureDetail, session: sessionFor(roles) },
			params: fixture.params
		});
	}

	it('offers the download to an Owner', async () => {
		await setupWithRoles(['owner'], pdfBlobResponse());

		await expect
			.element(testPage.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.toBeVisible();
	});

	it('does not render the download for a Doula, the role the endpoint refuses', async () => {
		await setupWithRoles(['doula'], pdfBlobResponse());

		await expect.element(testPage.getByText('Status: signed')).toBeVisible();
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

// #306: the same PDF a Client can download from the portal, mirrored on
// this side -- built fresh on every request from the plan's current
// answers, never a stored snapshot. Birth Plan only, not Care Plan: the
// decision recorded on #306 scoped the download to Birth Plan alone.
describe('the Birth Plan PDF download (#306)', () => {
	const birthPlanInstance = {
		engagementId: fixture.params.engagementId,
		planType: 'birth_plan',
		fields: [
			{ id: 'location', type: 'single_select', label: 'Planned birth location', options: ['Home', 'Hospital'], order: 0 }
		],
		answers: { location: 'Hospital' }
	};

	async function setupWithBirthPlan(pdfResponse: Response) {
		await testPage.viewport(1440, 900);
		const respond = toApiResponder(fixture);
		apiFetchWithSession.mockImplementation((path: string) => {
			if (path.endsWith('/plans/birth_plan/pdf')) return Promise.resolve(pdfResponse);
			if (path.endsWith('/plans/birth_plan')) return Promise.resolve(jsonResponse(birthPlanInstance));
			return respond(path);
		});
		await render(Page, {
			data: { ...fixtureDetail, session: sessionFor() },
			params: fixture.params
		});
	}

	it('offers no download before a Birth Plan has been created', async () => {
		await testPage.viewport(1440, 900);
		await render(Page, { data: { ...fixtureDetail, session: sessionFor() }, params: fixture.params });

		expect(testPage.getByRole('button', { name: 'Download Birth Plan (PDF)' }).elements()).toHaveLength(0);
	});

	it('offers a download once a Birth Plan exists', async () => {
		await setupWithBirthPlan(pdfBlobResponse());

		await expect.element(testPage.getByRole('button', { name: 'Download Birth Plan (PDF)' })).toBeVisible();
	});

	it('never offers a Care Plan download, even once a Birth Plan exists', async () => {
		await setupWithBirthPlan(pdfBlobResponse());

		expect(testPage.getByRole('button', { name: 'Download Care Plan (PDF)' }).elements()).toHaveLength(0);
	});

	it('fetches the pdf path when clicked', async () => {
		await setupWithBirthPlan(pdfBlobResponse());
		await testPage.getByRole('button', { name: 'Download Birth Plan (PDF)' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			`/api/practices/${fixture.params.practiceId}/engagements/${fixture.params.engagementId}/plans/birth_plan/pdf`
		);
	});

	it('reports a failed pdf fetch in words', async () => {
		await setupWithBirthPlan(new Response('no plan instance found for this engagement and plan type', { status: 500 }));
		await testPage.getByRole('button', { name: 'Download Birth Plan (PDF)' }).click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('no plan instance found for this engagement and plan type');
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

		await expect.element(testPage.getByLabelText('Who is this Visit for?')).toBeVisible();
		expect(
			testPage.getByRole('option', { name: 'Winifred Abernathy-Castellano' }).elements()
		).toHaveLength(0);
	});

	it('offers a select, not a free-text staff id, for reassigning a Visit', async () => {
		await setupWithRoster();

		const picker = testPage.getByLabelText('Reassign to').first();
		await expect.element(picker).toBeVisible();
		expect(picker.element().tagName).toBe('SELECT');
	});

	// The defect this replaces: no screen in the product prints a staff
	// id, so a field asking for one could not be filled in.
	it('offers no staff-id text field anywhere on the page', async () => {
		await setupWithRoster();

		await expect.element(testPage.getByLabelText('Who is this Visit for?')).toBeVisible();
		expect(testPage.getByLabelText(/staff id/i).elements()).toHaveLength(0);
	});

	// #274's own criterion: absent, not broken and not erroring. A plain
	// Doula may not read the roster (ADR-0008), so she is offered no
	// assign control at all -- and still logs her own Visit, which needs
	// no roster.
	it('leaves both pickers out for a reader the roster read refuses, and still lets her log her own Visit', async () => {
		await setupWithRoster(jsonResponse('not permitted to read this', 403), ['doula']);

		await expect.element(testPage.getByRole('button', { name: 'Add a Visit' })).toBeVisible();
		expect(testPage.getByLabelText('Who is this Visit for?').elements()).toHaveLength(0);
		expect(testPage.getByLabelText('Reassign to').elements()).toHaveLength(0);
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

			const picker = testPage.getByLabelText('Who is this Visit for?');
			await expect.element(picker).toHaveValue('staff-1');
			// First non-placeholder option, so a standing answer is visible
			// as an answer rather than as an arbitrary top of the list.
			const options = [...picker.element().querySelectorAll('option')].map((o) => o.textContent);
			expect(options[1]).toBe('Anne-Marie Ochieng-Whitfield (you)');
		});

		it('logs the Visit for the caller without her touching the picker', async () => {
			await setupWithRoster();
			await expect.element(testPage.getByLabelText('Who is this Visit for?')).toHaveValue('staff-1');
			await testPage.getByRole('button', { name: 'Add a Visit' }).click();

			expect(apiFetchWithSession).toHaveBeenCalledWith(
				'/api/practices/practice-1/engagements/engagement-1/visits',
				expect.objectContaining({
					body: JSON.stringify({ scheduledAt: undefined, staffId: 'staff-1' })
				})
			);
		});

		// The other two thirds of the split. An Admin who is not on the
		// Doula roster has no self to fall back on, so she is asked, and
		// nothing is chosen for her.
		it('opens on the placeholder for an Admin who holds no Doula role', async () => {
			await setupWithRoster(undefined, ['admin'], 'staff-bookkeeper');

			await expect.element(testPage.getByLabelText('Who is this Visit for?')).toHaveValue('');
		});

		// And a plain Doula is never asked at all -- the roster read
		// refuses her, and an absent assignee already means her.
		it('asks a plain Doula nothing, and still lets her log her own Visit', async () => {
			await setupWithRoster(jsonResponse('not permitted to read this', 403), ['doula'], 'staff-2');

			await expect.element(testPage.getByRole('button', { name: 'Add a Visit' })).toBeVisible();
			expect(testPage.getByLabelText('Who is this Visit for?').elements()).toHaveLength(0);
		});

		// The standing answer is a default, not a lock: the same Owner also
		// books her colleagues' Visits, and naming one has to stick.
		it('keeps the colleague she names, rather than reverting to her own name', async () => {
			await setupWithRoster();
			const picker = testPage.getByLabelText('Who is this Visit for?');
			await expect.element(picker).toHaveValue('staff-1');

			await picker.selectOptions('Jordan Reyes');

			await expect.element(picker).toHaveValue('staff-2');
		});

		// visit-1 is assigned to staff-1, visit-2 to staff-2 -- offering
		// either her own Visit back to her is a move that did not happen.
		it("leaves a Visit's current assignee out of its own reassign picker", async () => {
			await setupWithRoster();

			const forVisitOne = testPage.getByLabelText('Reassign to').first();
			await expect.element(forVisitOne).toBeVisible();
			expect(
				forVisitOne.getByRole('option', { name: 'Anne-Marie Ochieng-Whitfield (you)' }).elements()
			).toHaveLength(0);
			// She is still offered on a Visit that is not already hers.
			const forVisitTwo = testPage.getByLabelText('Reassign to').nth(1);
			expect(
				forVisitTwo.getByRole('option', { name: 'Anne-Marie Ochieng-Whitfield (you)' }).elements()
			).toHaveLength(1);
			expect(forVisitTwo.getByRole('option', { name: 'Jordan Reyes' }).elements()).toHaveLength(0);
		});

		// ADR-0024's floor, on the real route rather than on a style-guide
		// demo: the fixture roster is fourteen Doulas with long real-world
		// names, and the marked option is the longest of the lot.
		it('never scrolls the document sideways at 320px with a fourteen-name roster', async () => {
			await setupWithRoster();
			await testPage.viewport(320, 800);

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

			await expect
				.element(testPage.getByText('There is nobody else to reassign this Visit to.').first())
				.toBeVisible();
			// visit-2 is Jordan Reyes's, and she is not on this roster, so
			// its own picker still offers the sole Doula.
			expect(testPage.getByLabelText('Reassign to').elements()).toHaveLength(1);
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
		await testPage.getByLabelText('Who is this Visit for?').selectOptions('Jordan Reyes');
		await testPage.getByRole('button', { name: 'Add a Visit' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/engagement-1/visits',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ scheduledAt: undefined, staffId: 'staff-2' })
			})
		);
	});

	it('sends the chosen colleague on the reassign request', async () => {
		await setupWithRoster();
		await testPage.getByLabelText('Reassign to').first().selectOptions('Jordan Reyes');
		await testPage.getByRole('button', { name: 'Reassign' }).first().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/engagement-1/visits/visit-1',
			expect.objectContaining({ method: 'PATCH', body: JSON.stringify({ staffId: 'staff-2' }) })
		);
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

		const picker = testPage.getByLabelText('Reassign to').first();
		await expect.element(picker).toBeVisible();
		expect(picker.getByRole('option', { name: unattachedOption }).elements()).toHaveLength(1);
	});

	// What the marker means and what changes it, in text, before anybody
	// has to choose a name to find out -- not conveyed by color and not by
	// a bare `disabled` attribute.
	it('says in text what the marker means and what makes her nameable', async () => {
		await setupWithRoster();

		await expect
			.element(testPage.getByText(/Send her an Offer, then name her on a Visit/).first())
			.toBeVisible();
	});

	// The hard block: the request is never sent, and the refusal names the
	// act that changes it (GOV.UK's error-message rule).
	it('blocks the create submit rather than letting the BFF 400 be the first news', async () => {
		await setupWithRoster();
		await testPage.getByLabelText('Who is this Visit for?').selectOptions(unattachedOption);
		apiFetchWithSession.mockClear();
		await testPage.getByRole('button', { name: 'Add a Visit' }).click();

		expect(apiFetchWithSession).not.toHaveBeenCalled();
		await expect
			.element(
				testPage.getByText(new RegExp(`^${unattached} is a contractor who has not accepted`))
			)
			.toBeVisible();
	});

	it('blocks the reassign submit the same way', async () => {
		await setupWithRoster();
		await testPage.getByLabelText('Reassign to').first().selectOptions(unattachedOption);
		apiFetchWithSession.mockClear();
		await testPage.getByRole('button', { name: 'Reassign' }).first().click();

		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	// The caller's own row takes the self rule, not the named-colleague
	// one: `assignee` (api/internal/visit/roles.go) takes the self path
	// whenever the requested id equals the caller's own, so the fixture's
	// Owner -- herself a contractor Doula with no attachment here -- is
	// accepted by the write when she names herself, and the picker must
	// not block a write the BFF allows.
	it('offers the caller herself without the marker, though she is an unattached contractor', async () => {
		await setupWithRoster();

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
		await testPage.viewport(1440, 900);
		apiFetchWithSession.mockResolvedValue(jsonResponse('not available', 403));
		await render(Page, {
			data: { ...detail, session: { ...sessionFor(roles), isContractor } },
			params: fixture.params
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
