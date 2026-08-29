import { describe, expect, it, vi } from 'vitest';
import { connect, loadConnectStatus } from './payments.js';
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
