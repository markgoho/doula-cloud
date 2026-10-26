import { describe, expect, it, vi } from 'vitest';
import {
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
