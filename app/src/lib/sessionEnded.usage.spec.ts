import { globSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

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
 * The check is deliberately narrow: a *quoted or template literal* that
 * names the flag. Prose that mentions it in a comment is not an offense,
 * and neither is an identifier like `hasSessionEnded` -- only a file
 * saying the wire word to itself is.
 *
 * Spec and fixture files are exempt, and that is not a hole. Asserting
 * the exact address is the point of those files: `api.spec.ts`, the eight
 * route-load specs, `account.svelte.spec.ts` and the two `page.fixture.ts`
 * route variants are what pin `/login?sessionEnded=true` and
 * `/portal/login?sessionEnded=true` byte-for-byte (#1131's fourth AC). If
 * the module ever changes the address, those pins are what go red. They
 * are the counter-check to the owner, not a second copy of it.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const OWNER = 'src/lib/sessionEnded.ts';

/*
 * Single-quoted, double-quoted, and backticked spans alike -- the writers
 * this replaces all built the address inside a template literal.
 */
const LITERAL = /'([^'\n]*)'|"([^"\n]*)"|`([^`]*)`/g;
const FLAG = /sessionEnded/;

interface Offense {
	file: string;
	found: string;
}

/**
 * Every quoted or templated string span in a source file.
 */
function literals(source: string): string[] {
	return source
		.matchAll(LITERAL)
		.map((match) => match[1] ?? match[2] ?? match[3] ?? '')
		.toArray();
}

function findOffenses(file: string, source: string): Offense[] {
	return literals(source)
		.filter((literal) => FLAG.test(literal))
		.map((literal) => ({ file, found: literal }));
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
		const offenses = sourceFiles.flatMap((file) =>
			findOffenses(file, readFileSync(path.join(appRoot, file), 'utf8'))
		);

		expect(offenses.map((offense) => `${offense.file}: ${offense.found}`)).toEqual([]);
	});
});

describe('findOffenses', () => {
	it('flags a redirect that builds the query string itself', () => {
		expect(findOffenses('x.ts', 'redirect(303, `${resolve(r)}?sessionEnded=true`);')).toEqual([
			{ file: 'x.ts', found: '${resolve(r)}?sessionEnded=true' }
		]);
	});

	it('flags a reader that names the param itself', () => {
		expect(findOffenses('x.svelte', "url.searchParams.get('sessionEnded') === 'true'")).toEqual([
			{ file: 'x.svelte', found: 'sessionEnded' }
		]);
	});

	it('allows a comment that mentions the flag in prose', () => {
		expect(findOffenses('x.ts', '// carries sessionEnded=true to the login screen')).toEqual([]);
	});

	it('allows an identifier built from the same words', () => {
		expect(findOffenses('x.svelte', 'const hasSessionEnded = $derived(sessionEndedFrom(page.url));')).toEqual(
			[]
		);
	});
});
