/**
 * How an address and a phone number read on a screen (#2147).
 *
 * Both are stored exactly as they were typed; this is display only, the
 * way `dates.ts` turns a stored "1988-02-09" into words. Every screen that
 * shows either one goes through here, so the check page, the record and
 * the intake name page cannot read one value two ways.
 */

export interface AddressParts {
	addressLine1?: string;
	addressLine2?: string;
	addressLocality?: string;
	addressRegion?: string;
	addressPostalCode?: string;
}

/**
 * An address on one line, the way a US address is written (USPS
 * Publication 28): "12 Park Avenue, Apt 2, Rochester, NY 14607". Commas
 * between the lines and between the city and the state, and a space, not a
 * comma, before the ZIP code. A part left empty leaves no separator
 * behind. Nothing at all gives "", so a caller keeps its own fallback.
 */
function part(value: string | undefined): string {
	return (value ?? '').trim();
}

export function formatAddress(address: AddressParts): string {
	const cityAndState = [part(address.addressLocality), part(address.addressRegion)].filter(Boolean).join(', ');
	const lastLine = [cityAndState, part(address.addressPostalCode)].filter(Boolean).join(' ');
	return [part(address.addressLine1), part(address.addressLine2), lastLine].filter(Boolean).join(', ');
}

/*
 * What may sit between the digits of a US number as people type it:
 * spaces, parentheses, hyphens, dots and slashes. Anything else -- a
 * letter, an "x" or "ext" before an extension, a "+" that is not "+1" --
 * means the number is not one this can read safely, so it is shown as
 * typed.
 */
const SEPARATORS = /[\s().\-/]/g;

/*
 * Ten digits in the North American Numbering Plan's shape: neither the
 * area code nor the exchange starts with a 0 or a 1. A ten-digit number
 * that breaks that rule is not a US number, whatever its length.
 */
const NANP = /^([2-9]\d{2})([2-9]\d{2})(\d{4})$/;

/**
 * A phone number as a person reads it. A US number -- ten digits, or
 * eleven with a leading 1 or +1, and nothing between them but spaces,
 * parentheses, hyphens, dots or slashes -- reads "(585) 555-0142",
 * whichever way it was typed. Every other number (another country's, one
 * with an extension, one too short) is shown exactly as typed, trimmed:
 * GOV.UK's Phone numbers pattern accepts any format, and a number this
 * cannot read is never rewritten.
 */
export function formatPhoneNumber(phone: string): string {
	const typed = phone.trim();
	const withoutCountryCode = typed.startsWith('+1') ? typed.slice(2) : typed;
	if (withoutCountryCode.includes('+')) return typed;
	const stripped = withoutCountryCode.replaceAll(SEPARATORS, '');
	if (!/^\d+$/.test(stripped)) return typed;
	const digits = stripped.length === 11 && stripped.startsWith('1') ? stripped.slice(1) : stripped;
	const match = NANP.exec(digits);
	if (!match) return typed;
	return `(${match[1]}) ${match[2]}-${match[3]}`;
}
