import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientDetail, HistoryEntry } from '#lib/clientDetail.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts. This
// route's RecordDetail also needs the primitives registered, not just their
// CSS: <center-l max="none"> only lifts the default var(--measure) cap via
// the custom element's own attribute handling, and an unregistered
// center-l never runs it, leaving every DataTable narrower than its floor.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { detail as baseDetail, fixture } from './page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The Client this hub shows, and the `page` it reads, both come from the
 * route's own fixture (#596) -- so what this spec asserts on and what the
 * continuum sweep measures are one description. `vi.mock` is hoisted
 * above every import, so `pageState` is declared empty and filled from
 * the fixture once the imports have run. Same installation, through the
 * same `toPageState`, as `route-continuum.svelte.spec.ts`.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const { practiceId, clientId } = fixture.params;
// The Address DescriptionList row joins the same fields the same way the
// route does -- an empty addressLine2 is filtered out rather than shown
// as a bare comma.
const address = [
	baseDetail.addressLine1,
	baseDetail.addressLine2,
	baseDetail.addressLocality,
	baseDetail.addressRegion,
	baseDetail.addressPostalCode
]
	.filter(Boolean)
	.join(', ');

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

interface SetupOptions {
	overrides?: Partial<ClientDetail>;
	/** The signed-in Staff member's own id, as `/api/staff/session` would
	 * report it -- defaults to somebody who made no Request on this
	 * Client, so the Withdraw button stays absent unless a test opts a
	 * specific requester in. */
	sessionStaffId?: string;
	/** `/api/staff/session`'s own response, when a test needs it to fail
	 * rather than to name somebody. */
	sessionOk?: boolean;
	/** Makes the session read reject outright -- a network failure rather
	 * than a refusal, which nothing awaits and so must not escape as an
	 * unhandled rejection. */
	sessionThrows?: boolean;
	/** `+page.ts`'s own load result (#465) -- defaults to a non-contractor
	 * so existing tests, which render the route directly without going
	 * through SvelteKit's load cycle, keep seeing "Start new work with". */
	isContractor?: boolean;
}

// Two endpoints now share one mocked fetcher (#504's session read joins
// the existing detail read), so setup() dispatches on the path and hands
// back a fresh Response per call -- one shared Response object would have
// its body consumed by the first .json()/.text() and throw on the second.
async function setup({
	overrides = {},
	sessionStaffId = 'nobody',
	sessionOk = true,
	sessionThrows = false,
	isContractor = false
}: SetupOptions = {}) {
	// DataTable's own content floor (#508) stacks it into a <dl> below
	// 46rem, and this file's assertions are about the <table> specifically.
	await testPage.viewport(1440, 900);
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path === '/api/staff/session') {
			if (sessionThrows) return Promise.reject(new Error('network down'));
			return Promise.resolve(sessionOk ? jsonResponse({ staffId: sessionStaffId }) : jsonResponse('signed out', 401));
		}
		return Promise.resolve(jsonResponse({ ...baseDetail, ...overrides }));
	});
	return render(Page, { data: { isContractor } });
}

describe('client detail hub', () => {
	it('renders the twelve structural columns fetched by id', async () => {
		await setup();

		expect(apiFetchWithSession).toHaveBeenCalledWith(`/api/practices/${practiceId}/clients/${clientId}`);
		await expect.element(testPage.getByRole('heading', { level: 1, name: fixture.readyText })).toBeVisible();
		await expect.element(testPage.getByText(baseDetail.email)).toBeVisible();
		await expect.element(testPage.getByText(baseDetail.phone)).toBeVisible();
		await expect.element(testPage.getByText(address)).toBeVisible();
		await expect.element(testPage.getByText(baseDetail.dateOfBirth)).toBeVisible();
	});

	it("groups the structural columns per #432's decision: identity, contact, address", async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 2, name: `Who ${fixture.readyText} is` })).toBeVisible();
		await expect
			.element(testPage.getByRole('heading', { level: 2, name: `How to reach ${fixture.readyText}` }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('heading', { level: 2, name: `Where ${fixture.readyText} lives` }))
			.toBeVisible();
	});

	it('shows every active Practice-defined field, blank or not, and labels an archived one held', async () => {
		await setup({
			overrides: {
				resolvedFields: [
					{ fieldId: 'f1', label: 'Doula notes', type: 'short_text' },
					{ fieldId: 'f2', label: 'Pronouns', type: 'short_text', value: 'she/her' },
					{ fieldId: 'f3', label: 'Old field', type: 'short_text', value: 'kept value', note: 'No longer collected' }
				]
			}
		});

		await expect.element(testPage.getByText('Doula notes')).toBeVisible();
		await expect.element(testPage.getByText('she/her')).toBeVisible();
		await expect.element(testPage.getByText('kept value (No longer collected)')).toBeVisible();
	});

	it("renders her Engagements identifying each one's kind and status", async () => {
		await setup({
			// The fixture's own history carries a pending birth request, whose
			// row would otherwise also read "Birth" and make the query
			// ambiguous -- cleared here because this test is about the
			// Engagements section, not the History one.
			overrides: {
				engagements: [{ engagementId: 'e1', kind: 'birth', status: 'active', createdAt: '2026-01-01T00:00:00Z' }],
				history: []
			}
		});

		await expect.element(testPage.getByRole('cell', { name: 'Birth' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'active' })).toBeVisible();
	});

	it('renders client_events and engagement_requests as one merged timeline', async () => {
		await setup({
			overrides: {
				history: [
					{
						type: 'engagement_request',
						at: '2026-01-02T00:00:00Z',
						engagementRequest: {
							requestId: 'r1',
							kind: 'birth',
							state: 'pending',
							requestedBy: 's1',
							requestedByName: 'Jamie Doula',
							requestedAt: '2026-01-02T00:00:00Z'
						}
					},
					{
						type: 'client_event',
						at: '2026-01-01T00:00:00Z',
						clientEvent: {
							eventType: 'created',
							diff: {},
							actorKind: 'staff',
							actorName: 'Sam Admin',
							createdAt: '2026-01-01T00:00:00Z'
						}
					}
				]
			}
		});

		await expect.element(testPage.getByRole('cell', { name: 'Birth Engagement requested' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Jamie Doula' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Record created' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Sam Admin' })).toBeVisible();
	});

	it('says an erased record was erased, and that Stripe is not done yet (ADR-0027)', async () => {
		await setup({
			overrides: {
				givenName: 'Erased Client',
				familyName: '',
				email: '',
				erasedAt: '2026-03-01T00:00:00Z',
				stripeRedactionEligibleAt: '2026-05-30T00:00:00Z'
			}
		});

		await expect
			.element(testPage.getByText(/This client's data was erased on request/))
			.toBeVisible();
		await expect
			.element(testPage.getByText(/Payment records at Stripe are redactable from/))
			.toBeVisible();
	});

	it('says nothing about erasure for a Client who has not asked', async () => {
		await setup({});

		await expect.element(testPage.getByText(/was erased on request/)).not.toBeInTheDocument();
	});

	it('does not claim Stripe is outstanding once the redaction has run', async () => {
		await setup({ overrides: { erasedAt: '2026-03-01T00:00:00Z' } });

		await expect.element(testPage.getByText(/was erased on request/)).toBeVisible();
		await expect
			.element(testPage.getByText(/redactable from/))
			.not.toBeInTheDocument();
	});

	it('names an erasure as its own act, not another edit (ADR-0027)', async () => {
		await setup({
			overrides: {
				history: [
					{
						type: 'client_event',
						at: '2026-03-01T00:00:00Z',
						clientEvent: {
							eventType: 'erased',
							diff: { contracts: 1, stripeCustomers: 1, portalAccount: true },
							actorKind: 'staff',
							actorName: 'Sam Owner',
							createdAt: '2026-03-01T00:00:00Z'
						}
					},
					{
						type: 'client_event',
						at: '2026-01-01T00:00:00Z',
						clientEvent: {
							eventType: 'updated',
							diff: { erased: true },
							actorKind: 'staff',
							actorName: 'Sam Owner',
							createdAt: '2026-01-01T00:00:00Z'
						}
					}
				]
			}
		});

		await expect.element(testPage.getByRole('cell', { name: 'Data erased on request' })).toBeVisible();
		// The shredded entry that precedes it still reads as the edit it
		// was -- crypto-shredding takes the detail, not the entry.
		await expect.element(testPage.getByRole('cell', { name: 'Record updated' })).toBeVisible();
	});

	it('names a system-authored event "Doula Cloud", never "System" (ADR-0022)', async () => {
		await setup({
			overrides: {
				history: [
					{
						type: 'client_event',
						at: '2026-01-01T00:00:00Z',
						clientEvent: { eventType: 'updated', diff: {}, actorKind: 'system', createdAt: '2026-01-01T00:00:00Z' }
					}
				]
			}
		});

		await expect.element(testPage.getByRole('cell', { name: 'Doula Cloud' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'System', exact: true })).not.toBeInTheDocument();
	});

	it('shows a block naming who asked and when while a Request is pending', async () => {
		await setup({
			overrides: {
				history: [
					{
						type: 'engagement_request',
						at: '2026-01-02T12:00:00Z',
						engagementRequest: {
							requestId: 'r1',
							kind: 'postpartum',
							state: 'pending',
							requestedBy: 's1',
							requestedByName: 'Jamie Doula',
							requestedAt: '2026-01-02T12:00:00Z'
						}
					}
				]
			}
		});

		await expect
			.element(testPage.getByText('Postpartum Engagement requested by Jamie Doula on 1/2/2026'))
			.toBeVisible();
	});

	it('renders an edit link naming whose record it edits', async () => {
		const { container } = await setup();

		const link = testPage.getByRole('link', { name: 'Edit' });
		await expect.element(link).toHaveAttribute('href', `/practices/${practiceId}/clients/${clientId}/edit`);

		// "Edit" alone doesn't say whose record it edits (#513); the
		// distinguishing name is a sibling joined by aria-describedby, the
		// same pattern CheckAnswers' Change links use, so no accessible
		// query names it directly.
		const describedBy = link.element().getAttribute('aria-describedby') ?? '';
		expect(container.querySelector(`#${describedBy}`)?.textContent).toBe(fixture.readyText);
	});

	it('offers the Engagement Request as an action naming her', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: `Start new work with ${fixture.readyText}` }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/${clientId}/engagement-requests/new`);
	});

	it('withholds Start new work from a contractor Doula (ADR-0017: she originates nothing)', async () => {
		await setup({ isContractor: true });

		await expect
			.element(testPage.getByRole('link', { name: `Start new work with ${fixture.readyText}` }))
			.not.toBeInTheDocument();
		// Edit follows read, not the originate rule -- a contractor keeps it.
		await expect.element(testPage.getByRole('link', { name: 'Edit' })).toBeVisible();
	});

	it('shows an error notice when the Client fails to load', async () => {
		apiFetchWithSession.mockImplementation((path: string) =>
			Promise.resolve(
				path === '/api/staff/session' ? jsonResponse({ staffId: 'nobody' }) : jsonResponse('client not found', 404)
			)
		);

		await render(Page, {});

		await expect.element(testPage.getByText('client not found')).toBeVisible();
	});
});

describe('the pending-request block: Withdraw (#504)', () => {
	// Both engagement_requests kinds can be pending on the same Client at
	// once (ADR-0017's unique index is per kind), so the fixture below
	// carries one of each -- the surface a bare "Withdraw" label would
	// have hidden.
	const requestedByHerself: HistoryEntry = {
		type: 'engagement_request',
		at: '2026-01-02T12:00:00Z',
		engagementRequest: {
			requestId: 'request-mendoza-riquelme-postpartum',
			kind: 'postpartum',
			state: 'pending',
			requestedBy: 'staff-mendoza-riquelme',
			requestedByName: 'Alejandra Mendoza-Riquelme',
			requestedAt: '2026-01-02T12:00:00Z'
		}
	};
	const requestedBySomeoneElse: HistoryEntry = {
		type: 'engagement_request',
		at: '2026-01-03T09:00:00Z',
		engagementRequest: {
			requestId: 'request-okonkwo-birth',
			kind: 'birth',
			state: 'pending',
			requestedBy: 'staff-okonkwo',
			requestedByName: 'Chidinma Okonkwo',
			requestedAt: '2026-01-03T09:00:00Z'
		}
	};

	it('offers Withdraw, naming its own kind, only on the Request the signed-in Staff member made herself', async () => {
		await setup({
			overrides: { history: [requestedByHerself, requestedBySomeoneElse] },
			sessionStaffId: 'staff-mendoza-riquelme'
		});

		await expect.element(testPage.getByRole('button', { name: 'Withdraw Postpartum request' })).toBeVisible();
		await expect
			.element(testPage.getByRole('button', { name: 'Withdraw Birth request' }))
			.not.toBeInTheDocument();
	});

	it('offers no Withdraw button when the signed-in Staff member requested nothing pending here', async () => {
		await setup({
			overrides: { history: [requestedByHerself] },
			sessionStaffId: 'staff-someone-uninvolved'
		});

		await expect.element(testPage.getByText('Postpartum Engagement requested by')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: /Withdraw/ })).not.toBeInTheDocument();
	});

	it('offers no Withdraw button when the session read itself fails', async () => {
		await setup({
			overrides: { history: [requestedByHerself] },
			sessionStaffId: 'staff-mendoza-riquelme',
			sessionOk: false
		});

		await expect.element(testPage.getByText('Postpartum Engagement requested by')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: /Withdraw/ })).not.toBeInTheDocument();
	});

	// Nothing awaits the session read, so a rejection here would surface as
	// an unhandled rejection rather than as a missing button.
	it('survives the session read failing outright, and still draws the Client', async () => {
		await setup({
			overrides: { history: [requestedByHerself] },
			sessionStaffId: 'staff-mendoza-riquelme',
			sessionThrows: true
		});

		await expect.element(testPage.getByText('Postpartum Engagement requested by')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: /Withdraw/ })).not.toBeInTheDocument();
	});

	it('withdraws on click: the block disappears and a status notice announces it', async () => {
		await setup({
			overrides: { history: [requestedByHerself] },
			sessionStaffId: 'staff-mendoza-riquelme'
		});

		await testPage.getByRole('button', { name: 'Withdraw Postpartum request' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			`/api/practices/${practiceId}/engagement-requests/request-mendoza-riquelme-postpartum/withdraw`,
			{ method: 'POST' }
		);
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Postpartum Engagement request withdrawn');
		await expect
			.element(testPage.getByText('Postpartum Engagement requested by Alejandra Mendoza-Riquelme'))
			.not.toBeInTheDocument();
		// The merged history row reflects the same withdrawal rather than
		// staying frozen on "requested" until a reload.
		await expect.element(testPage.getByRole('cell', { name: 'Postpartum Engagement request withdrawn' })).toBeVisible();
	});

	it('shows the refusal text and leaves the block standing when withdraw fails', async () => {
		apiFetchWithSession.mockImplementation((path: string) => {
			if (path === '/api/staff/session') {
				return Promise.resolve(jsonResponse({ staffId: 'staff-mendoza-riquelme' }));
			}
			if (path.endsWith('/withdraw')) {
				return Promise.resolve(jsonResponse('that request is no longer pending -- it is approved', 409));
			}
			return Promise.resolve(jsonResponse({ ...baseDetail, history: [requestedByHerself] }));
		});

		await render(Page, {});
		await testPage.getByRole('button', { name: 'Withdraw Postpartum request' }).click();

		await expect
			.element(testPage.getByText('that request is no longer pending -- it is approved'))
			.toBeVisible();
		await expect
			.element(testPage.getByText('Postpartum Engagement requested by Alejandra Mendoza-Riquelme'))
			.toBeVisible();
	});
});
