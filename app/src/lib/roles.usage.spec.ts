import { globSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * #262's third AC as a gate, in the mold of `clientRegister.usage.spec.ts`
 * (#212) and `formErrors.usage.spec.ts` (#467): once every surface spends
 * `roles.ts`, nothing may quietly grow a second way to say `owner` or
 * `contractor` to a person.
 *
 * Two checks, both lexical, both deliberately narrow:
 *
 * 1. **No raw `roles` join.** `member.roles.join(', ')` is exactly the
 *    defect this issue names -- stored values printed verbatim -- and it
 *    has no other legitimate use in a source file, so the call shape alone
 *    is the offence.
 * 2. **No second label map.** A file that quotes two or more of `Owner`,
 *    `Admin`, `Doula`, or both of `Employee` and `Contractor`, is naming
 *    the enum's display words; that is `roles.ts`'s job. One such literal
 *    on its own is not an offence, because a column heading ("Doula") and
 *    a table fixture ("Employee") each legitimately hold one and neither
 *    is a map.
 *
 * Spec files are exempt -- asserting the words a screen shows is the point
 * of a spec -- and so is `roles.ts` itself, which is the one map.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const ROLE_WORDS = ['Owner', 'Admin', 'Doula'];
const EMPLOYMENT_WORDS = ['Employee', 'Contractor'];
const QUOTED = /'([^'\n]*)'|"([^"\n]*)"/g;
const RAW_JOIN = /\broles\.join\(/;

interface Offence {
	file: string;
	found: string;
}

/**
 * Every quoted string literal in a source file, however quoted.
 */
function quotedLiterals(source: string): string[] {
	return source
		.matchAll(QUOTED)
		.map((match) => match[1] ?? match[2] ?? '')
		.toArray();
}

function findOffences(file: string, source: string): Offence[] {
	const offences: Offence[] = [];
	if (RAW_JOIN.test(source)) offences.push({ file, found: 'a raw roles join' });

	const literals = new Set(quotedLiterals(source));
	const roleWords = ROLE_WORDS.filter((word) => literals.has(word));
	if (roleWords.length > 1) offences.push({ file, found: `a second role label map (${roleWords.join(', ')})` });
	if (EMPLOYMENT_WORDS.every((word) => literals.has(word)))
		offences.push({ file, found: 'a second employment-type label map' });

	return offences;
}

const sourceFiles = globSync('src/**/*.{svelte,ts}', { cwd: appRoot }).filter(
	(file) => !file.includes('.spec.') && file !== 'src/lib/roles.ts'
);

describe('practice_role and employment_type are labeled in one place', () => {
	it('reads the whole app source tree', () => {
		// A glob that silently matched nothing would make the assertion
		// below pass while checking no source at all.
		expect(sourceFiles.length).toBeGreaterThan(100);
	});

	it('finds no raw join and no second label map', () => {
		const offences = sourceFiles.flatMap((file) =>
			findOffences(file, readFileSync(path.join(appRoot, file), 'utf8'))
		);

		expect(offences.map((offence) => `${offence.file}: ${offence.found}`)).toEqual([]);
	});
});

describe('findOffences', () => {
	it('flags a raw roles join', () => {
		expect(findOffences('x.svelte', 'member.roles.join(", ")')).toEqual([
			{ file: 'x.svelte', found: 'a raw roles join' }
		]);
	});

	it('flags a file that names two role words itself', () => {
		expect(findOffences('x.ts', "const m = { owner: 'Owner', doula: 'Doula' };")).toEqual([
			{ file: 'x.ts', found: 'a second role label map (Owner, Doula)' }
		]);
	});

	it('flags a file that names both employment-type words itself', () => {
		expect(findOffences('x.ts', 'const m = ["Employee", "Contractor"];')).toEqual([
			{ file: 'x.ts', found: 'a second employment-type label map' }
		]);
	});

	it('allows a single role word used as a heading', () => {
		expect(findOffences('x.svelte', "<legend>{'Doula'}</legend>")).toEqual([]);
	});

	it('allows a single employment word used as fixture data', () => {
		expect(findOffences('x.svelte', "const row = { employment: 'Employee' };")).toEqual([]);
	});
});
