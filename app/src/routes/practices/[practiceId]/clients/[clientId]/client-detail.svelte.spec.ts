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

if (!customElements.get('center-l')) registerLayoutPrimitives();

vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1', clientId: 'client-1' } }
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const baseDetail: ClientDetail = {
	id: 'client-1',
	givenName: 'Ada',
	familyName: 'Lovelace',
	preferredName: 'Ada',
	email: 'ada@example.com',
	phone: '555-0100',
	addressLine1: '1 Analytical Engine Way',
	addressLine2: '',
	addressLocality: 'London',
	addressRegion: 'LDN',
	addressPostalCode: 'SW1A 1AA',
	dateOfBirth: '1815-12-10',
	resolvedFields: [],
	engagements: [],
	history: []
};

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
}

// Two endpoints now share one mocked fetcher (#504's session read joins
// the existing detail read), so setup() dispatches on the path and hands
// back a fresh Response per call -- one shared Response object would have
// its body consumed by the first .json()/.text() and throw on the second.
async function setup({
	overrides = {},
	sessionStaffId = 'nobody',
	sessionOk = true,
	sessionThrows = false
}: SetupOptions = {}) {
	// DataTable's own content floor (#508) stacks it into a <dl> below
	// 44rem, and this file's assertions are about the <table> specifically.
	await testPage.viewport(1440, 900);
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path === '/api/staff/session') {
			if (sessionThrows) return Promise.reject(new Error('network down'));
			return Promise.resolve(sessionOk ? jsonResponse({ staffId: sessionStaffId }) : jsonResponse('signed out', 401));
		}
		return Promise.resolve(jsonResponse({ ...baseDetail, ...overrides }));
	});
	return render(Page, {});
}

describe('client detail hub', () => {
	it('renders the twelve structural columns fetched by id', async () => {
		await setup();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/practices/practice-1/clients/client-1');
		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Ada' })).toBeVisible();
		await expect.element(testPage.getByText('ada@example.com')).toBeVisible();
		await expect.element(testPage.getByText('555-0100')).toBeVisible();
		await expect.element(testPage.getByText('1 Analytical Engine Way, London, LDN, SW1A 1AA')).toBeVisible();
		await expect.element(testPage.getByText('1815-12-10')).toBeVisible();
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
			overrides: { engagements: [{ engagementId: 'e1', kind: 'birth', status: 'active', createdAt: '2026-01-01T00:00:00Z' }] }
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
		await expect.element(link).toHaveAttribute('href', '/practices/practice-1/clients/client-1/edit');

		// "Edit" alone doesn't say whose record it edits (#513); the
		// distinguishing name is a sibling joined by aria-describedby, the
		// same pattern CheckAnswers' Change links use, so no accessible
		// query names it directly.
		const describedBy = link.element().getAttribute('aria-describedby') ?? '';
		expect(container.querySelector(`#${describedBy}`)?.textContent).toBe('Ada');
	});

	it('offers the Engagement Request as an action naming her', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Start new work with Ada' }))
			.toHaveAttribute('href', '/practices/practice-1/clients/client-1/engagement-requests/new');
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
			'/api/practices/practice-1/engagement-requests/request-mendoza-riquelme-postpartum/withdraw',
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
