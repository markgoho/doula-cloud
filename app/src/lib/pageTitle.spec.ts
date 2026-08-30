import { describe, expect, it } from 'vitest';
import { formatPageTitle } from './pageTitle.js';

describe('formatPageTitle', () => {
	it('defaults the service name to the product name', () => {
		expect(formatPageTitle('Clients')).toBe('Clients — Doula Cloud');
	});

	it('uses the given service name, for the Practice-named portal', () => {
		expect(formatPageTitle('Your care', { serviceName: 'Riverside Doulas' })).toBe(
			'Your care — Riverside Doulas'
		);
	});

	it('prefixes Error: for a genuinely refused page', () => {
		expect(formatPageTitle('Log in', { isError: true })).toBe('Error: Log in — Doula Cloud');
	});

	it('does not prefix Error: by default', () => {
		expect(formatPageTitle('Log in', { isError: false })).toBe('Log in — Doula Cloud');
	});
});
