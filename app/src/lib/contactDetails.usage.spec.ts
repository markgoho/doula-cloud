import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { globFiles } from './globFiles';

/*
 * #2147's gate, the same shape as `dates.usage.spec.ts`'s date-of-birth
 * rule (#2008). `contactDetails.ts` decides how an address and a phone
 * number read on one line, and the check page and the record each used to
 * decide it for themselves: both joined every address line with ", ", so a
 * comma fell before the ZIP code, and each printed the phone number raw.
 *
 * A raw value reaching the screen is a value, not a call, so what this can
 * find reliably is the shape that happened, in a `.svelte` file:
 *
 * - a read of `.addressRegion` or `.addressPostalCode` that is not only
 *   tested for presence. A screen that names the state or the ZIP code
 *   itself is composing an address; `formatAddress` takes the whole record
 *   instead. The address question names its boxes by quoted key, which
 *   this does not read.
 * - a read of `.phone` that is not handed to `formatPhoneNumber`, tested
 *   for presence, an input's own `value={…}` (what she is typing, shown as
 *   she types it), or carried into a variable by assignment (the search
 *   filling its own box back in).
 *
 * It does not read a `.ts` file, where a draft carries the stored value on
 * to the BFF. A deliberate exception stays by putting `contact:ignore` and
 * the reason on the line.
 */
const IGNORE_MARKER = 'contact:ignore';
const CHAIN = /[\w$]+(?:!?\??\.[\w$]+)*/g;
const ADDRESS_PART = /!?\??\.address(?:Region|PostalCode)$/;
// `.phone.trim` too: the name page printed `answers.phone.trim()`.
const PHONE = /!?\??\.phone(?:\.trim)?$/;
const NEGATED_BEFORE = /!\s*$/;
const TESTED_AFTER = /^\s*(?:\?(?![?.])|&&)/;
const INPUT_VALUE_BEFORE = /value=\{\s*$/;
const ASSIGNED_BEFORE = /(?:^|[^=!<>])=\s*$/;

// Inside a call to `formatter` that has not closed yet, however deep.
function isInsideCall(before: string, formatter: string): boolean {
	const opened = before.lastIndexOf(formatter);
	if (opened === -1) return false;
	const call = before.slice(opened + formatter.length - 1);
	let depth = 0;
	for (const character of call) {
		if (character === '(') depth += 1;
		else if (character === ')') depth -= 1;
		if (depth === 0) return false;
	}
	return true;
}

interface Offense {
	file: string;
	line: number;
}

function findRawContactDetails(file: string, source: string): Offense[] {
	return source.split('\n').flatMap((text, index) => {
		if (text.includes(IGNORE_MARKER)) return [];
		const isRaw = text.matchAll(CHAIN).some((match) => {
			const isAddressPart = ADDRESS_PART.test(match[0]);
			const isPhone = PHONE.test(match[0]);
			if (!isAddressPart && !isPhone) return false;
			const before = text.slice(0, match.index);
			const after = text.slice(match.index + match[0].length);
			if (NEGATED_BEFORE.test(before) || TESTED_AFTER.test(after)) return false;
			if (isAddressPart) return true;
			return (
				!isInsideCall(before, 'formatPhoneNumber(') &&
				!INPUT_VALUE_BEFORE.test(before) &&
				!ASSIGNED_BEFORE.test(before)
			);
		});
		return isRaw ? [{ file, line: index + 1 }] : [];
	});
}

const appRoot = fileURLToPath(new URL('../../', import.meta.url));
const svelteFiles = globFiles('src/**/*.svelte', { cwd: appRoot });
const offenses = svelteFiles.flatMap((file) =>
	findRawContactDetails(file, readFileSync(path.join(appRoot, file), 'utf8'))
);

describe('an address and a phone number reach the screen through contactDetails.ts (#2147)', () => {
	it('reads the screens that print one', () => {
		expect(svelteFiles).toContain('src/routes/practices/[practiceId]/clients/[clientId]/+page.svelte');
		expect(svelteFiles).toContain('src/routes/practices/[practiceId]/clients/new/name/+page.svelte');
	});

	it('finds an address composed on the page and a phone number printed raw', () => {
		expect(findRawContactDetails('a.svelte', '[d.addressLocality, d.addressRegion].join(", ")')).toEqual([
			{ file: 'a.svelte', line: 1 }
		]);
		expect(findRawContactDetails('a.svelte', '\n<dd>{detail!.addressPostalCode}</dd>')).toEqual([
			{ file: 'a.svelte', line: 2 }
		]);
		expect(findRawContactDetails('a.svelte', "changeable('Phone number', detail!.phone || '—')")).toEqual([
			{ file: 'a.svelte', line: 1 }
		]);
		expect(findRawContactDetails('a.svelte', "{ label: 'Phone number', value: draft.answers.phone.trim() }")).toEqual([
			{ file: 'a.svelte', line: 1 }
		]);
		expect(findRawContactDetails('a.svelte', '<dd>{x.phone}</dd>')).toEqual([{ file: 'a.svelte', line: 1 }]);
		expect(findRawContactDetails('a.svelte', "formatPhoneNumber(x.y) + (z.phone ?? '')")).toEqual([
			{ file: 'a.svelte', line: 1 }
		]);
	});

	it('leaves a formatted number, an input value, an assignment, a presence test and a marked line', () => {
		expect(findRawContactDetails('a.svelte', "formatPhoneNumber(detail!.phone) || '—'")).toEqual([]);
		expect(findRawContactDetails('a.svelte', 'value: formatPhoneNumber(String(d.answers.phone))')).toEqual([]);
		expect(findRawContactDetails('a.svelte', 'value={journey.draft.answers.phone}')).toEqual([]);
		expect(findRawContactDetails('a.svelte', 'phone = typed.phone;')).toEqual([]);
		expect(findRawContactDetails('a.svelte', 'if (!fields.email && !fields.phone) {')).toEqual([]);
		expect(findRawContactDetails('a.svelte', "x.addressRegion ? 'yes' : 'no'")).toEqual([]);
		expect(findRawContactDetails('a.svelte', 'x.phone && call()')).toEqual([]);
		expect(findRawContactDetails('a.svelte', '{x.phone} <!-- ' + IGNORE_MARKER + ': a demo -->')).toEqual([]);
		expect(findRawContactDetails('a.svelte', "{ key: 'addressPostalCode', label: 'ZIP code' }")).toEqual([]);
	});

	it('finds no address or phone number printed around contactDetails.ts', () => {
		expect(
			offenses.map(
				(offense) =>
					`${offense.file}:${offense.line} prints an address part or a phone number as it is stored -- use formatAddress or formatPhoneNumber, or put ${IGNORE_MARKER} and a reason on the line`
			)
		).toEqual([]);
	});
});
