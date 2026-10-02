import { afterEach, describe, expect, it, vi } from 'vitest';

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

/*
 * Every request the two loads make, answered by path. A real `Response`
 * each time: `isMFARequired` reads a `.clone()` of the body.
 */
function respondWith(answers: Record<string, { status: number; body: unknown }>) {
	const fetchMock = vi.fn(async (path: string) => {
		const answer = answers[path] ?? { status: 500, body: { message: `unexpected request: ${path}` } };
		return Response.json(answer.body, { status: answer.status });
	});
	vi.stubGlobal('fetch', fetchMock);
	return fetchMock;
}

afterEach(() => {
	vi.unstubAllGlobals();
});

const listUrl = new URL('https://example.test/feedback');
const pieceUrl = new URL('https://example.test/feedback/feedback-1');
const emptyPage = { items: [], hasMore: false };
const mfaRequired = { code: 'MFA_REQUIRED', message: 'this page requires a second sign-in factor' };
const notFound = { code: 'NOT_FOUND', message: 'not found' };

async function loadList() {
	const { load } = await import('./+page.js');
	return load({ params: {}, url: listUrl } as Parameters<typeof load>[0]);
}

async function loadPiece() {
	const { load } = await import('./[feedbackId]/+page.js');
	return load({ params: { feedbackId: 'feedback-1' }, url: pieceUrl } as Parameters<typeof load>[0]);
}

describe('feedback/+page.ts load', () => {
	it('reads the unopened list and the whole list, and returns both first pages', async () => {
		const unopened = { items: [{ id: 'feedback-2' }], hasMore: false };
		const fetchMock = respondWith({
			'/api/staff/feedback?issue=unopened': { status: 200, body: unopened },
			'/api/staff/feedback': { status: 200, body: emptyPage }
		});

		await expect(loadList()).resolves.toEqual({ unopened, all: emptyPage });
		expect(fetchMock).toHaveBeenCalledWith('/api/staff/feedback', expect.objectContaining({ credentials: 'include' }));
	});

	it('sends a person with no session to the login screen', async () => {
		respondWith({
			'/api/staff/feedback?issue=unopened': { status: 401, body: {} },
			'/api/staff/feedback': { status: 401, body: {} }
		});

		await expect(loadList()).rejects.toMatchObject({ status: 303, location: '/login?sessionEnded=true' });
	});

	it('sends the founder with no second factor to TOTP enrollment, and back here after it', async () => {
		respondWith({
			'/api/staff/feedback?issue=unopened': { status: 403, body: mfaRequired },
			'/api/staff/feedback': { status: 403, body: mfaRequired }
		});

		await expect(loadList()).rejects.toMatchObject({ status: 303, location: '/mfa/enroll?returnTo=%2Ffeedback' });
	});

	// Anybody but the founder: the BFF answers 404, and the page answers
	// the same "not found" a URL with no route behind it does.
	it('answers a Staff member who is not the founder with a 404', async () => {
		respondWith({
			'/api/staff/feedback?issue=unopened': { status: 404, body: notFound },
			'/api/staff/feedback': { status: 404, body: notFound }
		});

		await expect(loadList()).rejects.toMatchObject({ status: 404 });
	});

	it('throws with the response status on any other failure', async () => {
		respondWith({
			'/api/staff/feedback?issue=unopened': { status: 200, body: emptyPage },
			'/api/staff/feedback': { status: 500, body: { message: 'internal error' } }
		});

		await expect(loadList()).rejects.toMatchObject({ status: 500 });
	});
});

describe('feedback/[feedbackId]/+page.ts load', () => {
	it('reads the one piece the URL names', async () => {
		const piece = { id: 'feedback-1', text: 'The invoice total is wrong.' };
		const fetchMock = respondWith({ '/api/staff/feedback/feedback-1': { status: 200, body: piece } });

		await expect(loadPiece()).resolves.toEqual({ piece });
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it('sends the founder with no second factor to TOTP enrollment, and back to this piece after it', async () => {
		respondWith({ '/api/staff/feedback/feedback-1': { status: 403, body: mfaRequired } });

		await expect(loadPiece()).rejects.toMatchObject({
			status: 303,
			location: '/mfa/enroll?returnTo=%2Ffeedback%2Ffeedback-1'
		});
	});

	// A piece that was erased, or is past its retention, is gone.
	it('answers a piece that is not there with a 404', async () => {
		respondWith({ '/api/staff/feedback/feedback-1': { status: 404, body: notFound } });

		await expect(loadPiece()).rejects.toMatchObject({ status: 404 });
	});
});
