import { describe, expect, it, vi } from 'vitest';
import { clearEmailSuppression, loadEmailSuppressions } from './emailSuppression.js';
import { jsonResponse } from './testResponse.js';

const bounce = {
	address: 'anne-marie.ochieng-whitfield@example.test',
	cause: 'bounce',
	createdAt: '2027-03-14T09:30:00Z',
	clearable: true
};

const complaint = {
	address: 'persephone@example.test',
	cause: 'complaint',
	createdAt: '2027-02-01T08:00:00Z',
	clearable: false
};

describe('loadEmailSuppressions', () => {
	it("reads the Practice's suppression path and returns the rows inside the envelope", async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ suppressions: [bounce, complaint] }));

		const result = await loadEmailSuppressions(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/email-suppressions');
		expect(result).toEqual([bounce, complaint]);
	});

	/*
	 * The Go handler always writes `suppressions: []` for a Practice with
	 * none, but a missing key would otherwise reach the screen as
	 * `undefined` and break the `.length` read that decides the empty
	 * state -- the one failure mode that would show a broken screen rather
	 * than an error.
	 */
	it('reads an absent suppressions key as no blocked addresses', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}));

		await expect(loadEmailSuppressions(fetcher, 'practice-1')).resolves.toEqual([]);
	});

	it("throws the BFF's own sentence when the read is refused", async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse({ message: 'forbidden: owner or admin only' }, 403));

		await expect(loadEmailSuppressions(fetcher, 'practice-1')).rejects.toThrow(
			'forbidden: owner or admin only'
		);
	});
});

describe('clearEmailSuppression', () => {
	it('POSTs the address in the body, never in the path', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(undefined, 204));

		await clearEmailSuppression(fetcher, 'practice-1', 'anne-marie+intake@example.test');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/email-suppressions/clear', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ address: 'anne-marie+intake@example.test' })
		});
	});

	it('throws the refusal when the address reported the email as spam', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(
				jsonResponse(
					{ message: 'this address reported the email as spam, so it stays blocked' },
					409
				)
			);

		await expect(
			clearEmailSuppression(fetcher, 'practice-1', 'persephone@example.test')
		).rejects.toThrow('this address reported the email as spam, so it stays blocked');
	});

	/*
	 * A 502 carries the one clause that decides what a person does next --
	 * nothing changed, so trying again is safe. This asserts it survives
	 * the trip, which is the whole reason this module reads a refusal
	 * through `apiErrorMessage` rather than `refusalMessage`.
	 */
	it("keeps the provider failure's own wording, including that nothing was changed", async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(
				jsonResponse(
					{ message: 'could not reach the email provider; nothing was changed' },
					502
				)
			);

		await expect(
			clearEmailSuppression(fetcher, 'practice-1', 'anne-marie@example.test')
		).rejects.toThrow('could not reach the email provider; nothing was changed');
	});
});
