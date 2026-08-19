import { afterEach, describe, expect, it, vi } from 'vitest';
import { SIGN_OUT_FAILED_MESSAGE, UNREGISTER_TIMEOUT_MS, signOutOfSession } from './signOut.js';

function response(isOk: boolean): Response {
	return { ok: isOk } as Response;
}

interface StubOptions {
	endSessionOk?: boolean;
	endSessionThrows?: boolean;
	unregisterThrows?: boolean;
	/**
	 * Leaves the unregister pending forever -- what a stalled connection to
	 * a down BFF does, since a hung request never rejects.
	 */
	unregisterHangs?: boolean;
}

function stubs({
	endSessionOk = true,
	endSessionThrows = false,
	unregisterThrows = false,
	unregisterHangs = false
}: StubOptions = {}) {
	const order: string[] = [];
	const unregisterPush = vi.fn(async () => {
		order.push('unregister');
		if (unregisterHangs) await new Promise<void>(() => {});
		if (unregisterThrows) throw new Error('no service worker');
	});
	const fetcher = vi.fn(async () => {
		order.push('end');
		if (endSessionThrows) throw new TypeError('Failed to fetch');
		return response(endSessionOk);
	});
	return { order, unregisterPush, fetcher };
}

afterEach(() => {
	vi.useRealTimers();
});

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

	it('gives up on an unregister that hangs rather than stranding the sign-out', async () => {
		vi.useFakeTimers();
		const { order, fetcher, unregisterPush } = stubs({ unregisterHangs: true });

		const pending = signOutOfSession({ practiceId: 'practice-1', fetcher, unregisterPush });
		await vi.advanceTimersByTimeAsync(UNREGISTER_TIMEOUT_MS);

		await expect(pending).resolves.toEqual({ ok: true });
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
