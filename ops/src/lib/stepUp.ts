import { codeFieldError, codeRefusal, credentialRefusal, type FormError } from './authRefusal.ts';
import { refusalMessage } from './session.ts';

export const STEP_UP_PASSWORD_FIELD = 'step-up-password';
export const STEP_UP_CODE_FIELD = 'step-up-code';

export type StepUpAnswers = { email: string; password: string; code: string };

export type StepUpDependencies = {
	freshIdToken: (email: string, password: string, code: string) => Promise<string>;
	fetcher?: typeof fetch;
};

const PASSWORD_REFUSALS = new Set(['auth/invalid-credential', 'auth/invalid-login-credentials', 'auth/wrong-password']);

function refusalOf(error: unknown): FormError {
	const code = typeof error === 'object' && error !== null && 'code' in error ? error.code : undefined;
	if (typeof code === 'string' && PASSWORD_REFUSALS.has(code)) {
		return { message: 'Enter the correct password', targetId: STEP_UP_PASSWORD_FIELD };
	}
	if (code === 'auth/too-many-requests') return credentialRefusal(error, STEP_UP_PASSWORD_FIELD);
	return codeRefusal(error, STEP_UP_CODE_FIELD);
}

// A sign-in Identity Platform refused is still sent, with no token, so doula-ops records the try.
export async function stepUp(answers: StepUpAnswers, { freshIdToken, fetcher = fetch }: StepUpDependencies): Promise<FormError[]> {
	const code = answers.code.replaceAll(/\s/g, '');
	const empty: FormError[] = [];
	if (answers.password === '') empty.push({ message: 'Enter your password', targetId: STEP_UP_PASSWORD_FIELD });
	const codeError = codeFieldError(code, STEP_UP_CODE_FIELD);
	if (codeError) empty.push(codeError);
	if (empty.length > 0) return empty;

	let idToken: string;
	try {
		idToken = await freshIdToken(answers.email, answers.password, code);
	} catch (error) {
		try {
			await fetcher('/api/session/step-up', { method: 'POST', credentials: 'include' });
		} catch {
			// The refusal is still the operator's to see when its record cannot be sent.
		}
		return [refusalOf(error)];
	}
	const response = await fetcher('/api/session/step-up', {
		method: 'POST',
		credentials: 'include',
		headers: { Authorization: `Bearer ${idToken}` }
	});
	return response.ok ? [] : [{ message: await refusalMessage(response) }];
}
