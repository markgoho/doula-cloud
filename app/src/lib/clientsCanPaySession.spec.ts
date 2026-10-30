import { afterEach, describe, expect, it, vi } from 'vitest';
import { invalidate } from '$app/navigation';
import { PRACTICE_SESSION_DEPENDENCY } from './practiceSession.js';
import { refreshSessionIfClientsCanPayChanged } from './clientsCanPaySession.js';

vi.mock('$app/navigation', () => ({ invalidate: vi.fn() }));

afterEach(() => {
	vi.mocked(invalidate).mockClear();
});

describe('refreshSessionIfClientsCanPayChanged', () => {
	it('reads the session again when the session says Clients cannot pay and the stored fact says they can', () => {
		refreshSessionIfClientsCanPayChanged({ clientsCanPay: false }, true);

		expect(invalidate).toHaveBeenCalledExactlyOnceWith(PRACTICE_SESSION_DEPENDENCY);
	});

	it('reads the session again when the session says Clients can pay and the stored fact says they cannot', () => {
		refreshSessionIfClientsCanPayChanged({ clientsCanPay: true }, false);

		expect(invalidate).toHaveBeenCalledExactlyOnceWith(PRACTICE_SESSION_DEPENDENCY);
	});

	it.each([true, false])('leaves the session alone when both say %s', (clientsCanPay) => {
		refreshSessionIfClientsCanPayChanged({ clientsCanPay }, clientsCanPay);

		expect(invalidate).not.toHaveBeenCalled();
	});

	it('leaves the session alone when the session does not carry the fact', () => {
		refreshSessionIfClientsCanPayChanged({}, true);

		expect(invalidate).not.toHaveBeenCalled();
	});

	it('leaves the session alone when the read does not carry the fact', () => {
		refreshSessionIfClientsCanPayChanged({ clientsCanPay: false }, undefined);

		expect(invalidate).not.toHaveBeenCalled();
	});
});
