import { describe, expect, it, vi } from 'vitest';
import { actKey, postAct, refusal, serviceProblem, stepUpRefusal } from './act.ts';
import { SERVICE_PROBLEM } from './session.ts';

describe('actKey', () => {
	it('names each act anew', () => {
		expect(actKey()).not.toBe(actKey());
	});
});

describe('postAct', () => {
	it('sends the body under the Idempotency-Key, with the session cookie', async () => {
		const fetcher = vi.fn(async () => new Response('{}'));

		await postAct('/api/an-act', 'key-1', { credits: 2 }, fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/an-act', {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'key-1' },
			body: '{"credits":2}'
		});
	});

	it('sends an empty object when the act names nothing', async () => {
		const fetcher = vi.fn(async () => new Response('{}'));

		await postAct('/api/an-act', 'key-1', undefined, fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/an-act', expect.objectContaining({ body: '{}' }));
	});
});

describe('serviceProblem', () => {
	it('says the service has a problem, in a new list each time', () => {
		expect(serviceProblem()).toEqual([{ message: SERVICE_PROBLEM }]);
		expect(serviceProblem()).not.toBe(serviceProblem());
	});
});

describe('refusal', () => {
	// A session route never answers 'step-up', so the arm is not there to read.
	it('reads a step-up answer from a session route as the refusal it says', async () => {
		const response = Response.json({ code: 'SECOND_FACTOR_REQUIRED', message: 'Enter a new code.' }, { status: 403 });

		expect(await refusal(response)).toEqual({ kind: 'refused', errors: [{ message: 'Enter a new code.' }] });
	});
});

describe('stepUpRefusal', () => {
	it('asks for a fresh code when doula-ops does', async () => {
		const response = Response.json({ code: 'SECOND_FACTOR_REQUIRED', message: 'Enter a new code.' }, { status: 403 });

		expect(await stepUpRefusal(response)).toEqual({ kind: 'step-up' });
	});

	it('reads any other 403 as a refusal', async () => {
		const response = Response.json({ code: 'FORBIDDEN', message: 'Not yours to do.' }, { status: 403 });

		expect(await stepUpRefusal(response)).toEqual({ kind: 'refused', errors: [{ message: 'Not yours to do.' }] });
	});

	it('reads a 403 with no JSON as a problem with the service', async () => {
		expect(await stepUpRefusal(new Response('denied', { status: 403 }))).toEqual({
			kind: 'refused',
			errors: [{ message: SERVICE_PROBLEM }]
		});
	});

	it('reads the rest as a session route does', async () => {
		expect(await stepUpRefusal(new Response(undefined, { status: 401 }))).toEqual({ kind: 'signed-out' });
	});
});
