import { describe, expect, it } from 'vitest';
import { load } from './+page.js';

describe('an address under a Practice that no route answers', () => {
	it('is a 404, so the Staff shell renders it', () => {
		expect(() => load({} as Parameters<typeof load>[0])).toThrow(expect.objectContaining({ status: 404 }));
	});
});
