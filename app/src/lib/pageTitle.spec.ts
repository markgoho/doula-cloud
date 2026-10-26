import { describe, expect, it } from 'vitest';
import { formatPageTitle } from './pageTitle.js';

describe('formatPageTitle', () => {
	it('defaults the service name to the product name', () => {
		expect(formatPageTitle('Clients')).toBe('Clients — DoulaCloud');
	});

	it('uses the given service name, for the Practice-named portal', () => {
		expect(formatPageTitle('Your care', { serviceName: 'Riverside Doulas' })).toBe(
			'Your care — Riverside Doulas'
		);
	});

	it('prefixes Error: for a genuinely refused page', () => {
		expect(formatPageTitle('Sign in', { isError: true })).toBe('Error: Sign in — DoulaCloud');
	});

	it('does not prefix Error: by default', () => {
		expect(formatPageTitle('Sign in', { isError: false })).toBe('Sign in — DoulaCloud');
	});
});
