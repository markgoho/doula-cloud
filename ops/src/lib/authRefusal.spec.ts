import { describe, expect, it } from 'vitest';
import { codeFieldError, codeRefusal, credentialRefusal, isSecondFactorRequired } from './authRefusal.ts';
import { emulatorURL } from './firebase.ts';
import { SERVICE_PROBLEM } from './session.ts';

describe('isSecondFactorRequired', () => {
	it('is the code Identity Platform answers a password that still needs a second factor', () => {
		expect(isSecondFactorRequired({ code: 'auth/multi-factor-auth-required' })).toBe(true);
		expect(isSecondFactorRequired({ code: 'auth/invalid-credential' })).toBe(false);
		expect(isSecondFactorRequired(new Error('offline'))).toBe(false);
		expect(isSecondFactorRequired(undefined)).toBe(false);
		expect(isSecondFactorRequired({ code: 7 })).toBe(false);
	});
});

describe('credentialRefusal', () => {
	it.each(['auth/invalid-credential', 'auth/invalid-login-credentials', 'auth/user-not-found', 'auth/wrong-password', 'auth/invalid-email'])(
		'names neither the email nor the password as the wrong one for %s',
		(code) => {
			expect(credentialRefusal({ code }, 'password')).toEqual({
				message: 'Enter a correct email address and password',
				field: 'password'
			});
		}
	);

	it('asks the operator to wait after too many tries', () => {
		expect(credentialRefusal({ code: 'auth/too-many-requests' }, 'password').message).toBe(
			'Too many tries. Wait a few minutes, then try again'
		);
	});

	it('shows anything else as a problem with the service', () => {
		expect(credentialRefusal({ code: 'auth/network-request-failed' }, 'password')).toEqual({ message: SERVICE_PROBLEM });
	});
});

describe('codeRefusal', () => {
	it('asks for the code the app shows now when a code is refused', () => {
		expect(codeRefusal({ code: 'auth/invalid-verification-code' }, 'code')).toEqual({
			message: 'Enter the 6-digit code your authenticator app shows now',
			field: 'code'
		});
		expect(codeRefusal({ code: 'auth/missing-code' }, 'code').field).toBe('code');
	});

	it('says an expired code has expired', () => {
		expect(codeRefusal({ code: 'auth/code-expired' }, 'code').message).toBe(
			'That code has expired. Enter the code your authenticator app shows now'
		);
	});

	it('shows anything else as a problem with the service', () => {
		expect(codeRefusal(new Error('offline'), 'code')).toEqual({ message: SERVICE_PROBLEM });
	});
});

describe('codeFieldError', () => {
	it('asks for a code when there is none', () => {
		expect(codeFieldError('  ', 'code')).toEqual({ message: 'Enter the 6-digit code from your authenticator app', field: 'code' });
	});

	it('asks for 6 digits when the code is not 6 digits', () => {
		expect(codeFieldError('12345', 'code')).toEqual({ message: 'Enter the code as 6 digits', field: 'code' });
	});

	it('takes 6 digits with spaces in them', () => {
		expect(codeFieldError('123 456', 'code')).toBeUndefined();
	});
});

describe('emulatorURL', () => {
	it('builds the emulator URL from its host, and none without one', () => {
		expect(emulatorURL('127.0.0.1:9099')).toBe('http://127.0.0.1:9099');
		expect(emulatorURL(undefined)).toBeUndefined();
	});
});
