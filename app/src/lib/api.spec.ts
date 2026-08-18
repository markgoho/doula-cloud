import { describe, it, expect, vi } from 'vitest';
import { apiBaseURL, apiErrorMessage, apiFetch } from './api';

describe('apiBaseURL', () => {
	it('defaults to same-origin (empty string) when unset', () => {
		expect(apiBaseURL()).toBe('');
	});
});

describe('apiFetch', () => {
	it('attaches the bearer token and merges caller headers', async () => {
		const fetchMock = vi.fn(async () => new Response(undefined, { status: 200 }));
		vi.stubGlobal('fetch', fetchMock);

		await apiFetch('/api/staff/session', 'tok-123', { headers: { 'X-Test': 'yes' } });

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/staff/session',
			expect.objectContaining({
				headers: { 'X-Test': 'yes', Authorization: 'Bearer tok-123' }
			})
		);

		vi.unstubAllGlobals();
	});
});

describe('apiErrorMessage', () => {
	it('extracts message from an APIError JSON body', async () => {
		const response = Response.json({ code: 'CONFLICT', message: 'already invited' });
		await expect(apiErrorMessage(response)).resolves.toBe('already invited');
	});

	it('returns the raw body for a plain-text error', async () => {
		const response = new Response('only a Practice Owner can do that');
		await expect(apiErrorMessage(response)).resolves.toBe('only a Practice Owner can do that');
	});

	it('returns the raw body for JSON with no message field', async () => {
		const response = Response.json({ code: 'CONFLICT' });
		await expect(apiErrorMessage(response)).resolves.toBe('{"code":"CONFLICT"}');
	});
});
