import { describe, expect, it, vi } from 'vitest';
import { loadPracticeTimezone, savePracticeTimezone } from './practiceTimezone.js';
import { jsonResponse } from './testResponse.js';

describe('loadPracticeTimezone', () => {
	it('fetches the practice path and returns the decoded zone', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ timezone: 'America/Denver' }));

		const result = await loadPracticeTimezone(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/timezone');
		expect(result).toEqual({ timezone: 'America/Denver' });
	});

	it('throws with the response body message on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('internal error', 500));

		await expect(loadPracticeTimezone(fetcher, 'practice-1')).rejects.toThrow('internal error');
	});
});

describe('savePracticeTimezone', () => {
	it('PUTs the zone as JSON to the practice path', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ timezone: 'America/Denver' }));

		const result = await savePracticeTimezone(fetcher, 'practice-1', 'America/Denver');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/timezone', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ timezone: 'America/Denver' })
		});
		expect(result).toEqual({ timezone: 'America/Denver' });
	});

	it("sends a zone the seven-entry list does not carry, because the API's database decides", async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse({ timezone: 'America/Indiana/Indianapolis' }));

		await savePracticeTimezone(fetcher, 'practice-1', 'America/Indiana/Indianapolis');

		expect(fetcher.mock.calls[0][1].body).toBe(
			JSON.stringify({ timezone: 'America/Indiana/Indianapolis' })
		);
	});

	it('throws with the refusal the BFF wrote on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not an IANA zone name', 400));

		await expect(
			savePracticeTimezone(fetcher, 'practice-1', 'Nowhere/Atlantis')
		).rejects.toThrow('not an IANA zone name');
	});
});
