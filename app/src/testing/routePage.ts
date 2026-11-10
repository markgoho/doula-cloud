import { vi } from 'vitest';

export const pageState: {
	params: Record<string, string>;
	url: URL;
	data: Record<string, unknown>;
	state: object;
} = {
	params: {},
	url: new URL('https://example.test/'),
	data: {},
	state: {}
};

export { pageState as page };

export const goto = vi.fn();

/**
`#lib/api.js`'s one signed-in fetch, for a route spec that mocks that module with this one.
*/
export const apiFetchWithSession = vi.fn();
