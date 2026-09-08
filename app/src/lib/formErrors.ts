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
 * The two things worth reading out of a refused response: a message for
 * the submission as a whole, and -- where the BFF names one -- the
 * field-keyed map `docs/api-design.md` section 7 calls `APIError.details`.
 * undefined means the refusal itself carries nothing a reader can act on: a
 * 5xx, or a body that came back empty. Shared by `refusalMessage` and
 * `refusalErrors` so the 5xx/empty/JSON split is written once.
 *
 * Exported (#840): this is the one reader of the `{code, message, details}`
 * envelope. `apiErrorMessage.ts` imports it to build its own one-line
 * projection rather than re-reading the body itself -- that file stays
 * `$app`-free either way, since this one is too (its only import is a
 * type from `ErrorSummary.svelte`).
 *
 * `opaque5xx` (default true) is the "5xx split" the module comment
 * above promises: a 5xx or an empty body reads as `undefined` --
 * nothing a reader can act on -- for every caller in *this* file. One
 * caller outside it disagrees on purpose: `emailSuppression.ts`'s
 * `clearEmailSuppression` reads a failed Mailgun call's 502 body
 * (`{message: "...; nothing was changed"}`) through `apiErrorMessage`
 * rather than `refusalMessage`, because that clause is the one thing a
 * person needs after a failed attempt, and collapsing every 5xx into
 * `SERVICE_PROBLEM` would lose it. `apiErrorMessage.ts` passes
 * `opaque5xx: false` for exactly that reason -- still the one reader of
 * the envelope, just without this file's own opinion on what a 5xx is
 * worth showing.
 */
export interface ParsedRefusal {
	message?: string;
	code?: string;
	details?: Record<string, string>;
	text: string;
}

// Overloaded on a literal `opaque5xx: false` so `apiErrorMessage.ts` gets
// back a plain `ParsedRefusal`, never `undefined` -- both of this
// function's `return undefined` branches below are gated on `opaque5xx`,
// so passing `false` makes that half of the return type unreachable, and
// the overload lets the type system say so instead of a caller needing a
// non-null assertion for a branch that can't happen.
export async function parseRefusal(
	response: Response,
	options: { opaque5xx: false }
): Promise<ParsedRefusal>;
export async function parseRefusal(
	response: Response,
	options?: { opaque5xx?: boolean }
): Promise<ParsedRefusal | undefined>;
export async function parseRefusal(
	response: Response,
	{ opaque5xx = true }: { opaque5xx?: boolean } = {}
): Promise<ParsedRefusal | undefined> {
	if (opaque5xx && response.status >= 500) return undefined;

	const text = await response.text();
	if (opaque5xx && text.trim() === '') return undefined;

	try {
		const parsed: unknown = JSON.parse(text);
		if (parsed !== null && typeof parsed === 'object') {
			const message =
				'message' in parsed && typeof parsed.message === 'string' ? parsed.message : undefined;
			const code = 'code' in parsed && typeof parsed.code === 'string' ? parsed.code : undefined;
			const rawDetails =
				'details' in parsed && parsed.details !== null && typeof parsed.details === 'object'
					? (parsed.details as Record<string, unknown>)
					: undefined;
			const details = rawDetails
				? Object.fromEntries(
						Object.entries(rawDetails).filter(
							(entry): entry is [string, string] => typeof entry[1] === 'string'
						)
					)
				: undefined;
			return {
				message,
				code,
				details: details && Object.keys(details).length > 0 ? details : undefined,
				text
			};
		}
	} catch {
		// Not JSON -- most endpoints still write plain text (see api.ts).
	}
	return { text };
}

/*
 * Reads a refused response into one message.
 *
 * A 4xx is the server saying something true about *this* submission --
 * that address is already invited, that Invitation has expired -- and it
 * is the only thing that knows, so its own words are shown. A 5xx or an
 * unreadable body is ours, and says so instead. Ignores `details` even
 * where the BFF sends it: a single sentence is what a `Notice` needs,
 * and picking one entry out of several field refusals would show less
 * than the server said. `refusalErrors` is where `details` is read.
 */
export async function refusalMessage(response: Response): Promise<string> {
	const parsed = await parseRefusal(response);
	if (parsed === undefined) return SERVICE_PROBLEM;
	return parsed.message ?? parsed.text;
}

/*
 * Reads a refused response into the list `ErrorSummary` wants.
 *
 * Where the BFF names fields (`APIError.details`, docs/api-design.md
 * section 7), each one becomes its own entry, mapped through fieldIds
 * onto the control it is about -- a key with no entry in fieldIds still
 * reports its message, just with no link, the same way `authRefusal`
 * leaves a whole-submission refusal untargeted. Most endpoints don't
 * populate `details` yet (tracked on #488), so most callers see the same
 * single untargeted entry `refusalMessage` would have shown; a route
 * that passes fieldIds keeps working once its endpoint catches up,
 * with no further change on the client.
 */
export async function refusalErrors(
	response: Response,
	fieldIds: Record<string, string> = {}
): Promise<FormError[]> {
	return toErrors(await parseRefusal(response), fieldIds);
}

function toErrors(
	parsed: ParsedRefusal | undefined,
	fieldIds: Record<string, string>
): FormError[] {
	if (parsed === undefined) return [{ message: SERVICE_PROBLEM }];

	return targetedErrors(parsed.message ?? parsed.text, parsed.details, fieldIds);
}

/*
 * The one mapping from a refusal onto the list `ErrorSummary` wants:
 * `details` becomes one entry per field, and a refusal that names no
 * field stays the single untargeted entry it has always been. Shared by
 * `toErrors` and `errorsFromCause` so the two ways a refusal reaches a
 * screen -- read straight off a `Response`, or thrown by a lib module and
 * caught -- cannot come to disagree about what a `details` map means.
 */
function targetedErrors(
	message: string,
	details: Record<string, string> | undefined,
	fieldIds: Record<string, string>
): FormError[] {
	if (details) {
		return Object.entries(details).map(([field, detail]) => ({
			message: detail,
			targetId: fieldIds[field]
		}));
	}
	return [{ message }];
}

/*
 * A refusal a lib module throws rather than returns (#754).
 *
 * `client.ts` and `engagementRequest.ts` do their fetching for a screen
 * and report a refusal by throwing, which is what let their three
 * `ErrorSummary` routes -- `clients/new`, `clients/[clientId]/edit` and
 * `engagement-requests/new` -- lose `details` before they could ever read
 * it: an `Error` carries one string. This carries the field map beside
 * the message, so those routes can target a refusal the same way the
 * `refusalErrors` routes already do.
 *
 * A subclass rather than a second return shape on every one of those
 * functions: the throwing is the existing contract those modules' many
 * callers are written against, and most of them (a load that fails, a
 * withdrawal) have no form to target anything onto. `errorsFromCause`
 * degrades to the message for every one of those, so nothing else has to
 * change.
 */
export class RefusalError extends Error {
	readonly details?: Record<string, string>;

	constructor(message: string, details?: Record<string, string>) {
		super(message);
		this.name = 'RefusalError';
		this.details = details;
	}
}

/*
 * Reads a refused response into the `RefusalError` a lib module throws.
 *
 * `opaque5xx: false`, matching `apiErrorMessage` -- the function this
 * replaces at those call sites -- so a 5xx still reports its own sentence
 * where it has one rather than collapsing to `SERVICE_PROBLEM`. A 5xx
 * carries no `details`, so nothing is targeted off one either way.
 */
export async function refusalError(response: Response): Promise<RefusalError> {
	const parsed = await parseRefusal(response, { opaque5xx: false });
	return new RefusalError(parsed.message ?? parsed.text, parsed.details);
}

/*
 * Reads whatever a `catch` caught into the list `ErrorSummary` wants.
 *
 * A `RefusalError` naming fields becomes one targeted entry each; any
 * other `Error` -- including a `RefusalError` for a refusal that named no
 * field -- stays the single untargeted entry those screens show today.
 * Something thrown that is not an `Error` at all is ours rather than
 * hers, and says so.
 */
export function errorsFromCause(
	cause: unknown,
	fieldIds: Record<string, string> = {}
): FormError[] {
	if (cause instanceof RefusalError) {
		return targetedErrors(cause.message, cause.details, fieldIds);
	}
	if (cause instanceof Error) return [{ message: cause.message }];
	return [{ message: SERVICE_PROBLEM }];
}

/*
 * What a refusal turned out to be: something to fix, or something to
 * press through.
 *
 * `confirmableCode` is which press-through this caller is reading for,
 * and it defaults to the sign-in one below because that is where the
 * shape started. #943 is the second: `BIRTH_OUTCOME_FROZEN` is the same
 * arrangement -- nothing is wrong with what she submitted, the same
 * request re-sent by an Owner with `correction: true` goes through, and
 * the words shown are the BFF's own. A caller names its own code rather
 * than this function matching every press-through code it knows,
 * because a screen that pressed through a refusal it was not expecting
 * would be overriding a rule it has never read.
 *
 * Three screens sign a person in behind a Continue-style button, and all
 * three can be refused by #610's cross-population check -- a browser
 * holds exactly one Doula Cloud session, so signing into the second
 * population evicts the first. That refusal is not a failure: nothing is
 * wrong with what she submitted, and the same submit sent again with
 * `X-Confirmed` goes through. It is announced as a `WarningText` above
 * the button rather than in the `ErrorSummary`, which is what the two
 * `kind`s here are for.
 *
 * Read off the BFF's `APIError.code`, never its prose (#692), and off a
 * code that means only this: `FAILED_PRECONDITION` already carries three
 * unrelated 409s (payments/connect, client/erase), so matching it would
 * make any of them render as a press-through the moment one reached one
 * of these screens. `apierr.CodeSessionEvictionUnconfirmed` was added for
 * that reason, per docs/api-design.md section 7's own rule. The words
 * shown are still the BFF's own, for the reason `refusalMessage` gives --
 * the server is the only thing that knows which population she is about
 * to leave.
 */
export type Refusal =
	| { kind: 'confirmable'; message: string }
	| { kind: 'errors'; errors: FormError[] };

export async function refusalOrConfirmable(
	response: Response,
	fieldIds: Record<string, string> = {},
	confirmableCode = 'SESSION_EVICTION_UNCONFIRMED'
): Promise<Refusal> {
	const parsed = await parseRefusal(response);
	if (response.status === 409 && parsed?.code === confirmableCode && parsed.message) {
		return { kind: 'confirmable', message: parsed.message };
	}
	return { kind: 'errors', errors: toErrors(parsed, fieldIds) };
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
 * unrecognized code is a fault we have not seen, and telling somebody to
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

/*
 * Identity Platform's refusal for the password re-entry step MFA
 * enrolment and removal both use (#606) -- a step-up reauthentication,
 * not the original sign-in, so there is no email field to blame and no
 * account-enumeration concern (she is already signed in as herself).
 */
export function passwordReauthRefusal(cause: unknown, passwordId: string): FormError {
	const code = authErrorCode(cause);
	switch (code) {
		case 'auth/invalid-credential':
		case 'auth/wrong-password': {
			return { message: 'Password is not correct', targetId: passwordId };
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

/*
 * A wrong or expired TOTP code, at either the enrolment step (confirming
 * a freshly scanned authenticator) or the sign-in challenge (#606's AC:
 * "fails as a sign-in failure ... not an app error"). TOTP has no
 * separate "expired" code the way SMS does (`auth/code-expired`) -- a
 * stale code from an app whose clock has drifted, or one entered a
 * window too late, both come back as `auth/invalid-verification-code`,
 * so one message covers both.
 */
export function totpCodeRefusal(cause: unknown, codeId: string): FormError {
	const code = authErrorCode(cause);
	switch (code) {
		case 'auth/invalid-verification-code': {
			return {
				message: 'The code is not correct. Enter the 6-digit code from your authenticator app.',
				targetId: codeId
			};
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

/*
 * Whether Identity Platform refused this because the address already has
 * an account.
 *
 * Signup asks (#745). An account whose signup half-landed -- the
 * Identity Platform account created, `POST /api/staff/signup` refused --
 * hits exactly this code on the retry, and the retry is the fix rather
 * than the failure: the screen signs in with the same credential and
 * finishes the half that is missing. `authRefusal` still owns what the
 * reader is told when that sign-in does not work.
 */
export function isEmailAlreadyInUse(cause: unknown): boolean {
	return authErrorCode(cause) === 'auth/email-already-in-use';
}

/*
 * Whether Identity Platform is waiting on a second factor to finish a
 * sign-in (#606) -- what `signInWithEmailAndPassword` throws for an
 * enrolled person, before any ID token exists. It is not a refusal, so
 * it carries no message; the login screen's only use for it is deciding
 * whether to open the TOTP challenge step in place of showing one.
 *
 * Lives here rather than being written inline where it is read: the
 * literal `'auth/multi-factor-auth-required'` is Identity Platform's own
 * identifier, the same category `authErrorCode`'s callers already read,
 * and `formErrors.usage.spec.ts` excludes this file from its banned-word
 * scan for exactly that reason -- an identifier containing "required" is
 * not the user-facing word the scan exists to catch.
 */
export function isMultiFactorAuthRequired(cause: unknown): boolean {
	return authErrorCode(cause) === 'auth/multi-factor-auth-required';
}

function authErrorCode(cause: unknown): string {
	if (cause !== null && typeof cause === 'object' && 'code' in cause && typeof cause.code === 'string') {
		return cause.code;
	}
	return '';
}
