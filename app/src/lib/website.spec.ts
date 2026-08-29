import { describe, expect, it, vi } from 'vitest';
import {
	loadWebsite,
	saveWebsite,
	WebsiteValidationError,
	MAX_FACT_LENGTH,
	type Fetcher
} from './website.js';

function response(body: unknown, init: { ok?: boolean; status?: number } = {}): Response {
	const text = typeof body === 'string' ? body : JSON.stringify(body);
	return {
		ok: init.ok ?? true,
		status: init.status ?? 200,
		text: () => Promise.resolve(text),
		json: () => Promise.resolve(body)
	} as Response;
}

const undeclared = {
	mode: 'undeclared',
	ownUrl: '',
	serviceDescription: '',
	cancellationPolicy: '',
	updatedBy: '',
	updatedAt: ''
};

describe('loadWebsite', () => {
	it('reads a Practice that has not answered as undeclared rather than an error', async () => {
		const fetcher: Fetcher = vi.fn(() => Promise.resolve(response(undeclared)));

		await expect(loadWebsite(fetcher, 'practice-1')).resolves.toEqual(undeclared);
		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/website');
	});

	it('throws with the server sentence on a failure', async () => {
		const fetcher: Fetcher = vi.fn(() =>
			Promise.resolve(
				response({ code: 'INTERNAL_ERROR', message: 'internal error' }, { ok: false, status: 500 })
			)
		);

		await expect(loadWebsite(fetcher, 'practice-1')).rejects.toThrow('internal error');
	});
});

describe('saveWebsite', () => {
	it('sends the declaration as a full replacement', async () => {
		const fetcher: Fetcher = vi.fn(() =>
			Promise.resolve(response({ ...undeclared, mode: 'own', ownUrl: 'https://example.com' }))
		);

		const saved = await saveWebsite(fetcher, 'practice-1', {
			mode: 'own',
			ownUrl: 'example.com'
		});

		expect(saved.mode).toBe('own');
		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/website',
			expect.objectContaining({ method: 'PUT' })
		);
	});

	it('turns a 400 into a WebsiteValidationError carrying the field details', async () => {
		const fetcher: Fetcher = vi.fn(() =>
			Promise.resolve(
				response(
					{
						code: 'INVALID_ARGUMENT',
						message: 'invalid request body',
						details: { ownUrl: 'Enter a web address in the correct format' }
					},
					{ ok: false, status: 400 }
				)
			)
		);

		await expect(
			saveWebsite(fetcher, 'practice-1', { mode: 'own', ownUrl: 'coming soon' })
		).rejects.toBeInstanceOf(WebsiteValidationError);

		try {
			await saveWebsite(fetcher, 'practice-1', { mode: 'own', ownUrl: 'coming soon' });
			expect.unreachable('saveWebsite should have thrown');
		} catch (error) {
			expect(error).toBeInstanceOf(WebsiteValidationError);
			expect((error as WebsiteValidationError).fieldErrors.ownUrl).toBe(
				'Enter a web address in the correct format'
			);
		}
	});

	it('still reports a 400 that is not JSON, with no field details', async () => {
		const fetcher: Fetcher = vi.fn(() =>
			Promise.resolve(response('only a Practice Owner can do that', { ok: false, status: 400 }))
		);

		try {
			await saveWebsite(fetcher, 'practice-1', { mode: 'own', ownUrl: 'example.com' });
			expect.unreachable('saveWebsite should have thrown');
		} catch (error) {
			expect(error).toBeInstanceOf(WebsiteValidationError);
			expect((error as WebsiteValidationError).message).toBe('only a Practice Owner can do that');
			expect((error as WebsiteValidationError).fieldErrors).toEqual({});
		}
	});

	it('reads a 400 whose body names no details and no message', async () => {
		const fetcher: Fetcher = vi.fn(() => Promise.resolve(response({}, { ok: false, status: 400 })));

		try {
			await saveWebsite(fetcher, 'practice-1', { mode: 'own', ownUrl: 'example.com' });
			expect.unreachable('saveWebsite should have thrown');
		} catch (error) {
			expect((error as WebsiteValidationError).fieldErrors).toEqual({});
		}
	});

	it('throws a plain Error for a refusal that is not a validation failure', async () => {
		const fetcher: Fetcher = vi.fn(() =>
			Promise.resolve(response('only a Practice Owner can do that', { ok: false, status: 403 }))
		);

		await expect(
			saveWebsite(fetcher, 'practice-1', { mode: 'own', ownUrl: 'example.com' })
		).rejects.toThrow('only a Practice Owner can do that');
	});
});

describe('MAX_FACT_LENGTH', () => {
	it('is the same budget the server enforces', () => {
		expect(MAX_FACT_LENGTH).toBe(500);
	});
});
