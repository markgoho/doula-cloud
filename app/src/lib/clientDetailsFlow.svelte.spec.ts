import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ClientDetailsFlow, clientDetails } from './clientDetailsFlow.svelte.js';
import { jsonResponse } from './testResponse.js';

const onFile = {
	id: 'client-1',
	givenName: 'Anne-Marie',
	familyName: 'Ochieng',
	preferredName: '',
	email: 'anne@example.com',
	phone: '',
	addressLine1: '',
	addressLine2: '',
	addressLocality: '',
	addressRegion: '',
	addressPostalCode: '',
	dateOfBirth: '',
	fieldValues: { referral: 'A sister' },
	resolvedFields: [],
	engagements: [],
	history: []
};

function fetcherFor(body: unknown = onFile, status = 200) {
	return vi.fn(async () => jsonResponse(body, status));
}

beforeEach(() => {
	sessionStorage.clear();
});

describe('ClientDetailsFlow', () => {
	it('is idle until it is asked to load', () => {
		expect(new ClientDetailsFlow().status).toBe('idle');
	});

	it('starts each question from the record on file', async () => {
		const flow = new ClientDetailsFlow();

		await flow.load(fetcherFor(), 'p1', 'client-1');

		expect(flow.status).toBe('ready');
		expect(flow.draft.answers.email).toBe('anne@example.com');
		expect(flow.draft.answers.fieldValues).toEqual({ referral: 'A sister' });
	});

	it('does not ask again for a Client it has already read', async () => {
		const flow = new ClientDetailsFlow();
		const fetcher = fetcherFor();

		await flow.load(fetcher, 'p1', 'client-1');
		await flow.load(fetcher, 'p1', 'client-1');

		expect(fetcher).toHaveBeenCalledTimes(1);
	});

	// A reload part of the way through keeps what was typed: the draft is
	// mirrored, and a mirrored draft is not seeded over.
	it('keeps what was typed across a page load', async () => {
		const first = new ClientDetailsFlow();
		await first.load(fetcherFor(), 'p1', 'client-1');
		first.draft.update({ phone: '585 555 0199' });

		const second = new ClientDetailsFlow();
		await second.load(fetcherFor(), 'p1', 'client-1');

		expect(second.draft.answers.phone).toBe('585 555 0199');
	});

	// The door forgets an earlier visit, so a second visit starts from
	// the record on file rather than from what was left half-typed.
	it('starts again from the record on file once the door opens it', async () => {
		const flow = new ClientDetailsFlow();
		await flow.load(fetcherFor(), 'p1', 'client-1');
		flow.draft.update({ phone: '585 555 0199' });

		flow.open('client-1', 'engagement-1');
		await flow.load(fetcherFor(), 'p1', 'client-1');

		expect(flow.draft.answers.phone).toBe('');
		expect(flow.engagementId).toBe('engagement-1');
	});

	// A merged row carries only its id and the survivor's (detail.go).
	it('opens no draft for a merged record', async () => {
		const flow = new ClientDetailsFlow();

		await flow.load(fetcherFor({ id: 'client-1', mergedInto: 'client-2' }), 'p1', 'client-1');

		expect(flow.status).toBe('ready');
		expect(flow.record?.mergedInto).toBe('client-2');
		expect(flow.draft.answers.givenName).toBe('');
	});

	it('reports a record it could not read', async () => {
		const flow = new ClientDetailsFlow();

		await flow.load(fetcherFor({ error: 'client not found' }, 404), 'p1', 'client-1');

		expect(flow.status).toBe('error');
		expect(flow.loadError).not.toBe('');
	});

	it('reports a failure that is not an error object in words', async () => {
		const flow = new ClientDetailsFlow();

		await flow.load(vi.fn(() => Promise.reject('offline')), 'p1', 'client-1');

		expect(flow.loadError).toBe('Failed to load Client');
	});

	it('is one shared instance', () => {
		expect(clientDetails).toBeInstanceOf(ClientDetailsFlow);
	});
});
