import { describe, expect, it, vi } from 'vitest';
import { SERVICE_PROBLEM } from './formErrors.js';
import { rotateSavedCodes, spendRecoveryCode, vouchForStaff } from './mfaRecovery.js';
import { jsonResponse } from './testResponse.js';

describe('vouchForStaff', () => {
	it("POSTs to the practice-scoped vouch path with the step-up token and the endpoint's confirmation header", async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(undefined, 204));

		await vouchForStaff(fetcher, 'practice-1', 'staff-9', 'fresh-id-token');

		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/staff/staff-9/mfa-recovery/vouch',
			{
				method: 'POST',
				headers: { Authorization: 'Bearer fresh-id-token', 'X-Confirmed': 'true' }
			}
		);
	});

	// The step-up token going stale is a 401 the caller has to be able to
	// tell apart from a dead session, which is why this throws rather than
	// letting api.ts's session handling see it at all.
	it('throws with the refusal message when the step-up token is no longer fresh', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse({ message: 'this action requires a fresh sign-in' }, 401));

		await expect(vouchForStaff(fetcher, 'practice-1', 'staff-9', 'stale')).rejects.toThrow(
			'this action requires a fresh sign-in'
		);
	});
});

describe('rotateSavedCodes', () => {
	it('POSTs to the rotate path and returns the plaintext set', async () => {
		const codes = ['AAAA1111', 'BBBB2222'];
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ codes }));

		const result = await rotateSavedCodes(fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/staff/mfa-recovery/saved-codes/rotate', {
			method: 'POST'
		});
		expect(result).toEqual(codes);
	});

	it("throws with the BFF's own words when the caller is not a sole owner", async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse(
				{ message: "saved recovery codes are only issued to a practice's sole owner" },
				403
			)
		);

		await expect(rotateSavedCodes(fetcher)).rejects.toThrow(
			"saved recovery codes are only issued to a practice's sole owner"
		);
	});
});

describe('spendRecoveryCode', () => {
	it('POSTs the address and the code as JSON', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(undefined, 204));

		await spendRecoveryCode(fetcher, 'anne@example.test', 'CODE-1');

		expect(fetcher).toHaveBeenCalledWith('/api/staff/mfa-recovery/spend', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email: 'anne@example.test', code: 'CODE-1' })
		});
	});

	// #168: the endpoint answers a wrong code and an unknown address with
	// one sentence, and this module's job is to carry that sentence
	// through unchanged rather than working out which case it was.
	it('throws the one refusal the endpoint gives for both a wrong code and an unknown address', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse({ message: 'this code is invalid or has expired' }, 400));

		await expect(spendRecoveryCode(fetcher, 'nobody@example.test', 'nope')).rejects.toThrow(
			'this code is invalid or has expired'
		);
	});

	// A bodyless 5xx is ours, not hers: it reads as the service message
	// rather than as an empty sentence a screen would render as a blank
	// error.
	it('throws the service message when the failure carries no words of its own', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('', 500));

		await expect(spendRecoveryCode(fetcher, 'anne@example.test', 'CODE-1')).rejects.toThrow(
			SERVICE_PROBLEM
		);
	});

	// A 429 is the per-account throttle (#602), not an app failure -- it
	// arrives as an ordinary refusal carrying the server's own sentence,
	// so a screen shows it beside the form rather than as a broken page.
	it('throws the throttle refusal with the wait it names', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(
				jsonResponse({ message: 'too many requests -- try again in 43 seconds' }, 429)
			);

		await expect(spendRecoveryCode(fetcher, 'anne@example.test', 'CODE-1')).rejects.toThrow(
			'too many requests -- try again in 43 seconds'
		);
	});
});
