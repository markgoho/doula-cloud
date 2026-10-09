/*
 * The rule for a Staff password, written once (#1538, ADR-0026's amendment
 * for #1493): 15 characters or more, no composition rule, spaces allowed,
 * paste allowed (#470). Identity Platform's password policy enforces the
 * same number on its side; the browser says it before she types and
 * refuses a short one before the request leaves.
 *
 * Only a screen that SETS a password reads this: /signup, /accept-invite
 * (when it makes a new account) and /reset-password. /login and
 * ReauthPrompt only read an existing password and check nothing about it.
 *
 * To add a requirement (a common-password refusal is the next one, #1539),
 * extend PASSWORD_HINT and add a check beside isPasswordTooShort.
 */
export const MIN_PASSWORD_LENGTH = 15;

/*
The line under the field, shown before she types.
*/
export const PASSWORD_HINT = `Must be ${MIN_PASSWORD_LENGTH} characters or more`;

/*
The refusal, and the sentence the error summary links to the field.
*/
export const PASSWORD_TOO_SHORT = `Password must be ${MIN_PASSWORD_LENGTH} characters or more`;

export function isPasswordTooShort(password: string): boolean {
	return password.length < MIN_PASSWORD_LENGTH;
}
