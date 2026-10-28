import { apiFetch, didFollowSessionRefusal } from './api.js';
import { errorKindForStatus, type ErrorKind } from './errorPage.js';
import { parseRefusal } from './formErrors.js';

/**
What a route's `load` hands its screen in place of the data itself
(#2249): the read, still running. A `load` that awaited its read held the
old screen up for the whole request after a click; one that returns the
read changes screen on the click, and the screen draws its loading state
until the read lands.

A read that failed carries its response, unread, so the screen can follow
an ended session the way `apiFetchWithSession` does. A read that never
reached the server carries none.
*/
export type StreamedRead<Value> =
	| { isLoaded: true; value: Value }
	| { isLoaded: false; response?: Response };

/**
Reads an API path for a route's `load`, through plain `apiFetch`: a `load`
also runs for a hovered link, which must not navigate anywhere. Never
rejects, so a read nobody consumes -- the hovered link that was never
clicked -- leaves no unhandled rejection behind.
*/
export async function streamRead<Value>(path: string): Promise<StreamedRead<Value>> {
	try {
		const response = await apiFetch(path);
		if (!response.ok) return { isLoaded: false, response };
		return { isLoaded: true, value: (await response.json()) as Value };
	} catch {
		return { isLoaded: false };
	}
}

/**
Two reads as one, for a screen that needs both: the first failure wins.
*/
export async function bothRead<First, Second>(
	first: Promise<StreamedRead<First>>,
	second: Promise<StreamedRead<Second>>
): Promise<StreamedRead<[First, Second]>> {
	const [one, two] = await Promise.all([first, second]);
	if (!one.isLoaded) return one;
	if (!two.isLoaded) return two;
	return { isLoaded: true, value: [one.value, two.value] };
}

/**
The value of a read that landed, shaped for the screen.
*/
export async function mapRead<Value, Mapped>(
	read: Promise<StreamedRead<Value>>,
	map: (value: Value) => Mapped
): Promise<StreamedRead<Mapped>> {
	const settled = await read;
	return settled.isLoaded ? { isLoaded: true, value: map(settled.value) } : settled;
}

/**
A screen's side of a `StreamedRead`. Construct it while the component
initializes: it follows the read it is given, and the next one whenever
`load` runs again (a narrowing, an invalidation).

`value` stays on the last read that landed while a later one runs, so a
screen that has drawn its rows keeps them rather than dropping back to
its loading state. A failure is the `ErrorPage` kind `practices/
+error.svelte` would have drawn for the same response, so a refused,
missing or broken read reads the same as it did when `load` threw it.
*/
export class Streamed<Value> {
	value = $state.raw<Value>();
	failure = $state<ErrorKind>();

	constructor(read: () => Promise<StreamedRead<Value>>) {
		$effect(() => {
			const pending = read();
			let isCurrent = true;
			void this.#settle(pending, () => isCurrent);
			return () => {
				isCurrent = false;
			};
		});
	}

	async #settle(pending: Promise<StreamedRead<Value>>, isCurrent: () => boolean): Promise<void> {
		const read = await pending;
		if (!isCurrent()) return;
		if (read.isLoaded) {
			this.failure = undefined;
			this.value = read.value;
			return;
		}
		const failure = await failureKind(read.response);
		if (failure === undefined || !isCurrent()) return;
		this.value = undefined;
		this.failure = failure;
	}
}

async function failureKind(response: Response | undefined): Promise<ErrorKind | undefined> {
	if (response === undefined) return 'problem';
	if (await didFollowSessionRefusal(response)) return undefined;
	const { code } = await parseRefusal(response, { opaque5xx: false });
	return errorKindForStatus(response.status, code);
}
