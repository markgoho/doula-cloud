import { describe, expect, it } from 'vitest';
import { errorKindForStatus } from './errorPage.js';

describe('errorKindForStatus', () => {
	it('maps 404 to notFound', () => {
		expect(errorKindForStatus(404)).toBe('notFound');
	});

	it('maps 403 to refused', () => {
		expect(errorKindForStatus(403)).toBe('refused');
	});

	it('maps 503 to unavailable', () => {
		expect(errorKindForStatus(503)).toBe('unavailable');
	});

	it('maps 500 to problem', () => {
		expect(errorKindForStatus(500)).toBe('problem');
	});

	it('maps any other unexpected status to problem', () => {
		expect(errorKindForStatus(418)).toBe('problem');
	});
});
