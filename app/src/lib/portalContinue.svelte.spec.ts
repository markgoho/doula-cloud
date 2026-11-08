import { beforeEach, describe, expect, it, vi } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { landInPortal, postContinue } from './portalContinue.js';

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('./api.js', () => ({ apiFetchWithSession, apiBaseURL: () => 'https://api.example.test' }));

beforeEach(() => {
	goto.mockReset();
	apiFetchWithSession.mockReset();
});

describe('postContinue', () => {
	it('posts the body with the cookie, and the confirmation only on the second press', async () => {
		const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{}'));

		await postContinue('/api/portal/magic-link', { token: 't' }, false);
		await postContinue('/api/portal/magic-link', { token: 't' }, true);

		const [first, second] = fetchSpy.mock.calls;
		expect(first![0]).toBe('https://api.example.test/api/portal/magic-link');
		expect(first![1]).toMatchObject({ method: 'POST', credentials: 'include', body: '{"token":"t"}' });
		expect(first![1]!.headers).not.toHaveProperty('X-Confirmed');
		expect(second![1]!.headers).toHaveProperty('X-Confirmed', 'true');
		fetchSpy.mockRestore();
	});
});

describe('landInPortal', () => {
	it('goes to the only Engagement', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({ engagements: [{ engagementId: 'e1', practiceName: 'P', status: 'active', createdAt: '2026-01-01T00:00:00Z' }] })
		);

		expect(await landInPortal()).toBeUndefined();

		expect(goto).toHaveBeenCalledWith('/portal/engagements/e1');
	});

	it('goes to the app root for none or several', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ engagements: [] }));

		await landInPortal();

		expect(goto).toHaveBeenCalledWith('/');
	});

	it('returns what the BFF refused when the session cannot be read', async () => {
		apiFetchWithSession.mockResolvedValue(new Response('', { status: 500 }));

		const errors = await landInPortal();

		expect(errors).toHaveLength(1);
		expect(goto).not.toHaveBeenCalled();
	});
});
