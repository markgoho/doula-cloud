import { describe, expect, it } from 'vitest';
import {
	PASSWORD_HINT,
	PASSWORD_TOO_COMMON,
	PASSWORD_TOO_SHORT,
	isPasswordTooCommon,
	isPasswordTooShort
} from './passwordRule.js';

describe('the Staff password rule', () => {
	it('says the rule before she types and when she breaks it', () => {
		expect(PASSWORD_HINT).toBe('Must be 15 characters or more, and not a commonly used password');
		expect(PASSWORD_TOO_SHORT).toBe('Password must be 15 characters or more');
		expect(PASSWORD_TOO_COMMON).toBe('Password is too common. Enter a different password');
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

describe('the common-password rule (#1539)', () => {
	it('refuses a password on the list', async () => {
		expect(await isPasswordTooCommon('passwordpassword')).toBe(true);
		expect(await isPasswordTooCommon('123456789012345')).toBe(true);
	});

	it('refuses it whatever its letters are in', async () => {
		expect(await isPasswordTooCommon('PasswordPassword')).toBe(true);
	});

	it('compares the whole password, never a part of it', async () => {
		// Each one holds an entry on the list, and none of them is one.
		expect(await isPasswordTooCommon('passwordpassword!')).toBe(false);
		expect(await isPasswordTooCommon('my passwordpassword')).toBe(false);
		expect(await isPasswordTooCommon(' passwordpassword')).toBe(false);
	});

	it('accepts a password that is not on the list', async () => {
		expect(await isPasswordTooCommon('correct horse battery staple')).toBe(false);
		expect(await isPasswordTooCommon('staff-password-123')).toBe(false);
	});
});
