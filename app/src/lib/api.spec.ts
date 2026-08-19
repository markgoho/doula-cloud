import { describe, it, expect, vi, afterEach } from 'vitest';

const goto = vi.fn();
vi.mock('$app/navigation', () => ({ goto }));

const signOut = vi.fn();
vi.mock('firebase/auth', () => ({ signOut }));

const getFirebaseAuth = vi.fn(() => 'the-auth-instance');
vi.mock('./firebase.js', () => ({ getFirebaseAuth }));

const { apiBaseURL, apiErrorMessage, apiFetchWithSession } = await import('./api');

describe('apiBaseURL', () => {
	it('defaults to same-origin (empty string) when unset', () => {
		expect(apiBaseURL()).toBe('');
	});
});

describe('apiFetchWithSession', () => {
	interface SetupOptions {
		status?: number;
		pathname?: string;
	}

	function setup({ status = 200, pathname = '/practices/prac-1' }: SetupOptions = {}) {
		goto.mockClear();
		signOut.mockClear();
		signOut.mockImplementation(async () => {});
		const fetchMock = vi.fn(async () => new Response(undefined, { status }));
		vi.stubGlobal('fetch', fetchMock);
		vi.stubGlobal('location', { pathname });
		return { fetchMock };
	}

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('sends credentials and returns the response on a non-401', async () => {
		const { fetchMock } = setup({ status: 200 });

		const response = await apiFetchWithSession('/api/practices/prac-1/session', {
			headers: { 'X-Test': 'yes' }
		});

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/practices/prac-1/session',
			expect.objectContaining({ headers: { 'X-Test': 'yes' }, credentials: 'include' })
		);
		expect(response.status).toBe(200);
		expect(goto).not.toHaveBeenCalled();
		expect(signOut).not.toHaveBeenCalled();
	});

	it('clears the signed-in Identity Platform user before redirecting on a 401', async () => {
		let resolveSignOut!: () => void;
		setup({ status: 401 });
		signOut.mockImplementation(() => new Promise<void>((resolve) => (resolveSignOut = resolve)));

		const pending = apiFetchWithSession('/api/practices/prac-1/session');
		await vi.waitFor(() => expect(signOut).toHaveBeenCalledWith('the-auth-instance'));
		expect(goto).not.toHaveBeenCalled();

		resolveSignOut();
		await pending;

		expect(goto).toHaveBeenCalled();
	});

	it('routes a 401 on a Staff route to the Staff login, carrying the session-ended explanation', async () => {
		setup({ status: 401, pathname: '/practices/prac-1' });

		await apiFetchWithSession('/api/practices/prac-1/session');

		expect(goto).toHaveBeenCalledWith('/login?sessionEnded=true');
	});

	it('routes a 401 on a portal route to the portal login, carrying the session-ended explanation', async () => {
		setup({ status: 401, pathname: '/portal/engagements/eng-1' });

		await apiFetchWithSession('/api/portal/engagements/eng-1/session');

		expect(goto).toHaveBeenCalledWith('/portal/login?sessionEnded=true');
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
