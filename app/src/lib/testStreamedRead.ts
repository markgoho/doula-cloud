import type { StreamedRead } from './streamedRead.svelte.js';

/**
The read a route's `load` handed over (#2249), once it has settled. For a
`load` spec: `data` is what `load` returned.
*/
export async function settledRead<Value>(data: unknown): Promise<StreamedRead<Value>> {
	return (data as { read: Promise<StreamedRead<Value>> }).read;
}

/**
The value of a read that landed. Undefined for one that failed, which the spec's own assertion then catches.
*/
export async function loadedValue<Value>(data: unknown): Promise<Value> {
	return ((await settledRead<Value>(data)) as { value: Value }).value;
}
