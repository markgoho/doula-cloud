import { describe, expect, it, vi } from 'vitest';
import { savePracticeName } from './practiceName.js';
import { jsonResponse } from './testResponse.js';

describe('savePracticeName', () => {
	it('PUTs the name as JSON to the practice path', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ name: 'Willow Birth Services' }));

		const result = await savePracticeName(fetcher, 'practice-1', 'Willow Birth Services');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/name', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name: 'Willow Birth Services' })
		});
		expect(result).toEqual({ name: 'Willow Birth Services' });
	});

	it('throws a refusal the screen can read for the field it named', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse(
				{
					code: 'INVALID_ARGUMENT',
					message: 'name is required',
					details: { name: 'Enter the name of your Practice' }
				},
				400
			)
		);

		await expect(savePracticeName(fetcher, 'practice-1', '')).rejects.toMatchObject({
			details: { name: 'Enter the name of your Practice' }
		});
	});
});
