import { describe, expect, it, vi } from 'vitest';
import {
	historyRows,
	loadClientDetail,
	pendingRequests,
	resolvedFieldGroups,
	resolvedFieldValueText,
	type HistoryEntry
} from './clientDetail.js';
import { jsonResponse as response } from './testResponse.js';

describe('loadClientDetail', () => {
	it('fetches the client detail path and returns the decoded record', async () => {
		const detail = { id: 'client-1', givenName: 'Ada', resolvedFields: [], engagements: [], history: [] };
		const fetcher = vi.fn().mockResolvedValue(response(detail));

		const result = await loadClientDetail(fetcher, 'practice-1', 'client-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/client-1');
		expect(result).toEqual(detail);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('client not found', 404));

		await expect(loadClientDetail(fetcher, 'practice-1', 'client-1')).rejects.toThrow('client not found');
	});
});

describe('resolvedFieldValueText', () => {
	it('renders an unset value as an empty string', () => {
		expect(resolvedFieldValueText({ fieldId: 'f1', label: 'Notes', type: 'short_text' })).toBe('');
	});

	it('renders checkbox true/false as Yes/No', () => {
		expect(resolvedFieldValueText({ fieldId: 'f1', label: 'Consented', type: 'checkbox', value: true })).toBe('Yes');
		expect(resolvedFieldValueText({ fieldId: 'f1', label: 'Consented', type: 'checkbox', value: false })).toBe('No');
	});

	it('joins a multi_select array', () => {
		expect(
			resolvedFieldValueText({ fieldId: 'f1', label: 'Diets', type: 'multi_select', value: ['Vegan', 'Gluten-free'] })
		).toBe('Vegan, Gluten-free');
	});

	it('renders a non-array multi_select value as plain text, defensively', () => {
		expect(resolvedFieldValueText({ fieldId: 'f1', label: 'Diets', type: 'multi_select', value: 'Vegan' })).toBe(
			'Vegan'
		);
	});

	it('renders a short_text value as plain text', () => {
		expect(resolvedFieldValueText({ fieldId: 'f1', label: 'Notes', type: 'short_text', value: 'Prefers texts' })).toBe(
			'Prefers texts'
		);
	});
});

function header(label: string) {
	return { fieldId: label, label, type: 'section_header' };
}

function text(label: string) {
	return { fieldId: label, label, type: 'short_text' };
}

describe('resolvedFieldGroups', () => {
	it('groups the fields between two headers under the first, with the fields before any header untitled', () => {
		expect(
			resolvedFieldGroups([text('Pets'), header('Health'), text('Allergies'), text('Medications'), header('Birth'), text('Place')])
		).toEqual([
			{ fields: [text('Pets')] },
			{ heading: 'Health', fields: [text('Allergies'), text('Medications')] },
			{ heading: 'Birth', fields: [text('Place')] }
		]);
	});

	it('drops a header with no field under it', () => {
		expect(resolvedFieldGroups([header('Empty'), header('Health'), text('Allergies'), header('Trailing')])).toEqual([
			{ heading: 'Health', fields: [text('Allergies')] }
		]);
	});
});

describe('pendingRequests', () => {
	it('returns only the engagement_request entries in the pending state', () => {
		const pending = {
			requestId: 'r1',
			kind: 'birth',
			state: 'pending',
			requestedBy: 's1',
			requestedByName: 'Jamie Doula',
			requestedAt: '2026-01-02T00:00:00Z'
		} as const;
		const history: HistoryEntry[] = [
			{
				type: 'engagement_request',
				at: '2026-01-02T00:00:00Z',
				engagementRequest: pending
			},
			{
				type: 'engagement_request',
				at: '2026-01-01T00:00:00Z',
				engagementRequest: {
					requestId: 'r2',
					kind: 'postpartum',
					state: 'refused',
					requestedBy: 's1',
					requestedByName: 'Jamie Doula',
					requestedAt: '2026-01-01T00:00:00Z'
				}
			},
			{
				type: 'client_event',
				at: '2026-01-01T00:00:00Z',
				clientEvent: { eventType: 'created', diff: {}, actorKind: 'staff', actorName: 'Sam Owner', createdAt: '2026-01-01T00:00:00Z' }
			}
		];

		expect(pendingRequests(history)).toEqual([pending]);
	});

	it('returns an empty array when nothing is pending', () => {
		expect(pendingRequests([])).toEqual([]);
	});
});

/*
 * #2280: asking for an Engagement and deciding it are two acts, each with
 * its own who and when, so a decided Request is two rows of the History.
 */
describe('historyRows', () => {
	const asked = {
		requestId: 'r1',
		kind: 'postpartum',
		requestedBy: 'staff-priya',
		requestedByName: 'Priya Raman',
		requestedAt: '2026-10-28T22:58:21Z'
	} as const;
	const created: HistoryEntry = {
		type: 'client_event',
		at: '2026-10-28T22:59:30Z',
		clientEvent: { eventType: 'updated', diff: {}, actorKind: 'staff', actorName: 'Sam Owner', createdAt: '2026-10-28T22:59:30Z' }
	};

	it('splits a Request asked by one Staff member and approved by another into the asking and the deciding, newest first', () => {
		const approved = {
			...asked,
			state: 'approved',
			decidedBy: 'staff-renata',
			decidedByName: 'Renata Alvarez',
			decidedAt: '2026-10-28T22:58:59Z'
		};
		const history: HistoryEntry[] = [
			created,
			{ type: 'engagement_request', at: asked.requestedAt, engagementRequest: approved }
		];

		expect(historyRows(history)).toEqual([
			created,
			{
				type: 'engagement_request',
				act: 'decided',
				at: '2026-10-28T22:58:59Z',
				who: 'Renata Alvarez',
				engagementRequest: approved
			},
			{
				type: 'engagement_request',
				act: 'asked',
				at: '2026-10-28T22:58:21Z',
				who: 'Priya Raman',
				engagementRequest: approved
			}
		]);
	});

	it('places a decision after the entries written between the asking and it', () => {
		const refused = {
			...asked,
			state: 'refused',
			decidedBy: 'staff-renata',
			decidedByName: 'Renata Alvarez',
			decidedAt: '2026-10-28T23:10:00Z'
		};
		const rows = historyRows([created, { type: 'engagement_request', at: asked.requestedAt, engagementRequest: refused }]);

		expect(rows.map((row) => row.at)).toEqual(['2026-10-28T23:10:00Z', created.at, asked.requestedAt]);
	});

	it('keeps a pending Request as the one row of its asking', () => {
		const pending = { ...asked, state: 'pending' };
		expect(historyRows([{ type: 'engagement_request', at: asked.requestedAt, engagementRequest: pending }])).toEqual([
			{ type: 'engagement_request', act: 'asked', at: asked.requestedAt, who: 'Priya Raman', engagementRequest: pending }
		]);
	});

	it('keeps a decided Request that carries no decision as the one row of its asking', () => {
		const undated = { ...asked, state: 'withdrawn' };
		expect(historyRows([{ type: 'engagement_request', at: asked.requestedAt, engagementRequest: undated }])).toHaveLength(1);
		const unnamed = { ...undated, decidedAt: '2026-10-28T23:00:00Z' };
		expect(historyRows([{ type: 'engagement_request', at: asked.requestedAt, engagementRequest: unnamed }])).toHaveLength(1);
	});
});
