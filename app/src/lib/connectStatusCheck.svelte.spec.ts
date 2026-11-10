import { afterEach, describe, expect, it, vi } from 'vitest';

import {
	ConnectStatusCheck,
	connectArrival,
	CONNECT_STATUS_LOAD_FAILED_MESSAGE
} from './connectStatusCheck.svelte.js';
import { CONNECT_STATUS_CHECK_FAILED_MESSAGE, type ConnectStatus, type ConnectStatusResult } from './payments.js';
import { jsonResponse } from './testResponse.js';

const DELAYS = [10, 20, 30];

function statusResult(status: ConnectStatus, requirementsDue: string[] = []): ConnectStatusResult {
	return { status, cardPaymentsStatus: 'pending', payoutsStatus: 'pending', requirementsDue };
}

type Reply = ConnectStatusResult | 'error' | 'held';

/**
 * A fetcher that answers each Connect status read with the next reply,
 * holding on the last. A `held` reply waits until the test releases it
 * by its position in the order the reads went out.
 */
function scripted(replies: Reply[]) {
	const held = new Map<number, (response: Response) => void>();
	let call = 0;
	const fetcher = vi.fn((): Promise<Response> => {
		const index = call;
		call += 1;
		const reply = replies[Math.min(index, replies.length - 1)];
		if (reply === 'held') return new Promise((resolve) => held.set(index, resolve));
		if (reply === 'error') return Promise.resolve(new Response('down', { status: 500 }));
		return Promise.resolve(jsonResponse(reply));
	});
	return {
		fetcher,
		release(index: number, result: ConnectStatusResult) {
			held.get(index)!(jsonResponse(result));
		},
		fail(index: number) {
			held.get(index)!(new Response('down', { status: 500 }));
		}
	};
}

function setup(replies: Reply[]) {
	const script = scripted(replies);
	const check = new ConnectStatusCheck(script.fetcher, 'practice-1', DELAYS);
	return { check, ...script };
}

// Settled on active, saying so, and reading no more than reads times.
async function expectFinishedAfter({ check, fetcher }: Pick<ReturnType<typeof setup>, 'check' | 'fetcher'>, reads: number) {
	expect(check.status).toEqual(statusResult('active'));
	expect(check.phase).toEqual({ phase: 'settled' });
	expect(check.banner).toBe('finished');
	await vi.advanceTimersByTimeAsync(1000);
	expect(fetcher).toHaveBeenCalledTimes(reads);
}

afterEach(() => {
	vi.useRealTimers();
});

function at(query: string) {
	return connectArrival(new URL(`https://example.test/settings/payments${query}`));
}

describe('connectArrival', () => {
	it('names the two trips back from Stripe, and anything else as a plain visit', () => {
		expect(at('?connect=return')).toBe('return');
		expect(at('?connect=refresh')).toBe('refresh');
		expect(at('')).toBe('plain');
		expect(at('?connect=something')).toBe('plain');
	});
});

describe('ConnectStatusCheck: the first read', () => {
	it('starts reading, with no status and no banner', () => {
		const { check } = setup([statusResult('pending')]);

		expect(check.phase).toEqual({ phase: 'reading' });
		expect(check.status).toBeUndefined();
		expect(check.banner).toBeUndefined();
		expect(check.isStripeCatchingUp).toBe(false);
	});

	it('settles a plain visit on its first read, and never reads again on its own', async () => {
		vi.useFakeTimers();
		const { check, fetcher } = setup([statusResult('pending')]);

		expect(await check.first('plain')).toBe(true);
		check.startPolling();
		await vi.advanceTimersByTimeAsync(1000);

		expect(check.phase).toEqual({ phase: 'settled' });
		expect(check.status).toEqual(statusResult('pending'));
		expect(check.banner).toBeUndefined();
		expect(fetcher).toHaveBeenCalledTimes(1);
	});

	it('says an expired link was followed', async () => {
		const { check } = setup([statusResult('onboarding_incomplete')]);

		await check.first('refresh');

		expect(check.banner).toBe('link-expired');
	});

	it('settles a return whose first read is already active, as finished', async () => {
		const { check } = setup([statusResult('active')]);

		await check.first('return');

		expect(check.phase).toEqual({ phase: 'settled' });
		expect(check.banner).toBe('finished');
	});

	it('polls a return whose status can still move, and says Stripe is catching up on outstanding requirements', async () => {
		const { check } = setup([statusResult('onboarding_incomplete', ['defaults.profile.business_url'])]);

		await check.first('return');

		expect(check.phase).toEqual({ phase: 'polling' });
		expect(check.banner).toBe('checking');
		expect(check.isStripeCatchingUp).toBe(true);
	});

	it("fails with the read's own words, or a sentence of its own", async () => {
		const worded = new ConnectStatusCheck(vi.fn().mockRejectedValue(new Error('Stripe is down')), 'practice-1');
		const silent = new ConnectStatusCheck(vi.fn().mockRejectedValue('offline'), 'practice-1');

		expect(await worded.first('plain')).toBe(false);
		await silent.first('plain');

		expect(worded.phase).toEqual({ phase: 'failed', message: 'Stripe is down' });
		expect(silent.phase).toEqual({ phase: 'failed', message: CONNECT_STATUS_LOAD_FAILED_MESSAGE });
	});

	it('drops a first read that answers after the screen was left, either way', async () => {
		const { check, release } = setup(['held']);
		const failing = setup(['held']);

		const landing = check.first('return');
		const failingLanding = failing.check.first('return');
		check.stop();
		failing.check.stop();
		release(0, statusResult('pending'));
		failing.fail(0);

		expect(await landing).toBe(false);
		expect(await failingLanding).toBe(false);
		expect(check.status).toBeUndefined();
		expect(failing.check.phase).toEqual({ phase: 'reading' });
	});
});

describe('ConnectStatusCheck: the poll', () => {
	it('re-reads until the status is active, then settles as finished', async () => {
		vi.useFakeTimers();
		const { check, fetcher } = setup([statusResult('pending'), statusResult('pending'), statusResult('active')]);
		await check.first('return');
		check.startPolling();
		check.startPolling();

		await vi.advanceTimersByTimeAsync(10);
		expect(check.phase).toEqual({ phase: 'polling' });

		await vi.advanceTimersByTimeAsync(20);
		await expectFinishedAfter({ check, fetcher }, 3);
	});

	it('believes an outstanding requirement once the schedule runs out', async () => {
		vi.useFakeTimers();
		const { check } = setup([statusResult('onboarding_incomplete', ['defaults.profile.business_url'])]);
		await check.first('return');
		check.startPolling();

		await vi.advanceTimersByTimeAsync(60);

		expect(check.phase).toEqual({ phase: 'settled' });
		expect(check.banner).toBe('still-restricted');
		expect(check.isStripeCatchingUp).toBe(false);
	});

	it('keeps the last good status and says the check failed when a re-read fails', async () => {
		vi.useFakeTimers();
		const { check } = setup([statusResult('pending'), 'error']);
		await check.first('return');
		check.startPolling();

		await vi.advanceTimersByTimeAsync(10);

		expect(check.status).toEqual(statusResult('pending'));
		expect(check.refreshError).toBe(CONNECT_STATUS_CHECK_FAILED_MESSAGE);
		expect(check.phase).toEqual({ phase: 'settled' });
	});

	it('clears an earlier failure once a poll read lands', async () => {
		vi.useFakeTimers();
		const { check } = setup([statusResult('pending'), 'error', statusResult('pending')]);
		await check.first('return');
		await check.recheck();
		expect(check.refreshError).toBe(CONNECT_STATUS_CHECK_FAILED_MESSAGE);
		check.startPolling();

		await vi.advanceTimersByTimeAsync(10);

		expect(check.refreshError).toBe('');
	});

	it('starts nothing once the screen is left', async () => {
		vi.useFakeTimers();
		const { check, fetcher } = setup([statusResult('pending')]);
		await check.first('return');
		check.stop();
		check.startPolling();

		await vi.advanceTimersByTimeAsync(1000);

		expect(fetcher).toHaveBeenCalledTimes(1);
	});
});

describe('ConnectStatusCheck: the on-demand check beside the poll', () => {
	it('ends the poll the moment a check answers active, so the banner never says Stripe is still checking', async () => {
		vi.useFakeTimers();
		const { check, fetcher } = setup([statusResult('pending'), statusResult('active')]);
		await check.first('return');
		check.startPolling();

		await check.recheck();

		await expectFinishedAfter({ check, fetcher }, 2);
	});

	it('never lets a slower, older read overwrite a newer one', async () => {
		vi.useFakeTimers();
		// The first read, then the check (held), then the poll's re-read.
		const { check, release } = setup([statusResult('pending'), 'held', statusResult('payouts_restricted')]);
		await check.first('return');
		check.startPolling();

		const rechecking = check.recheck();
		await vi.advanceTimersByTimeAsync(10);
		expect(check.status).toEqual(statusResult('payouts_restricted'));

		release(1, statusResult('pending'));
		await rechecking;

		expect(check.status).toEqual(statusResult('payouts_restricted'));
		expect(check.isRechecking).toBe(false);
		check.stop();
	});

	it('drops a poll read that answers after a newer check has landed', async () => {
		vi.useFakeTimers();
		// The first read, the poll's re-read (held), then the check.
		const { check, release } = setup([statusResult('pending'), 'held', statusResult('payouts_restricted')]);
		await check.first('return');
		check.startPolling();
		await vi.advanceTimersByTimeAsync(10);
		await check.recheck();
		expect(check.status).toEqual(statusResult('payouts_restricted'));

		release(1, statusResult('pending'));
		await vi.advanceTimersByTimeAsync(0);

		expect(check.status).toEqual(statusResult('payouts_restricted'));
		check.stop();
	});

	it("drops an older check's failure once a newer read has landed, and a poll failure older than a check", async () => {
		vi.useFakeTimers();
		// The first read, the check (held), the poll's re-read (held), a second check.
		const { check, fail } = setup([statusResult('pending'), 'held', 'held', statusResult('pending')]);
		await check.first('return');
		check.startPolling();
		const rechecking = check.recheck();
		await vi.advanceTimersByTimeAsync(10);
		await check.recheck();

		fail(1);
		await rechecking;
		fail(2);
		await vi.advanceTimersByTimeAsync(0);

		expect(check.refreshError).toBe('');
		expect(check.status).toEqual(statusResult('pending'));
	});

	it('says a failed check did not go through, and keeps the status', async () => {
		const { check } = setup([statusResult('not_connected'), 'error']);
		await check.first('plain');

		const rechecking = check.recheck();
		expect(check.isRechecking).toBe(true);
		await rechecking;

		expect(check.isRechecking).toBe(false);
		expect(check.refreshError).toBe(CONNECT_STATUS_CHECK_FAILED_MESSAGE);
		expect(check.status).toEqual(statusResult('not_connected'));
	});

	it('stays busy until the latest of two overlapping checks answers', async () => {
		const { check, release } = setup([statusResult('pending'), 'held', 'held']);
		await check.first('plain');

		const older = check.recheck();
		const newer = check.recheck();
		release(1, statusResult('pending'));
		await older;
		expect(check.isRechecking).toBe(true);

		release(2, statusResult('active'));
		await newer;
		expect(check.isRechecking).toBe(false);
		expect(check.status).toEqual(statusResult('active'));
	});
});
