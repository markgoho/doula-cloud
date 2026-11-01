import { describe, expect, it } from 'vitest';
import { money } from './money.ts';

describe('money', () => {
	it('writes cents as dollars', () => {
		expect(money(4274)).toBe('$42.74');
	});
});
