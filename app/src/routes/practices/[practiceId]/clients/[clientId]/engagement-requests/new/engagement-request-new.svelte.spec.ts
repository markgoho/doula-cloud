import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientDetail, EngagementSummary } from '#lib/clientDetail.js';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1', clientId: 'client-1' } }
}));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const baseDetail: ClientDetail = {
	id: 'client-1',
	givenName: 'Ada',
	familyName: 'Lovelace',
	preferredName: '',
	email: 'ada@example.com',
	phone: '',
	addressLine1: '',
	addressLine2: '',
	addressLocality: '',
	addressRegion: '',
	addressPostalCode: '',
	dateOfBirth: '',
	resolvedFields: [],
	engagements: [],
	history: []
};

const liveEngagement: EngagementSummary = {
	engagementId: 'engagement-0',
	kind: 'postpartum',
	status: 'active',
	createdAt: '2026-01-01T00:00:00Z'
};

const draftKey = 'engagement-request-draft:client-1';

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	sessionStorage.clear();
});

interface MockOptions {
	detail?: ClientDetail;
	roles?: string[];
	balance?: number;
	requestOutcome?: unknown;
	requestStatus?: number;
}

function mockFetches({
	detail = baseDetail,
	roles = ['doula'],
	balance = 3,
	requestOutcome,
	requestStatus = 201
}: MockOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.endsWith('/session')) return Promise.resolve(jsonResponse({ roles }));
		if (path.endsWith('/billing')) return Promise.resolve(jsonResponse({ balance, ledger: { items: [], hasMore: false } }));
		if (path.endsWith('/engagement-requests')) {
			return Promise.resolve(jsonResponse(requestOutcome ?? { requestId: 'request-1', state: 'pending' }, requestStatus));
		}
		return Promise.resolve(jsonResponse(detail));
	});
}

async function setup(options: MockOptions = {}) {
	mockFetches(options);
	await render(Page, {});
	await expect.element(testPage.getByRole('heading', { level: 1 })).toBeVisible();
	return options;
}

describe('the Engagement Request screen', () => {
	it('shows a Doula the "Ask to" phrasing and no Credit preview', async () => {
		await setup({ roles: ['doula'] });

		await expect.element(testPage.getByRole('button', { name: 'Ask to start work with Ada' })).toBeVisible();
		await expect.element(testPage.getByText('Credit cost')).not.toBeInTheDocument();
	});

	it('shows an Owner the "Start work with" phrasing and the Credit cost and balance after', async () => {
		await setup({ roles: ['owner'], balance: 3 });

		await expect.element(testPage.getByRole('button', { name: 'Start work with Ada' })).toBeVisible();
		await expect.element(testPage.getByText('Credit cost')).toBeVisible();
		await expect.element(testPage.getByText('1 credit')).toBeVisible();
		await expect.element(testPage.getByText('2', { exact: true })).toBeVisible();
	});

	it('shows an Admin the same "Start work with" phrasing', async () => {
		await setup({ roles: ['admin'], balance: 5 });

		await expect.element(testPage.getByRole('button', { name: 'Start work with Ada' })).toBeVisible();
	});

	it('refuses a submit with no kind or due date chosen, client-side, before any request', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Ask to start work with Ada' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Select whether this is birth or postpartum work' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Enter the due date' })).toBeVisible();
		// The load calls (detail, session) are the only requests made -- the
		// refusal never reached the network.
		expect(apiFetchWithSession).not.toHaveBeenCalledWith(
			expect.stringContaining('/engagement-requests'),
			expect.anything()
		);
	});

	it('warns on a second live Engagement without blocking the submit', async () => {
		await setup({ detail: { ...baseDetail, engagements: [liveEngagement] } });

		await expect.element(testPage.getByText('already has a live Engagement', { exact: false })).toBeVisible();

		await testPage.getByLabelText('Birth').click();
		await testPage.getByLabelText('Due date').fill('2027-03-01');
		await testPage.getByRole('button', { name: 'Ask to start work with Ada' }).click();

		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
	});

	it('lands back on the Client detail hub on a successful submit', async () => {
		await setup();

		await testPage.getByLabelText('Postpartum').click();
		await testPage.getByLabelText('Due date').fill('2027-03-01');
		await testPage.getByLabelText('Note').fill('Referred by the hospital');
		await testPage.getByRole('button', { name: 'Ask to start work with Ada' }).click();

		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith('/practices/practice-1/clients/client-1');

		const body: { kind: string; dueDate: string; note: string } = JSON.parse(
			(apiFetchWithSession.mock.calls.find(([path]) => (path as string).endsWith('/engagement-requests'))![1] as RequestInit)
				.body as string
		);
		expect(body).toEqual({ kind: 'postpartum', dueDate: '2027-03-01', note: 'Referred by the hospital' });
	});

	it('surfaces an empty balance with an inline Buy credits path, leaving the typed form in place', async () => {
		await setup({ roles: ['owner'], requestStatus: 402 });

		await testPage.getByLabelText('Birth').click();
		await testPage.getByLabelText('Due date').fill('2027-03-01');
		await testPage.getByRole('button', { name: 'Start work with Ada' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Buy credits' })).toBeVisible();
		// Nothing was lost: the same mounted form still holds what she typed.
		await expect.element(testPage.getByLabelText('Due date')).toHaveValue('2027-03-01');
		await expect.element(testPage.getByLabelText('Birth')).toBeChecked();
		expect(goto).not.toHaveBeenCalled();
		expect(sessionStorage.getItem(draftKey)).toContain('2027-03-01');
	});

	it('restores a saved draft on mount, the far side of the Buy Credits round trip', async () => {
		sessionStorage.setItem(draftKey, JSON.stringify({ kind: 'postpartum', dueDate: '2027-06-15', note: 'Twins' }));

		await setup();

		await expect.element(testPage.getByLabelText('Postpartum')).toBeChecked();
		await expect.element(testPage.getByLabelText('Due date')).toHaveValue('2027-06-15');
		await expect.element(testPage.getByLabelText('Note')).toHaveValue('Twins');
	});

	it('surfaces the endpoint refusal as an error rather than a silent failure', async () => {
		await setup({ requestOutcome: 'a pending request for this client and kind already exists', requestStatus: 409 });

		await testPage.getByLabelText('Birth').click();
		await testPage.getByLabelText('Due date').fill('2027-03-01');
		await testPage.getByRole('button', { name: 'Ask to start work with Ada' }).click();

		await expect
			.element(testPage.getByText('a pending request for this client and kind already exists'))
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('exposes the kind options, due date, note, submit and Cancel as reachable, labelled controls', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Birth')).toBeVisible();
		await expect.element(testPage.getByLabelText('Postpartum')).toBeVisible();
		await expect.element(testPage.getByLabelText('Due date')).toBeVisible();
		await expect.element(testPage.getByLabelText('Note')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Ask to start work with Ada' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Cancel' }))
			.toHaveAttribute('href', '/practices/practice-1/clients/client-1');
	});
});
