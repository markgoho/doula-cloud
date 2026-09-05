import { describe, it, expect, vi, afterEach } from 'vitest';

const goto = vi.fn();
vi.mock('$app/navigation', () => ({ goto }));

const signOut = vi.fn();
vi.mock('firebase/auth', () => ({ signOut }));

const getFirebaseAuth = vi.fn(() => 'the-auth-instance');
vi.mock('./firebase.js', () => ({ getFirebaseAuth }));

const { apiBaseURL, apiErrorMessage, apiFetch, apiFetchWithSession, probeSession } =
	await import('./api');

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
		// mockReset, not mockClear: a prior test in this block can leave a
		// pending-promise implementation behind (the concurrent-refusal test
		// below controls goto's own resolution), which would otherwise hang
		// the next test that never awaits it explicitly.
		goto.mockReset();
		signOut.mockClear();
		signOut.mockImplementation(async () => {});
		const fetchMock = vi.fn(async () => new Response(undefined, { status }));
		vi.stubGlobal('fetch', fetchMock);
		const location = { pathname };
		vi.stubGlobal('location', location);
		return { fetchMock, location };
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

	/*
	 * #606: a 403 carrying `{code: "MFA_REQUIRED"}` is a live, valid
	 * session that may not enter this Practice -- not an ended session, so
	 * it never touches `signOut` -- and it routes to enrolment rather than
	 * to a login screen.
	 */
	it('routes a 403 carrying MFA_REQUIRED to enrolment, carrying returnTo', async () => {
		setup({ pathname: '/practices/prac-1/invoices' });
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => Response.json({ code: 'MFA_REQUIRED' }, { status: 403 }))
		);

		await apiFetchWithSession('/api/practices/prac-1/invoices');

		expect(goto).toHaveBeenCalledWith('/mfa/enroll?returnTo=%2Fpractices%2Fprac-1%2Finvoices');
		expect(signOut).not.toHaveBeenCalled();
	});

	it('leaves every other 403 alone -- a role or Membership refusal is not this ticket’s', async () => {
		setup();
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => Response.json({ code: 'FORBIDDEN' }, { status: 403 }))
		);

		const response = await apiFetchWithSession('/api/practices/prac-1/invoices');

		expect(goto).not.toHaveBeenCalled();
		expect(response.status).toBe(403);
	});

	it('reads a 403 with no JSON body as anything other than MFA_REQUIRED', async () => {
		setup();
		vi.stubGlobal('fetch', vi.fn(async () => new Response('only an Owner can do that', { status: 403 })));

		await apiFetchWithSession('/api/practices/prac-1/invoices');

		expect(goto).not.toHaveBeenCalled();
	});

	/*
	 * practices/+layout.svelte and the practice landing page each fire
	 * several of these calls on one navigation -- confirmed as a real e2e
	 * failure, not a theoretical one: a second refusal's own redirect read
	 * location.pathname after the first had already landed on /mfa/enroll,
	 * clobbering returnTo with that path instead of the Practice she was
	 * trying to reach.
	 */
	it('lets a second concurrent MFA_REQUIRED refusal fall through instead of re-redirecting', async () => {
		setup({ pathname: '/practices/prac-1' });
		let resolveGoto!: () => void;
		goto.mockImplementation(() => new Promise<void>((resolve) => (resolveGoto = resolve)));
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => Response.json({ code: 'MFA_REQUIRED' }, { status: 403 }))
		);

		const first = apiFetchWithSession('/api/practices/prac-1/session');
		await vi.waitFor(() => expect(goto).toHaveBeenCalledTimes(1));

		// Fired while the first redirect is still in flight -- exactly the
		// concurrent-call shape the race needs.
		const second = await apiFetchWithSession('/api/practices/prac-1/invoices');

		expect(goto).toHaveBeenCalledTimes(1);
		expect(goto).toHaveBeenCalledWith('/mfa/enroll?returnTo=%2Fpractices%2Fprac-1');
		expect(second.status).toBe(403);

		resolveGoto();
		await first;
	});

	/*
	 * The actual shape of the e2e failure: practices/[practiceId]/+page.svelte's
	 * onMount awaits three such calls one after another, not concurrently --
	 * `redirecting` alone resets as soon as the first goto's own promise
	 * settles, which is *before* the later, sequential calls in that same
	 * onMount discover their own refusal. By then location.pathname already
	 * reads /mfa/enroll, so the later call must recognise it is already
	 * there rather than reading that as the destination to send returnTo to.
	 */
	it('lets a later, sequential MFA_REQUIRED refusal fall through once an earlier one has already landed on /mfa/enroll', async () => {
		const { location } = setup({ pathname: '/practices/prac-1' });
		goto.mockImplementation(async () => {
			location.pathname = '/mfa/enroll';
		});
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => Response.json({ code: 'MFA_REQUIRED' }, { status: 403 }))
		);

		const first = await apiFetchWithSession('/api/practices/prac-1/session');
		const second = await apiFetchWithSession('/api/practices/prac-1/invoices');

		expect(goto).toHaveBeenCalledTimes(1);
		expect(goto).toHaveBeenCalledWith('/mfa/enroll?returnTo=%2Fpractices%2Fprac-1');
		expect(first.status).toBe(403);
		expect(second.status).toBe(403);
	});

	it('redirects again on a later, unrelated refusal once the first redirect has finished', async () => {
		setup({ pathname: '/practices/prac-1' });
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => Response.json({ code: 'MFA_REQUIRED' }, { status: 403 }))
		);

		await apiFetchWithSession('/api/practices/prac-1/session');
		await apiFetchWithSession('/api/practices/prac-1/session');

		expect(goto).toHaveBeenCalledTimes(2);
	});
});

describe('apiFetch', () => {
	it('hands a 401 straight back instead of sending the caller to the login screen', async () => {
		goto.mockClear();
		signOut.mockClear();
		vi.stubGlobal('fetch', vi.fn(async () => new Response(undefined, { status: 401 })));

		const response = await apiFetch('/api/practices/prac-1/push-subscriptions', {
			method: 'DELETE'
		});

		expect(response.status).toBe(401);
		expect(goto).not.toHaveBeenCalled();
		expect(signOut).not.toHaveBeenCalled();
		vi.unstubAllGlobals();
	});
});

describe('probeSession', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('returns the parsed body for a live session', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => Response.json({ memberships: [] }))
		);

		await expect(probeSession('/api/staff/session')).resolves.toEqual({ memberships: [] });
	});

	it('reads a non-OK response as no session of this kind, not a failure', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => new Response('no session', { status: 401 }))
		);

		await expect(probeSession('/api/staff/session')).resolves.toBeUndefined();
	});

	it('reads a thrown fetch the same way as no session', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => {
				throw new TypeError('Failed to fetch');
			})
		);

		await expect(probeSession('/api/staff/session')).resolves.toBeUndefined();
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
