import { vi } from 'vitest';

export const pageState: {
	params: Record<string, string>;
	url: URL;
	data: Record<string, unknown>;
} = {
	params: {},
	url: new URL('https://example.test/'),
	data: {}
};

export { pageState as page };

export const goto = vi.fn();
