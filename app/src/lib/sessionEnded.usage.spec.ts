import { globSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { quotedStrings, quotedStringsInSource } from './quotedCopy.js';

/*
 * #1131's first AC as a gate, in the mold of `roles.usage.spec.ts` (#262)
 * and `formErrors.usage.spec.ts` (#467): once `sessionEnded.ts` owns the
 * flag that says "you are looking at this login form because your session
 * ended", nothing may quietly grow a second spelling of it.
 *
 * The convention was spelled by hand in fourteen places before this --
 * twelve writers building `?sessionEnded=true` into a redirect, two
 * readers matching it back -- and the writers and readers agreed only by
 * everyone having typed the same thing. Renaming the param, or spelling
 * it `sessionended` in one new redirect, was a silent miss rather than a
 * type error, and the notice would simply not render. There is no type
 * seam that can refuse it: a redirect target is a `string`, and
 * `redirect(303, ...)` takes whatever a caller hands it. So the seam is
 * lexical, and it runs in the unit suite, which `scripts/hooks/pre-commit`
 * runs in full -- a new bypass fails a commit rather than reaching CI.
 *
 * The walk is `quotedCopy.ts`'s, not this file's, for the reason that
 * module gives: every gate here asks "what quoted text does this file
 * carry, once its comments are gone", and differs only in what it looks
 * for. Taking it from there is also what keeps this gate off prose --
 * comments are stripped before a single literal is read, so a doc comment
 * may go on quoting `sessionEnded=true` verbatim. Only a file saying the
 * wire word to itself is an offense; an identifier built from the same
 * words is not one either.
 *
 * Spec and fixture files are exempt, and that is not a hole. Asserting
 * the exact address is the point of a spec: `api.spec.ts`, the eight
 * route-load specs and `account.svelte.spec.ts` are what pin
 * `/login?sessionEnded=true` and `/portal/login?sessionEnded=true`
 * byte-for-byte (#1131's fourth AC). If the module ever changes an
 * address, those pins are what go red -- they are the counter-check to
 * the owner, not a second copy of it.
 *
 * The two `page.fixture.ts` route variants are exempt on their own
 * grounds: a fixture describes the address a person lands on as the
 * unresolved route id (`/(signed-out)/login?...`), which is not what any
 * builder here produces and could not be, since the builders resolve.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const OWNER = 'src/lib/sessionEnded.ts';

/*
 * Importing the owner is the opposite of an offense, and its module id is
 * a quoted literal that spells the flag. Dropped before the test, so
 * `'#lib/sessionEnded.js'` reads as what it is.
 */
const OWNER_MODULE_ID = /sessionEnded\.js/g;
const FLAG = /sessionEnded/;

interface Offense {
	file: string;
	line: number;
	found: string;
}

function offensesIn(file: string, literals: { line: number; text: string }[]): Offense[] {
	return literals
		.filter((literal) => FLAG.test(literal.text.replaceAll(OWNER_MODULE_ID, '')))
		.map((literal) => ({ file, line: literal.line, found: literal.text }));
}

function findOffenses(file: string, source: string): Offense[] {
	return offensesIn(file, quotedStringsInSource(source));
}

const sourceFiles = globSync('src/**/*.{svelte,ts}', { cwd: appRoot }).filter(
	(file) => !file.includes('.spec.') && !file.includes('.fixture.') && file !== OWNER
);

describe('the session-ended flag is spelled in one place', () => {
	it('reads the whole app source tree', () => {
		// A glob that silently matched nothing would make the assertion
		// below pass while checking no source at all.
		expect(sourceFiles.length).toBeGreaterThan(100);
	});

	it('finds no file but the owner naming the flag', () => {
		const offenses = sourceFiles.flatMap((file) => offensesIn(file, quotedStrings(file, appRoot)));

		expect(offenses.map((offense) => `${offense.file}:${offense.line}: ${offense.found}`)).toEqual([]);
	});
});

describe('findOffenses', () => {
	it('flags a redirect that builds the query string itself', () => {
		expect(findOffenses('x.ts', 'redirect(303, `${resolve(r)}?sessionEnded=true`);')).toEqual([
			{ file: 'x.ts', line: 1, found: '${resolve(r)}?sessionEnded=true' }
		]);
	});

	it('flags a reader that names the param itself', () => {
		expect(findOffenses('x.svelte', "url.searchParams.get('sessionEnded') === 'true'")).toEqual([
			{ file: 'x.svelte', line: 1, found: 'sessionEnded' }
		]);
	});

	it('allows a doc comment quoting the flag verbatim', () => {
		expect(findOffenses('x.ts', '/*\n * carries `sessionEnded=true` to the login screen\n */')).toEqual([]);
	});

	it('allows importing the owner', () => {
		expect(findOffenses('x.ts', "import { didSessionEnd } from '#lib/sessionEnded.js';")).toEqual([]);
	});

	it('allows an identifier built from the same words', () => {
		const source = 'const hasSessionEnded = $derived(didSessionEnd(page.url));';

		expect(findOffenses('x.svelte', source)).toEqual([]);
	});
});
