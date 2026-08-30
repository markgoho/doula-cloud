import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientDetail } from '#lib/clientDetail.js';
import Page from './+page.svelte';

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

async function setup(overrides: Partial<ClientDetail> = {}) {
	apiFetchWithSession.mockResolvedValue(jsonResponse({ ...baseDetail, ...overrides }));
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
			resolvedFields: [
				{ fieldId: 'f1', label: 'Doula notes', type: 'short_text' },
				{ fieldId: 'f2', label: 'Pronouns', type: 'short_text', value: 'she/her' },
				{ fieldId: 'f3', label: 'Old field', type: 'short_text', value: 'kept value', note: 'No longer collected' }
			]
		});

		await expect.element(testPage.getByText('Doula notes')).toBeVisible();
		await expect.element(testPage.getByText('she/her')).toBeVisible();
		await expect.element(testPage.getByText('kept value (No longer collected)')).toBeVisible();
	});

	it("renders her Engagements identifying each one's kind and status", async () => {
		await setup({
			engagements: [{ engagementId: 'e1', kind: 'birth', status: 'active', createdAt: '2026-01-01T00:00:00Z' }]
		});

		await expect.element(testPage.getByRole('cell', { name: 'Birth' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'active' })).toBeVisible();
	});

	it('renders client_events and engagement_requests as one merged timeline', async () => {
		await setup({
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
		});

		await expect.element(testPage.getByRole('cell', { name: 'Birth Engagement requested' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Jamie Doula' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Record created' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Sam Admin' })).toBeVisible();
	});

	it('names a system-authored event "Doula Cloud", never "System" (ADR-0022)', async () => {
		await setup({
			history: [
				{
					type: 'client_event',
					at: '2026-01-01T00:00:00Z',
					clientEvent: { eventType: 'updated', diff: {}, actorKind: 'system', createdAt: '2026-01-01T00:00:00Z' }
				}
			]
		});

		await expect.element(testPage.getByRole('cell', { name: 'Doula Cloud' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'System', exact: true })).not.toBeInTheDocument();
	});

	it('shows a block naming who asked and when while a Request is pending', async () => {
		await setup({
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

	it('shows an error notice when the Client fails to load', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('client not found', 404));

		await render(Page, {});

		await expect.element(testPage.getByText('client not found')).toBeVisible();
	});
});
