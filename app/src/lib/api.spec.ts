import { describe, it, expect, vi } from 'vitest';
import { apiBaseURL, apiFetch } from './api';

describe('apiBaseURL', () => {
	it('defaults to same-origin (empty string) when unset', () => {
		expect(apiBaseURL()).toBe('');
	});
});

describe('apiFetch', () => {
	it('attaches the bearer token and merges caller headers', async () => {
		const fetchMock = vi.fn(async () => new Response(null, { status: 200 }));
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
