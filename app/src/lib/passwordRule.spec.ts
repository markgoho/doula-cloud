import { describe, expect, it } from 'vitest';
import { PASSWORD_HINT, PASSWORD_TOO_SHORT, isPasswordTooShort } from './passwordRule.js';

describe('the Staff password rule', () => {
	it('says the rule before she types and when she breaks it', () => {
		expect(PASSWORD_HINT).toBe('Must be 15 characters or more');
		expect(PASSWORD_TOO_SHORT).toBe('Password must be 15 characters or more');
	});

	it('refuses 14 characters and accepts 15', () => {
		expect(isPasswordTooShort('a'.repeat(14))).toBe(true);
		expect(isPasswordTooShort('a'.repeat(15))).toBe(false);
	});

	it('asks nothing of what the characters are', () => {
		expect(isPasswordTooShort('a'.repeat(64))).toBe(false);
		expect(isPasswordTooShort('correct horse battery staple')).toBe(false);
		expect(isPasswordTooShort(' '.repeat(15))).toBe(false);
	});

	it('counts characters, not UTF-16 units', () => {
		expect(isPasswordTooShort('😀'.repeat(8))).toBe(true);
		expect(isPasswordTooShort('😀'.repeat(15))).toBe(false);
	});
});
