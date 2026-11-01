import { describe, expect, it, vi } from 'vitest';
import { practiceList, readSecondFactorCheck, sendSecondFactorClear } from './secondFactor.ts';

function answering(status: number, body?: unknown) {
	return vi.fn<typeof fetch>(
		async () => new Response(body === undefined ? undefined : JSON.stringify(body), { status })
	);
}

const staff = { staffName: 'Priya Raman', practiceNames: ['Lakeside Birth Collective', 'Rooted Doula Care'] };

describe('practiceList', () => {
	it('names each Practice, or none', () => {
		expect(practiceList(staff.practiceNames)).toBe('Lakeside Birth Collective, Rooted Doula Care');
		expect(practiceList([])).toBe('None');
	});
});

describe('readSecondFactorCheck', () => {
	it('reads the Staff member and each Practice', async () => {
		const fetcher = answering(200, staff);

		expect(await readSecondFactorCheck('s-1', fetcher)).toEqual({ kind: 'ok', value: staff });
		expect(fetcher).toHaveBeenCalledWith('/api/staff/s-1/second-factor', { credentials: 'include' });
	});

	it('says there is no such Staff member', async () => {
		const message = 'There is no Staff member at this address.';

		expect(await readSecondFactorCheck('s-1', answering(404, { code: 'NOT_FOUND', message }))).toEqual({
			kind: 'refused',
			errors: [{ message }]
		});
	});
});

describe('sendSecondFactorClear', () => {
	it('sends no fields under its key and hands back what changed', async () => {
		const fetcher = answering(200, staff);

		expect(await sendSecondFactorClear('s-1', 'key-1', fetcher)).toEqual({ kind: 'ok', value: staff });
		expect(fetcher).toHaveBeenCalledWith('/api/staff/s-1/second-factor-clears', {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'key-1' },
			body: '{}'
		});
	});

	it('asks for a fresh code again', async () => {
		expect(
			await sendSecondFactorClear('s-1', 'key-1', answering(403, { code: 'SECOND_FACTOR_REQUIRED', message: 'code' }))
		).toEqual({ kind: 'step-up' });
	});

	it('says Identity Platform changed nothing', async () => {
		const message = 'Identity Platform did not reset two-factor authentication, so nothing has changed. Try again.';

		expect(await sendSecondFactorClear('s-1', 'key-1', answering(502, { code: 'INTERNAL', message }))).toEqual({
			kind: 'refused',
			errors: [{ message }]
		});
	});
});
