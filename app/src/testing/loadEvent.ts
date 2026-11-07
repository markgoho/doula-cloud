import type { Span } from '@opentelemetry/api';
import type { LoadEvent } from '@sveltejs/kit';

const span: Span = {
	spanContext: () => ({ traceId: '', spanId: '', traceFlags: 0 }),
	setAttribute: () => span,
	setAttributes: () => span,
	addEvent: () => span,
	addLink: () => span,
	addLinks: () => span,
	setStatus: () => span,
	updateName: () => span,
	end: () => {},
	isRecording: () => false,
	recordException: () => {}
};

function unexpected(name: string): never {
	throw new Error(`load called ${name}, which this spec did not provide`);
}

const defaults: LoadEvent = {
	params: {},
	// eslint-disable-next-line unicorn/no-null -- SvelteKit's LoadEvent types a route-less event with null
	route: { id: null },
	url: new URL('https://example.test/'),
	fetch: (input, init) => fetch(input, init),
	// eslint-disable-next-line unicorn/no-null -- SvelteKit's LoadEvent types a data-less event with null
	data: null,
	setHeaders: () => {},
	parent: () => unexpected('parent'),
	depends: () => {},
	untrack: (function_) => function_(),
	tracing: { enabled: false, root: span, current: span }
};

/**
 * A complete SvelteKit load event for `load`'s own route types, built from
 * the few fields a spec cares about. An override is checked against the
 * route's real event type; every other field is a harmless default, and
 * `parent` throws when read unset.
 */
export function loadEvent<E extends LoadEvent>(_load: (event: E) => unknown, overrides: NoInfer<Partial<E>>): E {
	return { ...defaults, ...overrides } as E;
}
