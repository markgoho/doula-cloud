import { describe, expect, it, vi } from 'vitest';
import { CheckRead } from './checkRead.svelte.ts';
import { SERVICE_PROBLEM } from './session.ts';

describe('CheckRead', () => {
	it('is loading until the read answers, then holds what it read', async () => {
		const read = new CheckRead<string>(vi.fn());
		const answer = Promise.withResolvers<{ kind: 'ok'; value: string }>();

		const loading = read.load(() => answer.promise);
		expect(read.state).toEqual({ status: 'loading' });
		expect(read.value).toBeUndefined();
		answer.resolve({ kind: 'ok', value: 'the check' });
		await loading;

		expect(read.state).toEqual({ status: 'loaded', value: 'the check' });
		expect(read.value).toBe('the check');
		expect(read.errors).toEqual([]);
	});

	it('fails with the refusal', async () => {
		const read = new CheckRead<string>(vi.fn());

		await read.load(async () => ({ kind: 'refused', errors: [{ message: 'That Practice is not on file.' }] }));

		expect(read.state).toEqual({ status: 'failed', errors: [{ message: 'That Practice is not on file.' }] });
		expect(read.errors).toEqual([{ message: 'That Practice is not on file.' }]);
	});

	// A dropped connection was silent before: no heading, no reason (the audit's "load never settles").
	it('fails with a problem with the service when the read throws', async () => {
		const read = new CheckRead<string>(vi.fn());

		await read.load(async () => {
			throw new TypeError('Failed to fetch');
		});

		expect(read.state).toEqual({ status: 'failed', errors: [{ message: SERVICE_PROBLEM }] });
	});

	it('hands back once the session has ended', async () => {
		const onsignedout = vi.fn();
		const read = new CheckRead<string>(onsignedout);

		await read.load(async () => ({ kind: 'signed-out' }));

		expect(read.state).toEqual({ status: 'signed-out' });
		expect(onsignedout).toHaveBeenCalledOnce();
	});

	it('holds a read the page made again', () => {
		const read = new CheckRead<string>(vi.fn());

		read.set('read again');

		expect(read.value).toBe('read again');
	});
});
