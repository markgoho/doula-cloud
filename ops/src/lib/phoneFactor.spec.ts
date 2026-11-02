import { describe, expect, it } from 'vitest';
import { codeListURL, isPhoneHint } from './phoneFactor.ts';

describe('isPhoneHint', () => {
	it('is true for the emulator phone factor and false for the authenticator app', () => {
		expect(isPhoneHint({ factorId: 'phone' })).toBe(true);
		expect(isPhoneHint({ factorId: 'totp' })).toBe(false);
	});
});

describe('codeListURL', () => {
	it('names where the emulator lists the operator tenant codes', () => {
		expect(codeListURL('127.0.0.1:9099', 'operators-local')).toBe(
			'http://127.0.0.1:9099/emulator/v1/projects/doula-cloud/tenants/operators-local/verificationCodes'
		);
	});

	it('is undefined with no emulator host, so a deployed build offers nothing', () => {
		expect(codeListURL(undefined, 'operators-local')).toBeUndefined();
		expect(codeListURL('', 'operators-local')).toBeUndefined();
	});
});
