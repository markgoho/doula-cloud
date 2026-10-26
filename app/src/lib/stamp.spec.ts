import { describe, expect, it } from 'vitest';
import { stampText } from './stamp.js';

describe('stampText', () => {
	it('reads the day, month first, then who', () => {
		expect(stampText({ at: '2026-08-12T15:00:00Z', by: 'Renata Alvarez' })).toBe('Aug 12, 2026 by Renata Alvarez');
	});
});
