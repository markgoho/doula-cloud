import { describe, expect, it } from 'vitest';
import {
	authRefusal,
	isMultiFactorAuthRequired,
	passwordReauthRefusal,
	refusalErrors,
	refusalMessage,
	refusalOrConfirmable,
	SERVICE_PROBLEM,
	totpCodeRefusal
} from './formErrors.js';
import { jsonResponse } from './testResponse.js';

const FIELDS = { emailId: 'email', passwordId: 'password' };

/*
 * `formErrors.usage.spec.ts` cannot read this module: it holds Identity
 * Platform's own error codes, and `auth/invalid-credential` is an
 * identifier we do not own. So the wording rules are asserted here
 * instead, over every message the module can actually return -- which is
 * stricter than a grep, because it checks the strings a reader sees
 * rather than the strings the file contains.
 */
const BANNED = /\b(please|valid|invalid|required)\b/i;

const EVERY_CODE = [
	'auth/invalid-credential',
	'auth/invalid-login-credentials',
	'auth/wrong-password',
	'auth/user-not-found',
	'auth/invalid-email',
	'auth/email-already-in-use',
	'auth/weak-password',
	'auth/user-disabled',
	'auth/too-many-requests',
	'auth/network-request-failed',
	'auth/something-we-have-never-seen'
];

describe('authRefusal', () => {
	it('says the same thing for every way a sign-in can be wrong', async () => {
		// One message for all four, on purpose: naming which half was wrong
		// tells anyone who asks whether an address has an account here.
		const messages = [
			'auth/invalid-credential',
			'auth/invalid-login-credentials',
			'auth/wrong-password',
			'auth/user-not-found'
		].map((code) => authRefusal({ code }, FIELDS));

		for (const refusal of messages) {
			expect(refusal.message).toBe('The email address or password is not correct');
			expect(refusal.targetId).toBe('email');
		}
	});

	it('sends a malformed address to the email field', () => {
		const refusal = authRefusal({ code: 'auth/invalid-email' }, FIELDS);

		expect(refusal.message).toBe(
			'Enter an email address in the correct format, like name@example.com'
		);
		expect(refusal.targetId).toBe('email');
	});

	it('sends an address that already has an account to the email field', () => {
		const refusal = authRefusal({ code: 'auth/email-already-in-use' }, FIELDS);

		expect(refusal.message).toContain('already has an account');
		expect(refusal.targetId).toBe('email');
	});

	it('sends a short password to the password field', () => {
		const refusal = authRefusal({ code: 'auth/weak-password' }, FIELDS);

		expect(refusal.message).toBe('Password must be 6 characters or more');
		expect(refusal.targetId).toBe('password');
	});

	/*
	 * Rate limiting, a disabled account and a dead connection are all true
	 * of the attempt rather than of an answer on the form, so none of them
	 * points at a control: sending a reader to a field she filled in
	 * correctly is a lie about what is wrong.
	 */
	it.each(['auth/user-disabled', 'auth/too-many-requests', 'auth/network-request-failed'])(
		'names no field for %s',
		(code) => {
			expect(authRefusal({ code }, FIELDS).targetId).toBeUndefined();
		}
	);

	it('falls back to the service message for a code we have never seen', () => {
		expect(authRefusal({ code: 'auth/brand-new' }, FIELDS).message).toBe(SERVICE_PROBLEM);
	});

	// A thrown string, a null, an Error with no code: all of them arrive
	// here, and none of them may crash the handler that is reporting a
	// failure.
	it.each([[undefined], ['a string'], [new Error('boom')], [{ code: 42 }]])(
		'falls back to the service message for %s',
		(cause) => {
			expect(authRefusal(cause, FIELDS).message).toBe(SERVICE_PROBLEM);
		}
	);

	it('never writes a banned word, whatever the code', () => {
		const offences = EVERY_CODE.map((code) => authRefusal({ code }, FIELDS).message).filter(
			(message) => BANNED.test(message)
		);

		expect(offences).toEqual([]);
	});

	it('never writes a banned word in the service message', () => {
		expect(BANNED.test(SERVICE_PROBLEM)).toBe(false);
	});
});

describe('passwordReauthRefusal', () => {
	const passwordId = 'password';

	it.each(['auth/invalid-credential', 'auth/wrong-password'])(
		'sends %s to the password field',
		(code) => {
			const refusal = passwordReauthRefusal({ code }, passwordId);

			expect(refusal.message).toBe('Password is not correct');
			expect(refusal.targetId).toBe(passwordId);
		}
	);

	// Rate limiting and a dead connection are both true of the attempt
	// rather than of the password itself, so neither points at the field.
	it.each(['auth/too-many-requests', 'auth/network-request-failed'])(
		'names no field for %s',
		(code) => {
			expect(passwordReauthRefusal({ code }, passwordId).targetId).toBeUndefined();
		}
	);

	it('falls back to the service message for a code it has never seen', () => {
		expect(passwordReauthRefusal({ code: 'auth/brand-new' }, passwordId).message).toBe(
			SERVICE_PROBLEM
		);
	});

	it('never writes a banned word, whatever the code', () => {
		const offences = [...EVERY_CODE, 'auth/brand-new']
			.map((code) => passwordReauthRefusal({ code }, passwordId).message)
			.filter((message) => BANNED.test(message));

		expect(offences).toEqual([]);
	});
});

describe('totpCodeRefusal', () => {
	const codeId = 'code';

	it('sends a wrong or expired code to the code field', () => {
		const refusal = totpCodeRefusal({ code: 'auth/invalid-verification-code' }, codeId);

		expect(refusal.message).toBe(
			'The code is not correct. Enter the 6-digit code from your authenticator app.'
		);
		expect(refusal.targetId).toBe(codeId);
	});

	it.each(['auth/too-many-requests', 'auth/network-request-failed'])(
		'names no field for %s',
		(code) => {
			expect(totpCodeRefusal({ code }, codeId).targetId).toBeUndefined();
		}
	);

	it('falls back to the service message for a code it has never seen', () => {
		expect(totpCodeRefusal({ code: 'auth/brand-new' }, codeId).message).toBe(SERVICE_PROBLEM);
	});

	it('never writes a banned word, whatever the code', () => {
		const offences = [...EVERY_CODE, 'auth/brand-new']
			.map((code) => totpCodeRefusal({ code }, codeId).message)
			.filter((message) => BANNED.test(message));

		expect(offences).toEqual([]);
	});
});

describe('isMultiFactorAuthRequired', () => {
	it('recognizes Identity Platform’s own code for a second-factor challenge', () => {
		expect(isMultiFactorAuthRequired({ code: 'auth/multi-factor-auth-required' })).toBe(true);
	});

	it.each([[undefined], ['a string'], [new Error('boom')], [{ code: 'auth/wrong-password' }]])(
		'reads %s as anything else',
		(cause) => {
			expect(isMultiFactorAuthRequired(cause)).toBe(false);
		}
	);
});

describe('refusalMessage', () => {
	/*
	 * A 4xx is the server saying something true about this submission, and
	 * it is the only thing that knows -- so its own words are shown. The
	 * BFF writes them two ways today (see api.ts), and this reads either.
	 */
	it('shows the server’s words for a refusal it alone knows the reason for', async () => {
		expect(await refusalMessage(jsonResponse('That address is already invited', 409))).toBe(
			'That address is already invited'
		);
	});

	it('reads the message out of a structured error body', async () => {
		expect(
			await refusalMessage(jsonResponse({ code: 'INVALID_ARGUMENT', message: 'The token has expired' }, 400))
		).toBe('The token has expired');
	});

	// A 5xx is ours. Whatever it says about itself, it says nothing the
	// reader can act on.
	it('owns a server fault rather than repeating it', async () => {
		expect(await refusalMessage(jsonResponse('sql: no rows in result set', 500))).toBe(
			SERVICE_PROBLEM
		);
	});

	it('owns an empty refusal, which tells the reader nothing at all', async () => {
		expect(await refusalMessage(jsonResponse('', 403))).toBe(SERVICE_PROBLEM);
	});

	it('falls back to the raw text when the body is neither JSON nor empty', async () => {
		expect(await refusalMessage(jsonResponse('engagement not found', 404))).toBe(
			'engagement not found'
		);
	});

	it('falls back to the raw text when the JSON carries no message', async () => {
		expect(await refusalMessage(jsonResponse({ code: 'INVALID_ARGUMENT' }, 400))).toBe(
			'{"code":"INVALID_ARGUMENT"}'
		);
	});
});

/*
 * #488's half of #467's own gap: `details` is a field-keyed map
 * (docs/api-design.md section 7), so a refusal that names fields maps
 * each one onto the route's own control id -- an unmapped key still
 * reports its message, just with no link to send the reader to.
 */
describe('refusalErrors', () => {
	const FIELD_IDS = { inviteToken: 'invite-token-field' };

	it('reads one FormError per details entry, targeted at the route’s control', async () => {
		const errors = await refusalErrors(
			jsonResponse(
				{ code: 'INVALID_ARGUMENT', message: 'invalid request body', details: { inviteToken: 'Enter the invitation link you were sent' } },
				400
			),
			FIELD_IDS
		);

		expect(errors).toEqual([
			{ message: 'Enter the invitation link you were sent', targetId: 'invite-token-field' }
		]);
	});

	it('leaves a details key with no matching control untargeted rather than dropping it', async () => {
		const errors = await refusalErrors(
			jsonResponse({ code: 'INVALID_ARGUMENT', message: 'refused', details: { someUnmappedField: 'Enter a value' } }, 400),
			FIELD_IDS
		);

		expect(errors).toEqual([{ message: 'Enter a value', targetId: undefined }]);
	});

	it('falls back to one untargeted entry when there is no details', async () => {
		const errors = await refusalErrors(jsonResponse('That address is already invited', 409));

		expect(errors).toEqual([{ message: 'That address is already invited' }]);
	});

	it('falls back to one untargeted entry when details is empty', async () => {
		const errors = await refusalErrors(
			jsonResponse({ code: 'INVALID_ARGUMENT', message: 'refused', details: {} }, 400)
		);

		expect(errors).toEqual([{ message: 'refused' }]);
	});

	it('reports the service problem as one untargeted entry for a 5xx', async () => {
		const errors = await refusalErrors(jsonResponse('sql: no rows in result set', 500));

		expect(errors).toEqual([{ message: SERVICE_PROBLEM }]);
	});

	it('reports the service problem as one untargeted entry for an empty body', async () => {
		const errors = await refusalErrors(jsonResponse('', 403));

		expect(errors).toEqual([{ message: SERVICE_PROBLEM }]);
	});

	it('defaults fieldIds to empty when the caller has no fields to target', async () => {
		const errors = await refusalErrors(
			jsonResponse({ code: 'INVALID_ARGUMENT', message: 'refused', details: { email: 'Enter an email address' } }, 400)
		);

		expect(errors).toEqual([{ message: 'Enter an email address', targetId: undefined }]);
	});
});

describe('refusalOrConfirmable', () => {
	/*
	 * #610's cross-population refusal, told apart by its code rather than
	 * by its prose: the page renders this as a warning above a
	 * press-through button, never in the error summary.
	 */
	it('reads a 409 SESSION_EVICTION_UNCONFIRMED as something to press through', async () => {
		const refusal = await refusalOrConfirmable(
			jsonResponse(
				{
					code: 'SESSION_EVICTION_UNCONFIRMED',
					message: 'Continuing signs you out of your Practice in this browser.'
				},
				409
			)
		);

		expect(refusal).toEqual({
			kind: 'confirmable',
			message: 'Continuing signs you out of your Practice in this browser.'
		});
	});

	it('reads a 409 carrying any other code as an error', async () => {
		const refusal = await refusalOrConfirmable(
			jsonResponse({ code: 'CONFLICT', message: 'That address is already invited' }, 409)
		);

		expect(refusal).toEqual({
			kind: 'errors',
			errors: [{ message: 'That address is already invited' }]
		});
	});

	it('reads the eviction code on any other status as an error', async () => {
		const refusal = await refusalOrConfirmable(
			jsonResponse({ code: 'SESSION_EVICTION_UNCONFIRMED', message: 'Add a website first' }, 400)
		);

		expect(refusal).toEqual({ kind: 'errors', errors: [{ message: 'Add a website first' }] });
	});

	/*
	 * The reason the eviction refusal got its own code rather than reusing
	 * FAILED_PRECONDITION: three unrelated 409s already carry that one
	 * (payments/connect, client/erase), and every one of them has to stay
	 * an error rather than a button she can press through.
	 */
	it('reads a 409 FAILED_PRECONDITION as an error, not a press-through', async () => {
		const refusal = await refusalOrConfirmable(
			jsonResponse({ code: 'FAILED_PRECONDITION', message: 'Settle the open invoice first' }, 409)
		);

		expect(refusal).toEqual({
			kind: 'errors',
			errors: [{ message: 'Settle the open invoice first' }]
		});
	});

	it('reads a 409 with no message at all as an error, never as an empty warning', async () => {
		const refusal = await refusalOrConfirmable(jsonResponse({ code: 'SESSION_EVICTION_UNCONFIRMED' }, 409));

		expect(refusal).toEqual({
			kind: 'errors',
			errors: [{ message: '{"code":"SESSION_EVICTION_UNCONFIRMED"}' }]
		});
	});

	it('maps details onto the route’s controls, same as refusalErrors', async () => {
		const refusal = await refusalOrConfirmable(
			jsonResponse(
				{ code: 'INVALID_ARGUMENT', message: 'refused', details: { token: 'Enter the code' } },
				400
			),
			{ token: 'token-field' }
		);

		expect(refusal).toEqual({
			kind: 'errors',
			errors: [{ message: 'Enter the code', targetId: 'token-field' }]
		});
	});

	it('reports the service problem for a 5xx', async () => {
		const refusal = await refusalOrConfirmable(jsonResponse('boom', 500));

		expect(refusal).toEqual({ kind: 'errors', errors: [{ message: SERVICE_PROBLEM }] });
	});
});
