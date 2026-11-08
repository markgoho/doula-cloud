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
