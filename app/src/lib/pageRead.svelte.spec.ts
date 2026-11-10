import { describe, expect, it } from 'vitest';

import { bothReads, PageRead, pageReadState, readFailure } from './pageRead.svelte.js';

function deferred<Value>() {
	return Promise.withResolvers<Value>();
}

describe('readFailure', () => {
	it("uses a thrown Error's own message", () => {
		expect(readFailure(new Error('Not found'), 'Failed to load')).toBe('Not found');
	});

	// A failure is never drawn as an empty Notice.
	it('falls back for an Error with no message, and for anything else thrown', () => {
		const silent = new Error('placeholder');
		silent.message = '';
		expect(readFailure(silent, 'Failed to load')).toBe('Failed to load');
		expect(readFailure('offline', 'Failed to load')).toBe('Failed to load');
	});
});

describe('pageReadState', () => {
	it('names what is loading, says why a read failed, and says nothing more once loaded', () => {
		expect(pageReadState({ status: 'loading' }, 'Loading your rates')).toEqual({
			status: 'loading',
			label: 'Loading your rates'
		});
		expect(pageReadState({ status: 'failed', message: 'Down' }, 'Loading your rates')).toEqual({
			status: 'failed',
			message: 'Down'
		});
		expect(pageReadState({ status: 'loaded', value: 1 }, 'Loading your rates')).toEqual({ status: 'loaded' });
	});
});

describe('bothReads', () => {
	it('is the first failure, then loading while either loads, then both values', () => {
		const loading = { status: 'loading' } as const;
		const failed = { status: 'failed', message: 'Down' } as const;
		const loaded = { status: 'loaded', value: 1 } as const;

		expect(bothReads(failed, { status: 'failed', message: 'Other' })).toEqual(failed);
		expect(bothReads(loading, failed)).toEqual(failed);
		expect(bothReads(loaded, loading)).toEqual(loading);
		expect(bothReads(loading, loaded)).toEqual(loading);
		expect(bothReads(loaded, { status: 'loaded', value: 'two' })).toEqual({ status: 'loaded', value: [1, 'two'] });
	});
});

describe('PageRead', () => {
	it('starts loading, with no value', () => {
		const read = new PageRead<string>();

		expect(read.state).toEqual({ status: 'loading' });
		expect(read.value).toBeUndefined();
		expect(read.isLoaded).toBe(false);
		expect(read.frame('Loading')).toEqual({ status: 'loading', label: 'Loading' });
	});

	describe('load', () => {
		it('is loaded with what the read resolved to', async () => {
			const read = new PageRead<string>();

			expect(await read.load(async () => 'rates', 'Failed to load rates')).toBe(true);

			expect(read.state).toEqual({ status: 'loaded', value: 'rates' });
			expect(read.value).toBe('rates');
			expect(read.isLoaded).toBe(true);
			expect(read.frame('Loading')).toEqual({ status: 'loaded' });
		});

		// The timezone screen drew a failed read as a loaded, empty form.
		it('fails, and is not loaded, when the read throws', async () => {
			const read = new PageRead<string>();

			const isWritten = await read.load(async () => {
				throw new Error('The Practice was not found');
			}, 'Failed to load');

			expect(isWritten).toBe(false);
			expect(read.state).toEqual({ status: 'failed', message: 'The Practice was not found' });
			expect(read.isLoaded).toBe(false);
			expect(read.value).toBeUndefined();
		});

		it('words every failure itself when given a function of what was thrown', async () => {
			const read = new PageRead<string>();

			await read.load(async () => {
				throw new TypeError('Failed to fetch');
			}, () => 'There is a problem with the service.');

			expect(read.state).toEqual({ status: 'failed', message: 'There is a problem with the service.' });
		});

		it('goes back to loading while a fresh read runs', async () => {
			const read = new PageRead<string>();
			await read.load(async () => 'first', 'Failed');
			const pending = deferred<string>();

			const loading = read.load(() => pending.promise, 'Failed');
			expect(read.state).toEqual({ status: 'loading' });
			pending.resolve('second');
			await loading;

			expect(read.value).toBe('second');
		});

		// clientDetailsFlow: a read for the record open before must not
		// overwrite the one open now.
		it('drops a read that settles after a later one started', async () => {
			const read = new PageRead<string>();
			const first = deferred<string>();
			const second = deferred<string>();

			const firstLoad = read.load(() => first.promise, 'Failed');
			const secondLoad = read.load(() => second.promise, 'Failed');
			second.resolve('B');
			first.resolve('A');

			expect(await secondLoad).toBe(true);
			expect(await firstLoad).toBe(false);
			expect(read.value).toBe('B');
		});

		it('drops a stale failure too', async () => {
			const read = new PageRead<string>();
			const first = deferred<string>();

			const firstLoad = read.load(() => first.promise, 'Failed');
			await read.load(async () => 'B', 'Failed');
			first.reject(new Error('A is gone'));

			expect(await firstLoad).toBe(false);
			expect(read.state).toEqual({ status: 'loaded', value: 'B' });
		});
	});

	describe('refresh', () => {
		it('keeps the page drawn while it runs, then holds the new value', async () => {
			const read = new PageRead<string>();
			await read.load(async () => 'before', 'Failed');
			const pending = deferred<string>();

			const refreshing = read.refresh(() => pending.promise, 'Failed');
			expect(read.state).toEqual({ status: 'loaded', value: 'before' });
			pending.resolve('after');

			expect(await refreshing).toBe(true);
			expect(read.value).toBe('after');
		});

		// settings/mfa: a save's re-read that failed replaced the page,
		// and the save's own success notice with it.
		it('leaves a loaded page loaded on failure, and says why', async () => {
			const read = new PageRead<string>();
			await read.load(async () => 'before', 'Failed');

			const isWritten = await read.refresh(async () => {
				throw new Error('Down');
			}, 'Failed to load this setting');

			expect(isWritten).toBe(false);
			expect(read.state).toEqual({ status: 'loaded', value: 'before' });
			expect(read.refreshError).toBe('Down');
		});

		it('clears the last failure when it starts', async () => {
			const read = new PageRead<string>();
			await read.load(async () => 'before', 'Failed');
			await read.refresh(async () => {
				throw 'offline';
			}, 'Failed to load this setting');
			expect(read.refreshError).toBe('Failed to load this setting');

			await read.refresh(async () => 'after', 'Failed');

			expect(read.refreshError).toBe('');
		});

		it('fails a page that never loaded', async () => {
			const read = new PageRead<string>();

			await read.refresh(async () => {
				throw new Error('Down');
			}, 'Failed');

			expect(read.state).toEqual({ status: 'failed', message: 'Down' });
			expect(read.refreshError).toBe('');
		});
	});

	describe('set, fail and reset', () => {
		it('writes a save’s own answer, dropping a read in flight', async () => {
			const read = new PageRead<string>();
			const pending = deferred<string>();
			const loading = read.load(() => pending.promise, 'Failed');

			read.set('signed');
			pending.resolve('stale');

			expect(await loading).toBe(false);
			expect(read.value).toBe('signed');
		});

		it('fails with a sentence the route found itself', () => {
			const read = new PageRead<string>();

			read.fail('Choose the kind of answer first.');

			expect(read.frame('Loading')).toEqual({ status: 'failed', message: 'Choose the kind of answer first.' });
		});

		it('goes back to loading, dropping a read in flight', async () => {
			const read = new PageRead<string>();
			const pending = deferred<string>();
			const loading = read.load(() => pending.promise, 'Failed');

			read.reset();
			pending.resolve('stale');

			expect(await loading).toBe(false);
			expect(read.state).toEqual({ status: 'loading' });
		});
	});
});
