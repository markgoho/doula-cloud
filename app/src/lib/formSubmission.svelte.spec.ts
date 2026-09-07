import { describe, expect, it, vi } from 'vitest';

import { FormSubmission, orServiceProblem, orThrownMessage } from './formSubmission.svelte.js';
import type { FormError } from './formErrors.js';

describe('FormSubmission', () => {
	it('starts clean', () => {
		const submission = new FormSubmission();

		expect(submission.errors).toEqual([]);
		expect(submission.isSubmitting).toBe(false);
	});

	describe('errorFor', () => {
		it("reads the entry targeting the given id's message", () => {
			const submission = new FormSubmission();
			submission.errors = [
				{ message: 'Enter your email address', targetId: 'email' },
				{ message: 'There is a problem with the service.' }
			];

			expect(submission.errorFor('email')).toBe('Enter your email address');
		});

		it('is undefined when no entry targets that id', () => {
			const submission = new FormSubmission();
			submission.errors = [{ message: 'There is a problem with the service.' }];

			expect(submission.errorFor('email')).toBeUndefined();
		});
	});

	describe('run', () => {
		it('resets a previous refusal before running', async () => {
			const submission = new FormSubmission();
			submission.errors = [{ message: 'stale' }];
			const held = Promise.withResolvers<void>();

			const run = submission.run(() => held.promise, orServiceProblem);

			expect(submission.errors).toEqual([]);
			held.resolve();
			await run;
		});

		it('is busy for the length of fn, and clears on success', async () => {
			const submission = new FormSubmission();
			const held = Promise.withResolvers<void>();

			const run = submission.run(() => held.promise, orServiceProblem);

			expect(submission.isSubmitting).toBe(true);
			held.resolve();
			await run;

			expect(submission.isSubmitting).toBe(false);
			expect(submission.errors).toEqual([]);
		});

		it('leaves errors empty when fn succeeds', async () => {
			const submission = new FormSubmission();

			await submission.run(async () => {}, orServiceProblem);

			expect(submission.errors).toEqual([]);
		});

		it('maps a thrown refusal through mapRefusal', async () => {
			const submission = new FormSubmission();

			await submission.run(
				() => {
					throw new Error('The practice was not found');
				},
				orThrownMessage
			);

			expect(submission.errors).toEqual([{ message: 'The practice was not found' }]);
			expect(submission.isSubmitting).toBe(false);
		});

		it('maps a rejected fn through mapRefusal the same way', async () => {
			const submission = new FormSubmission();

			await submission.run(
				() => Promise.reject(new Error('The practice was not found')),
				orThrownMessage
			);

			expect(submission.errors).toEqual([{ message: 'The practice was not found' }]);
		});

		it('maps a returned refusal through mapRefusal', async () => {
			const submission = new FormSubmission();
			const refusal = { code: 'not-found' };
			const mapRefusal = vi.fn().mockReturnValue([{ message: 'Not found' }]);

			await submission.run(async () => refusal, mapRefusal);

			expect(mapRefusal).toHaveBeenCalledWith(refusal);
			expect(submission.errors).toEqual([{ message: 'Not found' }]);
		});

		it('awaits an async mapRefusal before publishing errors', async () => {
			const submission = new FormSubmission();
			const held = Promise.withResolvers<FormError[]>();

			const run = submission.run(
				async () => ({ status: 400 }),
				() => held.promise
			);

			expect(submission.errors).toEqual([]);
			held.resolve([{ message: 'Refused' }]);
			await run;

			expect(submission.errors).toEqual([{ message: 'Refused' }]);
		});

		it('ignores a second call while one is in flight', async () => {
			const held = Promise.withResolvers<void>();
			const function_ = vi.fn().mockReturnValue(held.promise);
			const submission = new FormSubmission();

			const first = submission.run(function_, orServiceProblem);
			await submission.run(function_, orServiceProblem);

			expect(function_).toHaveBeenCalledTimes(1);
			expect(submission.isSubmitting).toBe(true);

			held.resolve();
			await first;

			expect(submission.isSubmitting).toBe(false);
		});

		it('lets a new run start once the previous one has cleared', async () => {
			const submission = new FormSubmission();

			await submission.run(async () => {}, orServiceProblem);
			await submission.run(
				() => {
					throw new Error('second failure');
				},
				orThrownMessage
			);

			expect(submission.errors).toEqual([{ message: 'second failure' }]);
		});
	});
});

describe('orServiceProblem', () => {
	it('passes an already-built FormError[] through unchanged', () => {
		const errors = [{ message: 'Enter your email address', targetId: 'email' }];

		expect(orServiceProblem(errors)).toBe(errors);
	});

	it('reads anything else as the generic service problem', () => {
		expect(orServiceProblem(new Error('boom'))).toEqual([
			{ message: 'There is a problem with the service. Try again in a few minutes.' }
		]);
	});
});

describe('orThrownMessage', () => {
	it('passes an already-built FormError[] through unchanged', () => {
		const errors = [{ message: 'Enter your email address', targetId: 'email' }];

		expect(orThrownMessage(errors)).toBe(errors);
	});

	it("surfaces a thrown Error's own message", () => {
		expect(orThrownMessage(new Error('The practice was not found'))).toEqual([
			{ message: 'The practice was not found' }
		]);
	});

	it('falls back to the generic service problem for a non-Error throw', () => {
		expect(orThrownMessage('offline')).toEqual([
			{ message: 'There is a problem with the service. Try again in a few minutes.' }
		]);
	});

	it('falls back to the generic service problem for an Error with no message', () => {
		const error = new Error('temporary');
		error.message = '';

		expect(orThrownMessage(error)).toEqual([
			{ message: 'There is a problem with the service. Try again in a few minutes.' }
		]);
	});
});
