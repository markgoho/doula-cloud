import type { FormError } from '$ui/molecules/ErrorSummary.svelte';
import { SERVICE_PROBLEM } from './session.ts';

export type { FormError } from '$ui/molecules/ErrorSummary.svelte';

function codeOf(error: unknown): string | undefined {
	if (typeof error !== 'object' || error === null || !('code' in error)) return undefined;
	return typeof error.code === 'string' ? error.code : undefined;
}

export function isSecondFactorRequired(error: unknown): boolean {
	return codeOf(error) === 'auth/multi-factor-auth-required';
}

// Identity Platform does not say which of the two was wrong, so neither does this.
export function credentialRefusal(error: unknown, passwordField: string): FormError {
	switch (codeOf(error)) {
		case 'auth/invalid-credential':
		case 'auth/invalid-login-credentials':
		case 'auth/user-not-found':
		case 'auth/wrong-password':
		case 'auth/invalid-email': {
			return { message: 'Enter a correct email address and password', targetId: passwordField };
		}
		case 'auth/too-many-requests': {
			return { message: 'Too many tries. Wait a few minutes, then try again', targetId: passwordField };
		}
		default: {
			return { message: SERVICE_PROBLEM };
		}
	}
}

export function codeRefusal(error: unknown, codeField: string): FormError {
	switch (codeOf(error)) {
		case 'auth/invalid-verification-code':
		case 'auth/missing-code': {
			return { message: 'Enter the 6-digit code your authenticator app shows now', targetId: codeField };
		}
		case 'auth/code-expired': {
			return { message: 'That code has expired. Enter the code your authenticator app shows now', targetId: codeField };
		}
		default: {
			return { message: SERVICE_PROBLEM };
		}
	}
}

export function codeFieldError(code: string, codeField: string): FormError | undefined {
	const trimmed = code.replaceAll(/\s/g, '');
	if (trimmed === '') return { message: 'Enter the 6-digit code from your authenticator app', targetId: codeField };
	if (!/^\d{6}$/.test(trimmed)) return { message: 'Enter the code as 6 digits', targetId: codeField };
	return undefined;
}
