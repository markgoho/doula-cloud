import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { globFiles } from './globFiles';
import { QUOTED, quotedStringsInSource, regionLines } from './quotedCopy';

/*
 * #2247 (`docs/design/brief.md`, Voice rule 8): a domain noun takes a
 * capital where it names a thing in the interface. The Contracts and
 * Invoices intros said "this practice" and "the engagement it belongs to"
 * while every row, heading and notice beside them said Practice and
 * Engagement, as GLOSSARY.md spells them.
 *
 * Rule 8 also keeps a noun lowercase in a running sentence about the
 * world, "For doulas and practice owners", so a bare lowercase noun is
 * not an offense and this cannot find every case. What it finds is the
 * shape that is never about the world: a determiner pointing at one of
 * the product's own records -- "this practice", "the engagement", "an
 * invoice", "every contract". Those are the records a screen shows, and
 * GLOSSARY.md capitalizes each. The roles (Doula, Owner, Admin) are left
 * out: rule 2 writes them lowercase when a screen does not yet know the
 * person ("a doula, an owner").
 *
 * ## What it reads
 *
 * The shapes `copy.signIn.usage.spec.ts` reads, through `quotedCopy.ts`
 * with comments stripped first: every component and route `.svelte` file
 * (a markup line whole, a `<script>` line's quoted strings only), and the
 * quoted strings of every plain `.ts` module under `src/lib` and
 * `src/routes` that is not a spec. A line that has to keep the lowercase
 * form puts `voice:ignore` and the reason on the same line.
 *
 * Two kinds of file are left out. A route fixture holds what a person
 * typed (a Client's note, a Message), which is not ours to capitalize.
 * And the screens a Client reads (`CLIENT_READ` in
 * `copy.secondPerson.usage.spec.ts`, and `yourCare.ts`, which only the
 * portal hub reads) follow the Client register, where GLOSSARY.md makes
 * "visits" her own word and whether Contract and Invoice take a capital is
 * open on #2327.
 */

const DOMAIN_NOUN =
	/\b(?:this|that|these|those|the|each|every|an?|your|its|no|any|one)\s+(?:practice|engagement|invoice|contract|client|visit|credit)s?\b/i;
const LOWERCASE_NOUN = /\s(?:practice|engagement|invoice|contract|client|visit|credit)s?\b/;
const MARKER_WITH_REASON = /voice:ignore\W*\w/;

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

// #2327 decides these. A narrower copy of `CLIENT_READ`, as a test: a
// spec file may not export it.
const CLIENT_READ =
	/^src\/(?:routes\/portal\/|lib\/components\/.*\/Portal[^/]*\.svelte$|lib\/(?:clientRegister|clientInvoice|clientPayment|portalLanding|portalVisits|yourCare)\.ts$)/;

const svelteFiles = globFiles('src/{lib/components,routes}/**/*.svelte', { cwd: appRoot }).filter(
	(file) => !CLIENT_READ.test(file)
);
const moduleFiles = globFiles('src/{lib,routes}/**/*.ts', { cwd: appRoot }).filter(
	(file) =>
		!/\.(?:test|spec)\.ts$/.test(file) && !file.endsWith('.d.ts') && !file.endsWith('.fixture.ts') && !CLIENT_READ.test(file)
);

// A Svelte block's own words -- `{#each practices as practice}` -- are
// code, not copy.
const SVELTE_BLOCK = /\{[#:/@][^}]*\}/g;

interface Offense {
	file: string;
	line: number;
	text: string;
}

/**
The phrases in one line of copy that point at a record in lowercase.
*/
function lowercasePhrases(text: string): string[] {
	const pattern = new RegExp(DOMAIN_NOUN.source, 'gi');
	return text
		.matchAll(pattern)
		.map((match) => match[0])
		.filter((phrase) => LOWERCASE_NOUN.test(phrase))
		.toArray();
}

function isMarked(source: string, line: number): boolean {
	return MARKER_WITH_REASON.test(source.split('\n')[line - 1]!);
}

function findInSvelte(file: string, source: string): Offense[] {
	return regionLines(source).flatMap(({ line, text, region }) => {
		if (region === 'style' || isMarked(source, line)) return [];
		const candidates =
			region === 'script'
				? text
						.matchAll(QUOTED)
						.map((match) => (match[1] ?? match[2] ?? match[3])!)
						.toArray()
				: [text.replaceAll(SVELTE_BLOCK, '')];
		return candidates.flatMap((candidate) => lowercasePhrases(candidate).map((phrase) => ({ file, line, text: phrase })));
	});
}

function findInStrings(file: string, source: string): Offense[] {
	return quotedStringsInSource(source)
		.filter(({ line }) => !isMarked(source, line))
		.flatMap(({ line, text }) => lowercasePhrases(text).map((phrase) => ({ file, line, text: phrase })));
}

// Read and scanned once, at module scope, for the reason
// copy.pronoun.usage.spec.ts gives (#1211).
const offenses = [
	...svelteFiles.flatMap((file) => findInSvelte(file, readFileSync(`${appRoot}${file}`, 'utf8'))),
	...moduleFiles.flatMap((file) => findInStrings(file, readFileSync(`${appRoot}${file}`, 'utf8')))
];

describe("a domain noun that points at one of the product's records takes its capital (#2247)", () => {
	it('reads every screen and every module', () => {
		// A glob that silently matched nothing would make the last assertion
		// pass while checking no copy at all.
		expect(svelteFiles).toContain('src/routes/practices/[practiceId]/contracts/ContractsScreen.svelte');
		expect(moduleFiles).toContain('src/lib/invoice.ts');
		expect(moduleFiles.some((file) => file.endsWith('.spec.ts'))).toBe(false);
		expect(svelteFiles.some((file) => file.startsWith('src/routes/portal/'))).toBe(false);
		expect(moduleFiles).not.toContain('src/lib/yourCare.ts');
	});

	it('finds a lowercase record in markup, in a script string and in a module string', () => {
		const svelte = [
			'<script>',
			"const intro = 'Every invoice this practice has billed';",
			'</script>',
			'<p>Open one to reach the engagement it belongs to.</p>'
		].join('\n');
		expect(findInSvelte('a.svelte', svelte).map((offense) => offense.text)).toEqual([
			'Every invoice',
			'this practice',
			'the engagement'
		]);
		expect(findInStrings('a.ts', "const message = 'Send a contract first.';")).toHaveLength(1);
	});

	it('leaves a capital, a noun about the world, a comment, an identifier and a marked line', () => {
		const svelte = [
			'<script>',
			'// the engagement this belongs to',
			'const engagement = the.engagement;',
			"const intro = 'Every Invoice this Practice has billed';",
			'</script>',
			'<!-- this practice -->',
			'{#each client.openEngagements as line (line.id)}',
			'<p>For doulas and practice owners.</p>',
			'<p>Ask a doula at the Practice.</p>',
			'<p>the practice <!-- voice:ignore: a quotation --></p>'
		].join('\n');
		expect(findInSvelte('a.svelte', svelte)).toEqual([]);
	});

	it('finds no lowercase record in any copy', () => {
		expect(offenses).toEqual([]);
	});
});
