import { describe, expect, it, vi } from 'vitest';
import {
	answerErrors,
	CREDITS_FIELD,
	longDate,
	money,
	readRefundCheck,
	REQUESTED_BY_FIELD,
	REQUESTED_ON_FIELD,
	requestedOn,
	sendRefund
} from './refund.ts';
import { SERVICE_PROBLEM } from './session.ts';

function answering(status: number, body?: unknown) {
	return vi.fn<typeof fetch>(
		async () => new Response(body === undefined ? undefined : JSON.stringify(body), { status })
	);
}

const answers = { credits: '2', requestedBy: 'owner-1', day: '30', month: '10', year: '2026' };
const check = { practiceName: 'Lakeside', owners: [{ staffId: 'owner-1', name: 'Renata Alvarez' }], quote: { credits: 2, amountCents: 4274, taxCents: 274, maxCredits: 5 } };

describe('money and longDate', () => {
	it('reads cents as dollars and a date as words', () => {
		expect(money(4274)).toBe('$42.74');
		expect(longDate('2026-10-30')).toBe('October 30, 2026');
	});
});

describe('requestedOn', () => {
	it('reads the three boxes as one date', () => {
		expect(requestedOn({ day: ' 3', month: '9', year: '2026' })).toBe('2026-09-03');
	});

	it.each([
		['a missing day', { day: '', month: '9', year: '2026' }],
		['a two-digit year', { day: '3', month: '9', year: '26' }],
		['a day the month does not have', { day: '31', month: '9', year: '2026' }],
		['words', { day: 'third', month: '9', year: '2026' }]
	])('refuses %s', (_, date) => {
		expect(requestedOn(date)).toBeUndefined();
	});
});

describe('answerErrors', () => {
	it('finds nothing missing in full answers', () => {
		expect(answerErrors(answers)).toEqual([]);
	});

	it('says what to enter for each empty answer', () => {
		expect(answerErrors({ credits: '0', requestedBy: '', day: '', month: '', year: '' })).toEqual([
			{ message: 'Enter the number of Credits to refund, 1 or more', field: CREDITS_FIELD },
			{ message: 'Choose the Owner who asked for the refund', field: REQUESTED_BY_FIELD },
			{ message: 'Enter the date the request arrived', field: REQUESTED_ON_FIELD }
		]);
		expect(answerErrors({ ...answers, credits: 'two' })[0].field).toBe(CREDITS_FIELD);
	});
});

describe('readRefundCheck', () => {
	it('reads the check for a number of Credits', async () => {
		const fetcher = answering(200, check);

		expect(await readRefundCheck('p-1', 2, fetcher)).toEqual({ kind: 'ok', value: check });
		expect(fetcher).toHaveBeenCalledWith('/api/practices/p-1/refund?credits=2', { credentials: 'include' });
	});

	it('reads the check for the whole purchase', async () => {
		const fetcher = answering(200, check);

		await readRefundCheck('p-1', undefined, fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/p-1/refund', { credentials: 'include' });
	});

	it('names an ended session', async () => {
		expect(await readRefundCheck('p-1', 2, answering(401, { message: 'ended' }))).toEqual({ kind: 'signed-out' });
	});

	it('puts a refused answer beside its field', async () => {
		const outcome = await readRefundCheck(
			'p-1',
			6,
			answering(422, { code: 'INVALID_ARGUMENT', message: 'Correct the answers marked below.', details: { credits: 'Number of Credits must be 5 or fewer' } })
		);

		expect(outcome).toEqual({ kind: 'refused', errors: [{ message: 'Number of Credits must be 5 or fewer', field: CREDITS_FIELD }] });
	});

	it('shows a refusal with no field as it is', async () => {
		const outcome = await readRefundCheck('p-1', 2, answering(404, { code: 'NOT_FOUND', message: 'There is no Practice at this address.' }));

		expect(outcome).toEqual({ kind: 'refused', errors: [{ message: 'There is no Practice at this address.' }] });
	});

	it('shows the service problem for a body that is not the error structure', async () => {
		const fetcher = vi.fn<typeof fetch>(async () => new Response('<html>', { status: 502 }));

		expect(await readRefundCheck('p-1', 2, fetcher)).toEqual({ kind: 'refused', errors: [{ message: SERVICE_PROBLEM }] });
	});

	it('shows the service problem for a 5xx carrying details', async () => {
		const outcome = await readRefundCheck('p-1', 2, answering(500, { code: 'INTERNAL_ERROR', message: 'x', details: { credits: 'y' } }));

		expect(outcome).toEqual({ kind: 'refused', errors: [{ message: SERVICE_PROBLEM }] });
	});
});

describe('sendRefund', () => {
	const order = { credits: 2, requestedBy: 'owner-1', requestedOn: '2026-10-30' };

	it('sends the order under its key and hands back what changed', async () => {
		const result = { practiceName: 'Lakeside', credits: 2, amountCents: 4274, taxCents: 274, requestedByName: 'Renata Alvarez', requestedOn: '2026-10-30' };
		const fetcher = answering(200, result);

		expect(await sendRefund('p-1', order, 'key-1', fetcher)).toEqual({ kind: 'ok', value: result });
		expect(fetcher).toHaveBeenCalledWith('/api/practices/p-1/refunds', {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'key-1' },
			body: JSON.stringify(order)
		});
	});

	it('asks for a fresh code again when the step-up is spent or stale', async () => {
		const fetcher = answering(403, { code: 'SECOND_FACTOR_REQUIRED', message: 'Enter the code' });

		expect(await sendRefund('p-1', order, 'key-1', fetcher)).toEqual({ kind: 'step-up' });
	});

	it('says why Stripe refused', async () => {
		const message = 'Stripe did not make the refund, so nothing has changed. Try again.';
		const fetcher = answering(502, { code: 'INTERNAL_ERROR', message });

		expect(await sendRefund('p-1', order, 'key-1', fetcher)).toEqual({ kind: 'refused', errors: [{ message }] });
	});
});
