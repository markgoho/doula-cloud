import { describe, expect, it, vi } from 'vitest';
import { loadContractTemplate, saveContractTemplate, validateProse } from './contractTemplate.js';
import { jsonResponse } from './testResponse.js';

describe('loadContractTemplate', () => {
	it('fetches the practice path and returns the decoded template', async () => {
		const template = { prose: 'Some prose' };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(template));

		const result = await loadContractTemplate(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/contract-template');
		expect(result).toEqual(template);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not found', 404));

		await expect(loadContractTemplate(fetcher, 'practice-1')).rejects.toThrow('not found');
	});
});

describe('saveContractTemplate', () => {
	it('PUTs prose as JSON to the practice path', async () => {
		const saved = { prose: 'Updated prose' };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(saved));

		const result = await saveContractTemplate(fetcher, 'practice-1', 'Updated prose');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/contract-template', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ prose: 'Updated prose' })
		});
		expect(result).toEqual(saved);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('only a Practice Owner can do that', 403));

		await expect(saveContractTemplate(fetcher, 'practice-1', 'prose')).rejects.toThrow(
			'only a Practice Owner can do that'
		);
	});
});

describe('validateProse', () => {
	it('accepts non-blank prose', () => {
		expect(validateProse('Some prose')).toBeUndefined();
	});

	it('rejects an empty string', () => {
		expect(validateProse('')).toBe('prose is required');
	});

	it('rejects whitespace-only prose', () => {
		expect(validateProse('   \n\t  ')).toBe('prose is required');
	});
});
