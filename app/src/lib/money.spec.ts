import { describe, expect, it } from 'vitest';
import { formatMoney } from './money.js';

describe('formatMoney', () => {
	it('renders a two-decimal currency from its minor-unit cents', () => {
		expect(formatMoney(2000, 'usd')).toBe('$20.00');
	});

	it('renders a partial-dollar amount', () => {
		expect(formatMoney(550, 'usd')).toBe('$5.50');
	});

	it('accepts an uppercase currency code the same as lowercase', () => {
		expect(formatMoney(2000, 'USD')).toBe(formatMoney(2000, 'usd'));
	});

	it('renders a zero-decimal currency without dividing by 100', () => {
		expect(formatMoney(2000, 'jpy')).toBe('¥2,000');
	});
});
