import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { globFiles } from './globFiles';
import { QUOTED, quotedStringsInSource, regionLines } from './quotedCopy';

/*
 * #2008's rule (`docs/design/brief.md`, Voice rule 9, and
 * `docs/design/govuk-alignment.md`, "Sign in, never log in"): a person
 * signs in and signs out on every screen. The app had said "Log in" for
 * the act and "Sign out" for its end; GOV.UK's style guide says "sign in"
 * rather than "log in" for entering one's details to reach a service.
 *
 * ## What it reads
 *
 * The same two shapes the other copy specs read, through `quotedCopy.ts`
 * with comments stripped first, since a comment is this repo's documented
 * voice and not a line a person reads:
 *
 * - every component and route `.svelte` file: a markup line whole, and a
 *   `<script>` line's quoted strings only;
 * - every plain `.ts` module under `src/lib` and `src/routes` that is not
 *   a spec, and every Go file under `api/` that is not a test: its quoted
 *   strings. A refusal the BFF writes ("This account already belongs to a
 *   Practice. Sign in instead.") reaches the screen as it was written, and
 *   `formErrors.ts` holds the app's own.
 *
 * It refuses the verb forms -- "log in", "log-in", "logging in", "logged
 * in", "log out" -- and not the token `login`: a route path (`/login`), a
 * mode value (`'login'`) and an identifier are not copy. "Login" as a noun
 * ("Delete your login") is a separate GOV.UK rule this does not decide.
 *
 * A line that has to keep the other form puts `voice:ignore` and the
 * reason on the same line, the marker `copy.secondPerson.usage.spec.ts`
 * reads.
 */

const LOG_IN = /\blog(?:ged|ging)?[ -](?:in|out)\b/i;
const MARKER_WITH_REASON = /voice:ignore\W*\w/;

const appRoot = fileURLToPath(new URL('../../', import.meta.url));
const repoRoot = fileURLToPath(new URL('../../../', import.meta.url));

const svelteFiles = globFiles('src/{lib/components,routes}/**/*.svelte', { cwd: appRoot });
const moduleFiles = globFiles('src/{lib,routes}/**/*.ts', { cwd: appRoot }).filter(
	(file) => !/\.(?:test|spec)\.ts$/.test(file) && !file.endsWith('.d.ts')
);
const goFiles = globFiles('api/**/*.go', { cwd: repoRoot }).filter((file) => !file.endsWith('_test.go'));

interface Offense {
	file: string;
	line: number;
	text: string;
}

function isMarked(source: string, line: number): boolean {
	return MARKER_WITH_REASON.test(source.split('\n')[line - 1]!);
}

/**
The rendered copy of a `.svelte` source that says "log in".
*/
function findInSvelte(file: string, source: string): Offense[] {
	return regionLines(source).flatMap(({ line, text, region }) => {
		if (region === 'style' || isMarked(source, line)) return [];
		const candidates =
			region === 'script'
				? text
						.matchAll(QUOTED)
						.map((match) => (match[1] ?? match[2] ?? match[3])!)
						.toArray()
				: [text];
		return candidates.filter((candidate) => LOG_IN.test(candidate)).map((candidate) => ({ file, line, text: candidate }));
	});
}

/**
The quoted strings of a `.ts` or `.go` source that say "log in".
*/
function findInStrings(file: string, source: string): Offense[] {
	return quotedStringsInSource(source)
		.filter(({ line, text }) => LOG_IN.test(text) && !isMarked(source, line))
		.map(({ line, text }) => ({ file, line, text }));
}

// Read and scanned once, at module scope, for the reason
// copy.pronoun.usage.spec.ts gives (#1211).
const offenses = [
	...svelteFiles.flatMap((file) => findInSvelte(file, readFileSync(`${appRoot}${file}`, 'utf8'))),
	...moduleFiles.flatMap((file) => findInStrings(file, readFileSync(`${appRoot}${file}`, 'utf8'))),
	...goFiles.flatMap((file) => findInStrings(`../${file}`, readFileSync(`${repoRoot}${file}`, 'utf8')))
];

describe('a person signs in and signs out, and never logs in (#2008)', () => {
	it('reads every screen, every module and the BFF', () => {
		// A glob that silently matched nothing would make the last assertion
		// pass while checking no copy at all.
		expect(svelteFiles).toContain('src/routes/(signed-out)/login/+page.svelte');
		expect(moduleFiles).toContain('src/lib/formErrors.ts');
		expect(goFiles).toContain('api/internal/staffauth/signup.go');
		expect(moduleFiles.some((file) => file.endsWith('.spec.ts'))).toBe(false);
		expect(goFiles.some((file) => file.endsWith('_test.go'))).toBe(false);
	});

	it('finds "log in" in markup, in a script string and in a module string, in each form', () => {
		const svelte = ['<script>', "const label = 'Continue to log in';", '</script>', '<h1>Log in</h1>', '<p>Logging in…</p>'].join('\n');
		expect(findInSvelte('a.svelte', svelte).map((offense) => offense.line)).toEqual([2, 4, 5]);
		expect(findInStrings('a.ts', "const message = 'Log in instead.';\nconst b = 'the log-in screen';")).toHaveLength(2);
		expect(findInStrings('a.go', 'const Msg = "Log out of every device"')).toHaveLength(1);
	});

	it('leaves "sign in", a path, a mode value, a comment, a style and a line that carries the marker', () => {
		const svelte = [
			'<script>',
			'// sent to log in again',
			"const href = resolve('/(signed-out)/login');",
			"let mode = $state<'signup' | 'login'>('signup');",
			'</script>',
			'<!-- both doors said "Log in" -->',
			'<h1>Sign in</h1>',
			'<p>Log in</p> <!-- voice:ignore: a quotation -->',
			'<style>',
			'.log-in { color: red; }',
			'</style>'
		].join('\n');
		expect(findInSvelte('a.svelte', svelte)).toEqual([]);
		expect(findInStrings('a.ts', "// tell her to log in\nconst path = '/portal/login';")).toEqual([]);
	});

	it('finds no "log in" anywhere a person reads', () => {
		expect(offenses.map((offense) => `${offense.file}:${offense.line} "${offense.text}" -- say "sign in"`)).toEqual([]);
	});
});
