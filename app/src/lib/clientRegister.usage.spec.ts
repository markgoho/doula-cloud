import { globSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { QUOTED, regionLines, regionLinesInFile, type Region } from './quotedCopy';

/*
 * #212 / #834's second added AC: a usage gate over `routes/portal/**` that
 * fails on a team word the Client register (ADR-0005, CONTEXT.md)
 * translates, the same mechanism `copy.pronoun.usage.spec.ts` (#463) and
 * `formErrors.usage.spec.ts` (#467) already use on the same `quotedCopy.ts`
 * walk.
 *
 * ## What it bans, and how
 *
 * "Engagement" -- the noun the register replaces with "care"/"my care" --
 * is checked the way the pronoun gate checks a pronoun: the whole line of
 * a markup region (real rendered text has no reason to quote itself), and
 * only a quoted script-string (so a `type Engagement` import is not an
 * offense).
 *
 * The raw `engagement_status`/`contract_status` enum values
 * (`intake`/`active`/`completed`/`draft`/`sent`/`signed`/`voided`) are
 * checked differently, and deliberately not the same way as "Engagement":
 * several of them are ordinary English words this codebase already uses
 * correctly ("we have sent a sign-in link", "signed out", the
 * `(signed-out)` route-group segment folded into a handful of `resolve()`
 * paths), so a substring-in-a-sentence check would fail the suite on
 * copy that is not the defect. A quoted literal is only an offense when
 * its *entire* content, once extracted, equals one of the enum values --
 * "sent" alone, not "...have sent...", and not "/portal/(signed-out)/..."
 * -- and it is still not an offense when the literal sits immediately
 * after a comparison operator (`===`, `!==`, `==`, `!=`): that shape is a
 * `{#if contract.status === 'sent'}` branch, Svelte control flow rather
 * than displayed copy, and every real occurrence in the covered glob
 * today is exactly that.
 *
 * ## What it deliberately does not catch
 *
 * A raw status value reaching a Client today does so through a bound
 * variable (`{d.status}`, `status={contract.status}`), not a quoted
 * literal -- no lexical scan catches that, the same limit
 * `formErrors.usage.spec.ts` accepts for server prose. This gate is the
 * regression fence once `clientRegister.ts` fixes the bound-variable
 * cases, not a detector for that class of bug.
 */

const NOUNS = ['Engagement'];
const STATUS_VALUES = new Set(['intake', 'active', 'completed', 'draft', 'sent', 'signed', 'voided']);
const COMPARISON_BEFORE = /[=!]==?$/;

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const sourceFiles = globSync('src/routes/portal/**/*.svelte', { cwd: appRoot });

interface Offense {
	file: string;
	line: number;
	found: string;
	text: string;
}

function nounOffensesInLine(file: string, line: number, text: string, region: Region): Offense[] {
	const found: Offense[] = [];
	const candidates =
		region === 'script'
			? text
					.matchAll(QUOTED)
					.map((match) => (match[1] ?? match[2] ?? match[3])!)
					.toArray()
			: [text];
	for (const candidate of candidates) {
		for (const noun of NOUNS) {
			if (new RegExp(String.raw`\b${noun}\b`).test(candidate)) {
				found.push({ file, line, found: noun, text: candidate });
			}
		}
	}
	return found;
}

function statusOffensesInLine(file: string, line: number, text: string): Offense[] {
	const found: Offense[] = [];
	for (const match of text.matchAll(QUOTED)) {
		const literal = (match[1] ?? match[2] ?? match[3])!;
		if (!STATUS_VALUES.has(literal)) continue;
		if (COMPARISON_BEFORE.test(text.slice(0, match.index).trimEnd())) continue;
		found.push({ file, line, found: literal, text: literal });
	}
	return found;
}

function findOffensesInLines(file: string, lines: ReturnType<typeof regionLines>): Offense[] {
	const found: Offense[] = [];
	for (const { line, text, region } of lines) {
		if (region === 'style') continue;
		found.push(...nounOffensesInLine(file, line, text, region), ...statusOffensesInLine(file, line, text));
	}
	return found;
}

function findOffenses(file: string): Offense[] {
	return findOffensesInLines(file, regionLinesInFile(file, appRoot));
}

describe('the Client portal speaks the register, not the team\'s words', () => {
	it('reads every portal route', () => {
		// A glob that silently matched nothing would make every assertion
		// below pass while checking no copy at all.
		expect(sourceFiles.length).toBeGreaterThan(10);
	});

	it('says no team word or raw status value the register translates', () => {
		const offenses = sourceFiles.flatMap((file) => findOffenses(file));

		expect(offenses.map((offense) => `${offense.file}:${offense.line} "${offense.text}"`)).toEqual([]);
	});
});

describe('findOffensesInLines', () => {
	it('flags "Engagement" in rendered markup text', () => {
		const offenses = findOffensesInLines('fixture.svelte', regionLines('<h2>Choose an Engagement</h2>'));

		expect(offenses).toEqual([
			{ file: 'fixture.svelte', line: 1, found: 'Engagement', text: '<h2>Choose an Engagement</h2>' }
		]);
	});

	it('ignores "Engagement" inside an HTML comment', () => {
		const offenses = findOffensesInLines('fixture.svelte', regionLines('<!-- her Engagement --> <p>ok</p>'));

		expect(offenses).toEqual([]);
	});

	it('ignores a bare `type Engagement` import in <script>', () => {
		const source = ['<script>', "import type { Engagement } from './x';", '</script>'].join('\n');

		expect(findOffensesInLines('fixture.svelte', regionLines(source))).toEqual([]);
	});

	it('flags a raw status value quoted alone as display text', () => {
		const offenses = findOffensesInLines('fixture.svelte', regionLines('<p>{\'voided\'}</p>'));

		expect(offenses).toEqual([{ file: 'fixture.svelte', line: 1, found: 'voided', text: 'voided' }]);
	});

	it('flags a raw status value used as a literal prop value', () => {
		const offenses = findOffensesInLines('fixture.svelte', regionLines('<Badge label="sent" />'));

		expect(offenses).toEqual([{ file: 'fixture.svelte', line: 1, found: 'sent', text: 'sent' }]);
	});

	it('ignores a status value compared in a Svelte conditional', () => {
		const offenses = findOffensesInLines(
			'fixture.svelte',
			regionLines("{#if contract.status === 'sent'}\n<p>ok</p>\n{/if}")
		);

		expect(offenses).toEqual([]);
	});

	it('ignores a status value that is only a substring of ordinary prose', () => {
		const offenses = findOffensesInLines(
			'fixture.svelte',
			regionLines('<p>If that address is on our records, we have sent a sign-in link.</p>')
		);

		expect(offenses).toEqual([]);
	});

	it('ignores "signed" inside an unrelated route path', () => {
		const source = ["<script>", "goto('/portal/(signed-out)/login');", '</script>'].join('\n');

		expect(findOffensesInLines('fixture.svelte', regionLines(source))).toEqual([]);
	});

	it('ignores "signed" inside ordinary prose about sessions, not Contracts', () => {
		const offenses = findOffensesInLines(
			'fixture.svelte',
			regionLines('<p>You are signed out on every device immediately, including this one.</p>')
		);

		expect(offenses).toEqual([]);
	});

	it('skips a <style> line entirely', () => {
		const source = ['<style>', '/* content: "voided"; */', 'p { color: red; }', '</style>'].join('\n');

		expect(findOffensesInLines('fixture.svelte', regionLines(source))).toEqual([]);
	});

	it('reads a real file from disk the same way', () => {
		// Exercises `regionLinesInFile`'s disk-reading wrapper directly, which
		// the glob-driven test above already covers incidentally -- this
		// asserts it in isolation with a route the fix landed clean.
		const offenses = findOffenses('src/routes/portal/(signed-out)/login/+page.svelte');

		expect(offenses).toEqual([]);
	});
});
