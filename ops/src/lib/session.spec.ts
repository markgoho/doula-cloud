import { describe, expect, it, vi } from 'vitest';
import { endSession, exchangeToken, readSession, SERVICE_PROBLEM } from './session.ts';

function answering(status: number, body?: unknown): typeof fetch {
	const text = body === undefined ? '' : (typeof body === 'string' ? body : JSON.stringify(body));
	return vi.fn(async () => new Response(status === 204 ? undefined : text, { status }));
}

describe('exchangeToken', () => {
	it('sends the token as a Bearer credential and names the operator a session was minted for', async () => {
		const fetcher = answering(200, { name: 'Mark Goho', email: 'mark@doula.cloud' });

		const result = await exchangeToken('id-token', fetcher);

		expect(result).toEqual({ kind: 'signed-in', operator: { name: 'Mark Goho', email: 'mark@doula.cloud' } });
		expect(fetcher).toHaveBeenCalledWith('/api/session', {
			method: 'POST',
			credentials: 'include',
			headers: { Authorization: 'Bearer id-token' }
		});
	});

	it('sends a login with no authenticator app to enroll', async () => {
		const result = await exchangeToken('id-token', answering(403, { code: 'MFA_REQUIRED', message: 'Set up' }));

		expect(result).toEqual({ kind: 'enroll' });
	});

	it("shows any other refusal in doula-ops's own words", async () => {
		const result = await exchangeToken(
			'id-token',
			answering(403, { code: 'FORBIDDEN', message: 'Cross-site request rejected.' })
		);

		expect(result).toEqual({ kind: 'refused', message: 'Cross-site request rejected.' });
	});

	it('shows a Staff token refused as doula-ops words it', async () => {
		const result = await exchangeToken(
			'id-token',
			answering(401, { code: 'UNAUTHORIZED', message: 'Sign in with an operator login.' })
		);

		expect(result).toEqual({ kind: 'refused', message: 'Sign in with an operator login.' });
	});

	it('shows a server failure as a problem with the service', async () => {
		expect(await exchangeToken('t', answering(500, { message: 'internal error' }))).toEqual({
			kind: 'refused',
			message: SERVICE_PROBLEM
		});
	});

	it('shows a refusal with no message, or no JSON, as a problem with the service', async () => {
		expect(await exchangeToken('t', answering(401, {}))).toEqual({ kind: 'refused', message: SERVICE_PROBLEM });
		expect(await exchangeToken('t', answering(401, 'not json'))).toEqual({ kind: 'refused', message: SERVICE_PROBLEM });
	});
});

describe('readSession', () => {
	it('names the signed-in operator', async () => {
		const operator = { name: 'Mark Goho', email: 'mark@doula.cloud' };

		expect(await readSession(answering(200, operator))).toEqual(operator);
	});

	it('answers nothing once the session has ended', async () => {
		expect(await readSession(answering(401, { code: 'UNAUTHORIZED' }))).toBeUndefined();
	});
});

describe('endSession', () => {
	it('ends the session', async () => {
		const fetcher = answering(204);

		await endSession(fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/session', { method: 'DELETE', credentials: 'include' });
	});

	it('treats a session that already ended as ended', async () => {
		await expect(endSession(answering(401, {}))).resolves.toBeUndefined();
	});

	it('fails on a server failure', async () => {
		await expect(endSession(answering(500, {}))).rejects.toThrow('sign out: 500');
	});
});
