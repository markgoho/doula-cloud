// `bun run sync-common-passwords` -- rebuilds the list of commonly used
// passwords a Staff password is refused for (#1539), and writes the same
// bytes to the two places that read it:
//
// - app/src/lib/commonPasswords.txt, which the browser loads, as a chunk of
//   its own, on the three screens that set a password (/signup,
//   /accept-invite, /reset-password);
// - api/internal/staffauth/commonpasswords.txt, which the BFF embeds for
//   the one path that already carries a password, the reset spend.
//
// Do not edit either file by hand: commonPasswords.spec.ts fails the build
// when the two differ. The sources are SecLists (MIT), pinned to one commit
// so a re-run with no edit here writes the same file. To update, move
// SECLISTS_COMMIT to a newer commit and run the script; docs/environment.md
// ("The common-password list") says the same.
//
// Only an entry of MIN_PASSWORD_LENGTH code points or more is kept: a
// shorter password is already refused for its length, so a shorter entry
// could never match and would only make the file bigger. Entries are
// lowercased because the check compares a lowercased password, and kept to
// printable ASCII so the browser's and Go's lowercasing cannot disagree
// about an entry. A `$HEX[...]` line is a cracking tool's encoding of a
// password, not a password anyone types, so it is dropped.

import { MIN_PASSWORD_LENGTH } from '../src/lib/passwordRule.ts';

const SECLISTS_COMMIT = '27c08068f849227f2fc1c8f7f00afae365957b96';

const SOURCES = [
	'Passwords/Common-Credentials/100k-most-used-passwords-NCSC.txt',
	'Passwords/Common-Credentials/xato-net-10-million-passwords-1000000.txt',
	'Passwords/Common-Credentials/Pwdb_top-1000000.txt'
];

const targets = [
	new URL('../src/lib/commonPasswords.txt', import.meta.url),
	new URL('../../api/internal/staffauth/commonpasswords.txt', import.meta.url)
];

const PRINTABLE_ASCII = /^[ -~]+$/;
const HEX_ENCODED = /^\$hex\[/;

async function fetchSource(path: string): Promise<string> {
	const url = `https://raw.githubusercontent.com/danielmiessler/SecLists/${SECLISTS_COMMIT}/${path}`;
	const response = await fetch(url);
	if (!response.ok) throw new Error(`${url} answered ${response.status}`);
	return response.text();
}

function isKept(entry: string): boolean {
	return (
		PRINTABLE_ASCII.test(entry) && !HEX_ENCODED.test(entry) && [...entry].length >= MIN_PASSWORD_LENGTH
	);
}

const entries = new Set<string>();
for (const path of SOURCES) {
	const text = await fetchSource(path);
	for (const line of text.split(/\r?\n/)) {
		const entry = line.toLowerCase();
		if (isKept(entry)) entries.add(entry);
	}
}

// Code-unit order, not locale order, so the file is the same on every machine.
function byCodeUnit(a: string, b: string): number {
	if (a < b) return -1;
	if (a > b) return 1;
	return 0;
}

const body = [...entries].toSorted(byCodeUnit).join('\n') + '\n';
for (const target of targets) await Bun.write(target, body);

console.log(`${entries.size} entries, ${body.length} bytes`);
