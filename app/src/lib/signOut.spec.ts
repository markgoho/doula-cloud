import { describe, expect, it, vi } from 'vitest';
import { SIGN_OUT_FAILED_MESSAGE, signOutOfSession } from './signOut.js';

function response(isOk: boolean): Response {
	return { ok: isOk } as Response;
}

interface StubOptions {
	endSessionOk?: boolean;
	endSessionThrows?: boolean;
	unregisterThrows?: boolean;
}

function stubs({ endSessionOk = true, endSessionThrows = false, unregisterThrows = false }: StubOptions = {}) {
	const order: string[] = [];
	const unregisterPush = vi.fn(async () => {
		order.push('unregister');
		if (unregisterThrows) throw new Error('no service worker');
	});
	const fetcher = vi.fn(async () => {
		order.push('end');
		if (endSessionThrows) throw new TypeError('Failed to fetch');
		return response(endSessionOk);
	});
	return { order, unregisterPush, fetcher };
}

describe('signOutOfSession', () => {
	it('unregisters this device push subscription before ending the session', async () => {
		const { order, unregisterPush, fetcher } = stubs();

		const outcome = await signOutOfSession({ practiceId: 'practice-1', fetcher, unregisterPush });

		expect(outcome).toEqual({ ok: true });
		expect(unregisterPush).toHaveBeenCalledWith(
			'/api/practices/practice-1/push-subscriptions',
			fetcher
		);
		expect(fetcher).toHaveBeenCalledWith('/api/session', { method: 'DELETE' });
		expect(order).toEqual(['unregister', 'end']);
	});

	it('skips the unregister when the screen carries no Practice scope', async () => {
		const { order, unregisterPush, fetcher } = stubs();

		const outcome = await signOutOfSession({ practiceId: undefined, fetcher, unregisterPush });

		expect(outcome).toEqual({ ok: true });
		expect(unregisterPush).not.toHaveBeenCalled();
		expect(order).toEqual(['end']);
	});

	it('still ends the session when the unregister fails', async () => {
		const { order, fetcher, unregisterPush } = stubs({ unregisterThrows: true });

		const outcome = await signOutOfSession({ practiceId: 'practice-1', fetcher, unregisterPush });

		expect(outcome).toEqual({ ok: true });
		expect(order).toEqual(['unregister', 'end']);
	});

	it('reports a failure the BFF refuses rather than reporting success', async () => {
		const { fetcher, unregisterPush } = stubs({ endSessionOk: false });

		const outcome = await signOutOfSession({ practiceId: 'practice-1', fetcher, unregisterPush });

		expect(outcome).toEqual({ ok: false, message: SIGN_OUT_FAILED_MESSAGE });
	});

	it('reports a failure the network never delivered rather than reporting success', async () => {
		const { fetcher, unregisterPush } = stubs({ endSessionThrows: true });

		const outcome = await signOutOfSession({ practiceId: 'practice-1', fetcher, unregisterPush });

		expect(outcome).toEqual({ ok: false, message: SIGN_OUT_FAILED_MESSAGE });
	});
});
