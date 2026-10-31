import { describe, expect, it } from 'vitest';
import { readReturnTo, withReturnTo } from './returnTo';

function loginWith(target: string): URL {
	const url = new URL('https://app.example.test/login');
	url.searchParams.set('returnTo', target);
	return url;
}

describe('readReturnTo', () => {
	it('hands back a path on this app, with its query and fragment', () => {
		expect(readReturnTo(loginWith('/practices/p-1/engagements/e-1/birth-plan?view=print#top'))).toBe(
			'/practices/p-1/engagements/e-1/birth-plan?view=print#top'
		);
	});

	it('reads an address with none as none', () => {
		expect(readReturnTo(new URL('https://app.example.test/login'))).toBeUndefined();
		expect(readReturnTo(loginWith(''))).toBeUndefined();
	});

	it.each([
		['a protocol-relative host', '//evil.example.com/practices'],
		['a backslash the browser folds into one', String.raw`/\evil.example.com`],
		['a tab hidden in the slashes', '/\t/evil.example.com'],
		['an absolute address', 'https://evil.example.com/practices'],
		['a script', 'javascript:alert(1)'],
		['a relative path', 'practices/p-1']
	])('refuses %s', (_, target) => {
		expect(readReturnTo(loginWith(target))).toBeUndefined();
	});

	it('refuses the screen reading it, so sign-in cannot send her back to sign-in', () => {
		expect(readReturnTo(loginWith('/login?sessionEnded=true'))).toBeUndefined();
	});
});

describe('withReturnTo', () => {
	it('adds returnTo to an address with no query', () => {
		expect(withReturnTo('/login', '/practices/p-1?x=1')).toBe('/login?returnTo=%2Fpractices%2Fp-1%3Fx%3D1');
	});

	it('keeps a query the address already has', () => {
		expect(withReturnTo('/login?sessionEnded=true', '/practices/p-1')).toBe(
			'/login?sessionEnded=true&returnTo=%2Fpractices%2Fp-1'
		);
	});

	it('is read back by readReturnTo', () => {
		const url = new URL(withReturnTo('/login', '/practices/p-1#birth-plan'), 'https://app.example.test');

		expect(readReturnTo(url)).toBe('/practices/p-1#birth-plan');
	});
});
