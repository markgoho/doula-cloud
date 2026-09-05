import { describe, expect, it, vi } from 'vitest';
import { loadMfaRequirementImpact, setMfaRequired } from './mfaRequirement.js';
import { jsonResponse } from './testResponse.js';

describe('loadMfaRequirementImpact', () => {
	it("fetches the practice's mfa-required impact path and returns the decoded result", async () => {
		const impact = { required: false, withoutSecondFactor: 4 };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(impact));

		const result = await loadMfaRequirementImpact(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/mfa-required/impact');
		expect(result).toEqual(impact);
	});

	it('throws with the refused response message on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('forbidden: owner only', 403));

		await expect(loadMfaRequirementImpact(fetcher, 'practice-1')).rejects.toThrow('forbidden: owner only');
	});
});

describe('setMfaRequired', () => {
	it("PUTs the new value with X-Confirmed to the practice's mfa-required path", async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(undefined, 204));

		await setMfaRequired(fetcher, 'practice-1', true);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/mfa-required', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json', 'X-Confirmed': 'true' },
			body: JSON.stringify({ required: true })
		});
	});

	it('throws with the refused response message on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse({ message: 'this action requires confirmation' }, 400)
		);

		await expect(setMfaRequired(fetcher, 'practice-1', false)).rejects.toThrow(
			'this action requires confirmation'
		);
	});
});
