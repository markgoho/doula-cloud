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
 * A commonly used password is refused too (#1539). The list it is checked
 * against is a chunk of its own (commonPasswords.ts) that only those three
 * screens load, and the comparison runs in the browser: nothing about the
 * password is sent anywhere to check it. Identity Platform has no such
 * list, so on /signup and /accept-invite this check is the only one; on
 * /reset-password the BFF makes it again (staffauth/reset.go), because
 * that request already carries the password to the Admin SDK.
 */
export const MIN_PASSWORD_LENGTH = 15;

/*
The line under the field, shown before she types.
*/
export const PASSWORD_HINT = `Must be ${MIN_PASSWORD_LENGTH} characters or more, and not a commonly used password`;

/*
The refusal, and the sentence the error summary links to the field.
*/
export const PASSWORD_TOO_SHORT = `Password must be ${MIN_PASSWORD_LENGTH} characters or more`;

export function isPasswordTooShort(password: string): boolean {
	// Code points, not UTF-16 units, so the browser counts the way the BFF does.
	return [...password].length < MIN_PASSWORD_LENGTH;
}

/*
The refusal for a password on the list, in GOV.UK's error-message form: the
field's own noun, what is wrong, and what to do.
*/
export const PASSWORD_TOO_COMMON = 'Password is too common. Enter a different password';

// One load per page, shared by the preload and every submit after it.
const loading: { list?: Promise<ReadonlySet<string>> } = {};

async function importCommonPasswords(): Promise<ReadonlySet<string>> {
	try {
		const module = await import('./commonPasswords.js');
		return module.COMMON_PASSWORDS;
	} catch (error) {
		// Forget the failure, so the next submit tries the chunk again
		// rather than being refused for one dropped connection forever.
		loading.list = undefined;
		throw error;
	}
}

function loadCommonPasswords(): Promise<ReadonlySet<string>> {
	loading.list ??= importCommonPasswords();
	return loading.list;
}

/*
Starts loading the list while she fills in the form, so the submit does not
wait for it. Never rejects: a failure here is not reported, because the
submit loads the list again and reports a failure there.
*/
export async function preloadCommonPasswords(): Promise<void> {
	try {
		await loadCommonPasswords();
	} catch {
		// Reported by the submit, not here.
	}
}

/*
Whether the whole password, in any letter case, is on the list. Rejects when
the list cannot be loaded: the screen then refuses the submit and says the
service has a problem, rather than let an unchecked password through (block
over warn).
*/
export async function isPasswordTooCommon(password: string): Promise<boolean> {
	const list = await loadCommonPasswords();
	return list.has(password.toLowerCase());
}
