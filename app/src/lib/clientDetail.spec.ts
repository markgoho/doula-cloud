import { describe, expect, it, vi } from 'vitest';
import {
	displayName,
	loadClientDetail,
	pendingRequests,
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

describe('displayName', () => {
	const base = {
		id: 'c1',
		givenName: 'Ada',
		familyName: 'Lovelace',
		preferredName: '',
		email: '',
		phone: '',
		addressLine1: '',
		addressLine2: '',
		addressLocality: '',
		addressRegion: '',
		addressPostalCode: '',
		dateOfBirth: ''
	};

	it('reads the preferred name when set', () => {
		expect(displayName({ ...base, preferredName: 'Ada' })).toBe('Ada');
	});

	it('falls back to given plus family name', () => {
		expect(displayName(base)).toBe('Ada Lovelace');
	});

	it('falls back to given name alone when family name is unset', () => {
		expect(displayName({ ...base, familyName: '' })).toBe('Ada');
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
				clientEvent: { eventType: 'created', diff: {}, actorKind: 'staff', createdAt: '2026-01-01T00:00:00Z' }
			}
		];

		expect(pendingRequests(history)).toEqual([pending]);
	});

	it('returns an empty array when nothing is pending', () => {
		expect(pendingRequests([])).toEqual([]);
	});
});
