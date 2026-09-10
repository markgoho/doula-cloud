import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	canConnectStatusStillMove,
	connect,
	loadConnectStatus,
	nudgeOwnersToConnect,
	pollConnectStatus,
	type ConnectStatusResult
} from './payments.js';
import { jsonResponse } from './testResponse.js';

describe('loadConnectStatus', () => {
	it('fetches the practice payments connect path and returns the decoded status', async () => {
		const status = {
			status: 'active',
			cardPaymentsStatus: 'active',
			payoutsStatus: 'active',
			requirementsDue: []
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(status));

		const result = await loadConnectStatus(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/payments/connect');
		expect(result).toEqual(status);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('forbidden', 403));

		await expect(loadConnectStatus(fetcher, 'practice-1')).rejects.toThrow('forbidden');
	});
});

describe('connect', () => {
	it('posts to the practice payments connect path and returns the onboarding URL', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ onboardingUrl: 'https://connect.stripe.com/setup/1' }));

		const result = await connect(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/payments/connect', { method: 'POST' });
		expect(result).toBe('https://connect.stripe.com/setup/1');
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('forbidden: owner only', 403));

		await expect(connect(fetcher, 'practice-1')).rejects.toThrow('forbidden: owner only');
	});

	it("reads #442's structured refusal as its sentence, not as its JSON", async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse(
				{
					code: 'FAILED_PRECONDITION',
					message: 'Tell us where Clients can find you online before you connect Stripe.'
				},
				400
			)
		);

		await expect(connect(fetcher, 'practice-1')).rejects.toThrow(
			'Tell us where Clients can find you online before you connect Stripe.'
		);
	});
});

function statusResult(overrides: Partial<ConnectStatusResult> = {}): ConnectStatusResult {
	return {
		status: 'pending',
		cardPaymentsStatus: 'pending',
		payoutsStatus: 'pending',
		requirementsDue: [],
		...overrides
	};
}

describe('nudgeOwnersToConnect', () => {
	it('posts to the nudge path under the connect path', async () => {
		const fetcher = vi.fn().mockResolvedValue(new Response(undefined, { status: 202 }));

		await nudgeOwnersToConnect(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/payments/connect/nudge', { method: 'POST' });
	});

	it('throws with the refusal the server sent', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('asked already this week', 409));

		await expect(nudgeOwnersToConnect(fetcher, 'practice-1')).rejects.toThrow('asked already this week');
	});
});

describe('canConnectStatusStillMove', () => {
	it('is true for pending, which always reports nothing outstanding', () => {
		expect(canConnectStatusStillMove(statusResult({ status: 'pending' }))).toBe(true);
	});

	it('is true for a non-active status that reports nothing outstanding', () => {
		expect(canConnectStatusStillMove(statusResult({ status: 'payouts_restricted', requirementsDue: [] }))).toBe(
			true
		);
	});

	it('is false once something is outstanding, whatever the status', () => {
		expect(
			canConnectStatusStillMove(
				statusResult({ status: 'onboarding_incomplete', requirementsDue: ['configuration.merchant.mcc'] })
			)
		).toBe(false);
	});

	it('is false once active, even with an empty requirementsDue', () => {
		expect(canConnectStatusStillMove(statusResult({ status: 'active', requirementsDue: [] }))).toBe(false);
	});
});

describe('pollConnectStatus', () => {
	const practiceId = 'practice-1';

	afterEach(() => {
		vi.useRealTimers();
	});

	it('re-reads after the first delay, reports the result, and stops once the status is active', async () => {
		vi.useFakeTimers();
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(statusResult({ status: 'active' })));
		const onResult = vi.fn();
		const onError = vi.fn();
		const onStopped = vi.fn();

		pollConnectStatus(fetcher, practiceId, statusResult(), { onResult, onError, onStopped }, [10, 20]);
		await vi.advanceTimersByTimeAsync(10);

		expect(fetcher).toHaveBeenCalledTimes(1);
		expect(onResult).toHaveBeenCalledWith(statusResult({ status: 'active' }));
		expect(onStopped).toHaveBeenCalledTimes(1);
		expect(onError).not.toHaveBeenCalled();
	});

	it('keeps re-reading on the schedule while the status keeps moving', async () => {
		vi.useFakeTimers();
		const fetcher = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(statusResult({ status: 'pending' })))
			.mockResolvedValueOnce(jsonResponse(statusResult({ status: 'active' })));
		const onResult = vi.fn();
		const onStopped = vi.fn();

		pollConnectStatus(fetcher, practiceId, statusResult(), { onResult, onError: vi.fn(), onStopped }, [10, 20]);

		await vi.advanceTimersByTimeAsync(10);
		expect(fetcher).toHaveBeenCalledTimes(1);
		expect(onStopped).not.toHaveBeenCalled();

		await vi.advanceTimersByTimeAsync(20);
		expect(fetcher).toHaveBeenCalledTimes(2);
		expect(onResult).toHaveBeenLastCalledWith(statusResult({ status: 'active' }));
		expect(onStopped).toHaveBeenCalledTimes(1);
	});

	it('stops at the ceiling, never reading past the end of the delay schedule', async () => {
		vi.useFakeTimers();
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(statusResult({ status: 'pending' })));
		const onStopped = vi.fn();

		pollConnectStatus(fetcher, practiceId, statusResult(), { onResult: vi.fn(), onError: vi.fn(), onStopped }, [
			10,
			10
		]);

		await vi.advanceTimersByTimeAsync(20);
		expect(fetcher).toHaveBeenCalledTimes(2);
		expect(onStopped).toHaveBeenCalledTimes(1);

		await vi.advanceTimersByTimeAsync(1000);
		expect(fetcher).toHaveBeenCalledTimes(2);
	});

	it('stops and reports the failure on a rejected read, without scheduling another', async () => {
		vi.useFakeTimers();
		const fetcher = vi.fn().mockRejectedValue(new Error('network down'));
		const onError = vi.fn();
		const onStopped = vi.fn();

		pollConnectStatus(fetcher, practiceId, statusResult(), { onResult: vi.fn(), onError, onStopped }, [10, 20]);
		await vi.advanceTimersByTimeAsync(10);

		expect(onError).toHaveBeenCalledTimes(1);
		expect(onStopped).toHaveBeenCalledTimes(1);

		await vi.advanceTimersByTimeAsync(1000);
		expect(fetcher).toHaveBeenCalledTimes(1);
	});

	it('discards an in-flight read that resolves after stop() -- no callback, no further schedule', async () => {
		vi.useFakeTimers();
		let resolveFetch!: (response: Response) => void;
		const fetcher = vi.fn(() => new Promise<Response>((resolve) => (resolveFetch = resolve)));
		const onResult = vi.fn();
		const onError = vi.fn();
		const onStopped = vi.fn();

		const handle = pollConnectStatus(
			fetcher,
			practiceId,
			statusResult(),
			{ onResult, onError, onStopped },
			[10, 20]
		);
		await vi.advanceTimersByTimeAsync(10);
		expect(fetcher).toHaveBeenCalledTimes(1);

		handle.stop();
		resolveFetch(jsonResponse(statusResult({ status: 'active' })));
		await vi.advanceTimersByTimeAsync(0);

		expect(onResult).not.toHaveBeenCalled();
		expect(onError).not.toHaveBeenCalled();
		expect(onStopped).not.toHaveBeenCalled();

		await vi.advanceTimersByTimeAsync(20);
		expect(fetcher).toHaveBeenCalledTimes(1);
	});

	it('stop() cancels the next scheduled read, and is safe to call more than once', async () => {
		vi.useFakeTimers();
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(statusResult({ status: 'pending' })));
		const onStopped = vi.fn();

		const handle = pollConnectStatus(
			fetcher,
			practiceId,
			statusResult(),
			{ onResult: vi.fn(), onError: vi.fn(), onStopped },
			[10, 20]
		);
		handle.stop();
		handle.stop();

		await vi.advanceTimersByTimeAsync(30);

		expect(fetcher).not.toHaveBeenCalled();
		expect(onStopped).not.toHaveBeenCalled();
	});
});
