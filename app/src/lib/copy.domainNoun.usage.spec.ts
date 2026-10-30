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
 * A route fixture is left out: it holds what a person typed (a Client's
 * note, a Message), which is not ours to capitalize.
 *
 * ## The screens a Client reads, the other way round (#2327)
 *
 * The screens a Client reads (`CLIENT_READ` below) follow the Client
 * register, and there the founder decided on 2026-10-30 that Contract, Invoice and Birth Plan are written
 * lowercase: "your contract", "your invoices", "your birth plan". So those
 * files are left out of the check above and read for the opposite
 * offense: a capitalized Contract or Invoice after another word in the
 * same sentence, or a Birth Plan whose "Plan" is capitalized. The first
 * word of a sentence, a heading or a label ("Invoices", "Contract signed")
 * keeps its sentence-case capital, so it is not an offense.
 */

const DOMAIN_NOUN =
	/\b(?:this|that|these|those|the|each|every|an?|your|its|no|any|one)\s+(?:practice|engagement|invoice|contract|client|visit|credit)s?\b/i;
const LOWERCASE_NOUN = /\s(?:practice|engagement|invoice|contract|client|visit|credit)s?\b/;
const MARKER_WITH_REASON = /voice:ignore\W*\w/;

// A word, then a capitalized Contract or Invoice: not the first word of
// its sentence. And "Birth Plan" wherever it stands, since sentence case
// never capitalizes a second word.
const CLIENT_CAPITAL = /(?<=\b\w[\w']*\s+)(?:Contract|Invoice)s?\b|\bBirth Plans?\b/g;

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

// The Client register's files (#2327): a copy of `CLIENT_READ`, as a spec
// file may not export it, plus what only a Client reads outside its globs:
// `yourCare.ts`, `portalDoor.ts`, and `SignContract`, which only the
// portal's contract page renders.
const CLIENT_READ =
	/^src\/(?:routes\/portal\/|lib\/components\/(?:.*\/Portal[^/]*|organisms\/SignContract)\.svelte$|lib\/(?:clientRegister|clientInvoice|clientPayment|portalDoor|portalLanding|portalVisits|yourCare)\.ts$)/;

const allSvelteFiles = globFiles('src/{lib/components,routes}/**/*.svelte', { cwd: appRoot });
const allModuleFiles = globFiles('src/{lib,routes}/**/*.ts', { cwd: appRoot }).filter(
	(file) => !/\.(?:test|spec)\.ts$/.test(file) && !file.endsWith('.d.ts') && !file.endsWith('.fixture.ts')
);
const svelteFiles = allSvelteFiles.filter((file) => !CLIENT_READ.test(file));
const moduleFiles = allModuleFiles.filter((file) => !CLIENT_READ.test(file));
const clientSvelteFiles = allSvelteFiles.filter((file) => CLIENT_READ.test(file));
const clientModuleFiles = allModuleFiles.filter((file) => CLIENT_READ.test(file));

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

/**
The phrases in one line of a Client's copy that capitalize a noun the
Client register writes lowercase (#2327).
*/
function capitalizedPhrases(text: string): string[] {
	return text
		.matchAll(CLIENT_CAPITAL)
		.map((match) => match[0])
		.toArray();
}

type Finder = (text: string) => string[];

function isMarked(source: string, line: number): boolean {
	return MARKER_WITH_REASON.test(source.split('\n')[line - 1]!);
}

function findInSvelte(file: string, source: string, find: Finder = lowercasePhrases): Offense[] {
	return regionLines(source).flatMap(({ line, text, region }) => {
		if (region === 'style' || isMarked(source, line)) return [];
		const candidates =
			region === 'script'
				? text
						.matchAll(QUOTED)
						.map((match) => (match[1] ?? match[2] ?? match[3])!)
						.toArray()
				: [text.replaceAll(SVELTE_BLOCK, '')];
		return candidates.flatMap((candidate) => find(candidate).map((phrase) => ({ file, line, text: phrase })));
	});
}

function findInStrings(file: string, source: string, find: Finder = lowercasePhrases): Offense[] {
	return quotedStringsInSource(source)
		.filter(({ line }) => !isMarked(source, line))
		.flatMap(({ line, text }) => find(text).map((phrase) => ({ file, line, text: phrase })));
}

// Read and scanned once, at module scope, for the reason
// copy.pronoun.usage.spec.ts gives (#1211).
const offenses = [
	...svelteFiles.flatMap((file) => findInSvelte(file, readFileSync(`${appRoot}${file}`, 'utf8'))),
	...moduleFiles.flatMap((file) => findInStrings(file, readFileSync(`${appRoot}${file}`, 'utf8')))
];
const clientOffenses = [
	...clientSvelteFiles.flatMap((file) => findInSvelte(file, readFileSync(`${appRoot}${file}`, 'utf8'), capitalizedPhrases)),
	...clientModuleFiles.flatMap((file) => findInStrings(file, readFileSync(`${appRoot}${file}`, 'utf8'), capitalizedPhrases))
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

describe('a Client reads Contract, Invoice and Birth Plan in lowercase (#2327)', () => {
	it('reads every screen a Client reads', () => {
		expect(clientSvelteFiles).toContain('src/routes/portal/(authenticated)/engagements/[engagementId]/+page.svelte');
		expect(clientModuleFiles).toContain('src/lib/yourCare.ts');
		expect(clientModuleFiles).toContain('src/lib/portalDoor.ts');
		expect(clientSvelteFiles).toContain('src/lib/components/organisms/SignContract.svelte');
		expect(clientModuleFiles).toContain('src/lib/clientRegister.ts');
	});

	it('finds a capital after another word, and a capitalized Birth Plan anywhere', () => {
		const svelte = [
			'<script>',
			"const loading = 'Loading your Invoices';",
			'</script>',
			'<p>Read and sign your Contract.</p>',
			'<h2>Birth Plan</h2>'
		].join('\n');
		expect(findInSvelte('a.svelte', svelte, capitalizedPhrases).map((offense) => offense.text)).toEqual([
			'Invoices',
			'Contract',
			'Birth Plan'
		]);
		expect(findInStrings('a.ts', "const entry = 'Your Contract was signed.';", capitalizedPhrases)).toHaveLength(1);
	});

	it('leaves the first word of a sentence, a heading or a label, lowercase, and a marked line', () => {
		const svelte = [
			'<script>',
			"const heading = 'Invoices';",
			"const status = 'Signed. Contract copy below.';",
			'</script>',
			'<h2>Your contract</h2>',
			'<p>Read your birth plan and your invoices.</p>',
			'<p>Contract signed</p>',
			'<p>the Contract <!-- voice:ignore: a quotation --></p>'
		].join('\n');
		expect(findInSvelte('a.svelte', svelte, capitalizedPhrases)).toEqual([]);
	});

	it('finds no capitalized Contract, Invoice or Birth Plan in any copy a Client reads', () => {
		expect(clientOffenses).toEqual([]);
	});
});
