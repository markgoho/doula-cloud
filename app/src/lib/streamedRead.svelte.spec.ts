import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { jsonResponse } from './testResponse.js';

const apiFetch = vi.hoisted(() => vi.fn());
const didFollowSessionRefusal = vi.hoisted(() => vi.fn(async () => false));
vi.mock('./api.js', () => ({ apiFetch, didFollowSessionRefusal }));

const { Streamed, bothRead, mapRead, streamRead } = await import('./streamedRead.svelte.js');
type StreamedRead<Value> = import('./streamedRead.svelte.js').StreamedRead<Value>;

afterEach(() => {
	apiFetch.mockReset();
	didFollowSessionRefusal.mockReset();
	didFollowSessionRefusal.mockImplementation(async () => false);
});

describe('streamRead', () => {
	it('reads the body of a response that landed', async () => {
		apiFetch.mockResolvedValue(jsonResponse({ balance: 3 }));

		await expect(streamRead('/api/practices/practice-1/billing')).resolves.toEqual({
			isLoaded: true,
			value: { balance: 3 }
		});
		expect(apiFetch).toHaveBeenCalledWith('/api/practices/practice-1/billing');
	});

	it('carries a refused response through unread', async () => {
		const response = jsonResponse('no', 403);
		apiFetch.mockResolvedValue(response);

		await expect(streamRead('/api/practices/practice-1/billing')).resolves.toEqual({ isLoaded: false, response });
	});

	// A hovered link that was never clicked leaves its read unconsumed, so
	// a fetch that throws must not reject.
	it('reads a fetch that threw as a failure with no response', async () => {
		apiFetch.mockRejectedValue(new TypeError('offline'));

		await expect(streamRead('/api/practices/practice-1/billing')).resolves.toEqual({ isLoaded: false });
	});
});

function landed<Value>(value: Value): Promise<StreamedRead<Value>> {
	return Promise.resolve({ isLoaded: true, value });
}

function refused(status: number): Promise<StreamedRead<never>> {
	return Promise.resolve({ isLoaded: false, response: jsonResponse('no', status) });
}

describe('bothRead', () => {
	it('lands both values together', async () => {
		await expect(bothRead(landed(1), landed('two'))).resolves.toEqual({ isLoaded: true, value: [1, 'two'] });
	});

	it.each([
		['the first', refused(403), landed(2), 403],
		['the second', landed(1), refused(404), 404],
		['the first of two', refused(403), refused(404), 403]
	])('fails with %s failure', async (_which, first, second, status) => {
		await expect(bothRead(first, second)).resolves.toMatchObject({ isLoaded: false, response: { status } });
	});
});

describe('mapRead', () => {
	it('shapes a value that landed', async () => {
		await expect(mapRead(landed(2), (value) => value * 2)).resolves.toEqual({
			isLoaded: true,
			value: 4
		});
	});

	it('passes a failure through', async () => {
		const failed = { isLoaded: false } as const;

		await expect(mapRead(Promise.resolve(failed), () => 'never')).resolves.toBe(failed);
	});
});

/**
A `Streamed` following whatever read `current` holds, as a screen's would.
*/
function follow<Value>(first: Promise<StreamedRead<Value>>) {
	const source = $state({ current: first });
	let streamed!: InstanceType<typeof Streamed<Value>>;
	const stop = $effect.root(() => {
		streamed = new Streamed(() => source.current);
	});
	flushSync();
	return {
		streamed,
		next: (read: Promise<StreamedRead<Value>>) => {
			source.current = read;
			flushSync();
		},
		stop
	};
}

describe('Streamed', () => {
	it('holds nothing while the read runs, then the value it lands', async () => {
		const read = Promise.withResolvers<StreamedRead<number>>();
		const { streamed, stop } = follow(read.promise);

		expect(streamed.value).toBeUndefined();
		expect(streamed.failure).toBeUndefined();

		read.resolve({ isLoaded: true, value: 7 });
		await vi.waitFor(() => expect(streamed.value).toBe(7));
		stop();
	});

	// A narrowing or an invalidation runs `load` again: the screen keeps
	// its rows until the next read lands, rather than dropping back to its
	// loading state.
	it('keeps the last value while a later read runs', async () => {
		const { streamed, next, stop } = follow(Promise.resolve({ isLoaded: true as const, value: 1 }));
		await vi.waitFor(() => expect(streamed.value).toBe(1));

		const later = Promise.withResolvers<StreamedRead<number>>();
		next(later.promise);
		expect(streamed.value).toBe(1);

		later.resolve({ isLoaded: true, value: 2 });
		await vi.waitFor(() => expect(streamed.value).toBe(2));
		stop();
	});

	it('ignores a read that was overtaken', async () => {
		const overtaken = Promise.withResolvers<StreamedRead<number>>();
		const { streamed, next, stop } = follow(overtaken.promise);
		next(Promise.resolve({ isLoaded: true, value: 2 }));
		await vi.waitFor(() => expect(streamed.value).toBe(2));

		overtaken.resolve({ isLoaded: true, value: 1 });
		await Promise.resolve();
		expect(streamed.value).toBe(2);
		stop();
	});

	it('draws a refusal as the ErrorPage kind its reason names', async () => {
		const { streamed, stop } = follow<number>(
			Promise.resolve({ isLoaded: false, response: Response.json({ code: 'PRACTICE_PENDING_DELETION' }, { status: 403 }) })
		);

		await vi.waitFor(() => expect(streamed.failure).toBe('practiceLocked'));
		stop();
	});

	it.each([
		[404, 'notFound'],
		[500, 'problem']
	])('draws a %i as %s', async (status, kind) => {
		const { streamed, stop } = follow<number>(Promise.resolve({ isLoaded: false, response: new Response('', { status }) }));

		await vi.waitFor(() => expect(streamed.failure).toBe(kind));
		stop();
	});

	it("keeps the refusal's own code and sentence beside its kind (#2270)", async () => {
		const { streamed, stop } = follow<number>(
			Promise.resolve({
				isLoaded: false,
				response: Response.json({ code: 'ATTACHMENT_ENDED', message: 'Your work ended.' }, { status: 410 })
			})
		);

		await vi.waitFor(() => expect(streamed.refusal).toEqual({ code: 'ATTACHMENT_ENDED', message: 'Your work ended.' }));
		stop();
	});

	it('draws a read that never reached the server as a problem', async () => {
		const { streamed, stop } = follow<number>(Promise.resolve({ isLoaded: false }));

		await vi.waitFor(() => expect(streamed.failure).toBe('problem'));
		stop();
	});

	it('drops the rows it held when a later read fails', async () => {
		const { streamed, next, stop } = follow(Promise.resolve({ isLoaded: true as const, value: 1 }));
		await vi.waitFor(() => expect(streamed.value).toBe(1));

		next(Promise.resolve({ isLoaded: false }));

		await vi.waitFor(() => expect(streamed.failure).toBe('problem'));
		expect(streamed.value).toBeUndefined();
		stop();
	});

	it('clears a failure once a later read lands', async () => {
		const { streamed, next, stop } = follow<number>(Promise.resolve({ isLoaded: false }));
		await vi.waitFor(() => expect(streamed.failure).toBe('problem'));

		next(Promise.resolve({ isLoaded: true, value: 3 }));

		await vi.waitFor(() => expect(streamed.value).toBe(3));
		expect(streamed.failure).toBeUndefined();
		stop();
	});

	// An ended session, a second factor, a Terms agreement: the browser is
	// leaving, so the screen keeps its loading state rather than drawing
	// the refusal for a frame.
	it('stays loading while a session refusal sends the browser elsewhere', async () => {
		didFollowSessionRefusal.mockImplementation(async () => true);
		const response = new Response('', { status: 401 });
		const { streamed, stop } = follow<number>(Promise.resolve({ isLoaded: false, response }));

		await vi.waitFor(() => expect(didFollowSessionRefusal).toHaveBeenCalledWith(response));
		await Promise.resolve();
		expect(streamed.failure).toBeUndefined();
		expect(streamed.value).toBeUndefined();
		stop();
	});

	it('draws nothing for a failure that was overtaken while it was read', async () => {
		const refusal = Promise.withResolvers<boolean>();
		didFollowSessionRefusal.mockImplementationOnce(() => refusal.promise);
		const { streamed, next, stop } = follow<number>(
			Promise.resolve({ isLoaded: false, response: new Response('', { status: 403 }) })
		);
		await vi.waitFor(() => expect(didFollowSessionRefusal).toHaveBeenCalledOnce());

		next(Promise.resolve({ isLoaded: true, value: 4 }));
		await vi.waitFor(() => expect(streamed.value).toBe(4));
		refusal.resolve(false);
		await new Promise((resolve) => setTimeout(resolve, 0));

		expect(streamed.failure).toBeUndefined();
		expect(streamed.value).toBe(4);
		stop();
	});
});
