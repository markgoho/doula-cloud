/*
 * The words a refused submit is reported in (#467).
 *
 * GOV.UK's error-message rules are short and absolute: say what went
 * wrong and what to do about it, start with the field's own noun, and
 * never write "please", "valid", "invalid" or "required". Two specs
 * enforce them: `formErrors.spec.ts` asserts every message this module
 * can return, and `formErrors.usage.spec.ts` reads every component and
 * route. A banned word fails the suite rather than reaching a person.
 *
 * Messages live here rather than in each route for the reason the
 * summary itself exists: the same refusal has to read the same way
 * wherever it happens. Four screens sign a person in, and "the email
 * address or password is not correct" drifting into four wordings is
 * the defect this pattern is for.
 */
export type { FormError } from './components/molecules/ErrorSummary.svelte';

import type { FormError } from './components/molecules/ErrorSummary.svelte';

/*
 * What the reader is told when the failure is ours and there is nothing
 * she can do about it -- a 5xx, a dropped connection, a response we
 * could not read. It names no field, so it carries no `targetId`: there
 * is no control on the page to send her to.
 */
export const SERVICE_PROBLEM = 'There is a problem with the service. Try again in a few minutes.';

/*
 * Reads a refused response into one message.
 *
 * A 4xx is the server saying something true about *this* submission --
 * that address is already invited, that Invitation has expired -- and it
 * is the only thing that knows, so its own words are shown. A 5xx or an
 * unreadable body is ours, and says so instead.
 *
 * The BFF has no per-field shape to read: `docs/api-design.md` section 7
 * defines `APIError.details` as a field-keyed map, and exactly one
 * package (`website`) writes it. Everything else returns one string or
 * plain text, so a per-field summary from a server refusal cannot be
 * built today without parsing prose. Tracked as its own API ticket
 * rather than guessed at here -- see the resolution comment on #467.
 */
export async function refusalMessage(response: Response): Promise<string> {
	if (response.status >= 500) return SERVICE_PROBLEM;

	const text = await response.text();
	if (text.trim() === '') return SERVICE_PROBLEM;

	try {
		const parsed: unknown = JSON.parse(text);
		if (
			parsed !== null &&
			typeof parsed === 'object' &&
			'message' in parsed &&
			typeof parsed.message === 'string'
		) {
			return parsed.message;
		}
	} catch {
		// Not JSON -- most endpoints still write plain text (see api.ts).
	}
	return text;
}

/*
 * Identity Platform's own error codes, in the reader's words.
 *
 * The SDK throws strings like "Firebase: Error (auth/invalid-credential)."
 * -- a product name, a code, and the banned word. Four screens catch
 * them, and until now each answered with one flat "Login failed" that
 * said nothing about which of the two fields to look at. This maps the
 * codes a person can actually cause onto a message and, where one
 * exists, the control that caused it.
 *
 * The catch-all is deliberately not "check your details": an
 * unrecognised code is a fault we have not seen, and telling somebody to
 * re-read a correct form is worse than admitting that.
 */
export function authRefusal(
	cause: unknown,
	fields: { emailId: string; passwordId: string }
): FormError {
	const code = authErrorCode(cause);

	switch (code) {
		case 'auth/invalid-credential':
		case 'auth/invalid-login-credentials':
		case 'auth/wrong-password':
		case 'auth/user-not-found': {
			// One message for all four, on purpose: naming which half was
			// wrong tells anyone who asks whether an address has an account
			// here, and a Client's Practice is a fact about her pregnancy.
			return {
				message: 'The email address or password is not correct',
				targetId: fields.emailId
			};
		}
		case 'auth/invalid-email': {
			return {
				message: 'Enter an email address in the correct format, like name@example.com',
				targetId: fields.emailId
			};
		}
		case 'auth/email-already-in-use': {
			return {
				message: 'This email address already has an account. Log in instead.',
				targetId: fields.emailId
			};
		}
		case 'auth/weak-password': {
			return {
				message: 'Password must be 6 characters or more',
				targetId: fields.passwordId
			};
		}
		case 'auth/user-disabled': {
			return { message: 'This account has been turned off. Ask an Owner to let you back in.' };
		}
		case 'auth/too-many-requests': {
			return { message: 'Too many attempts from this device. Wait a few minutes and try again.' };
		}
		case 'auth/network-request-failed': {
			return { message: 'We could not reach the service. Check your connection and try again.' };
		}
		default: {
			return { message: SERVICE_PROBLEM };
		}
	}
}

function authErrorCode(cause: unknown): string {
	if (cause !== null && typeof cause === 'object' && 'code' in cause && typeof cause.code === 'string') {
		return cause.code;
	}
	return '';
}
