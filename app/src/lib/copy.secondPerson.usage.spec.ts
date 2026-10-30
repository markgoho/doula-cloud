import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { CLIENT_READ } from './clientRead';
import { globFiles } from './globFiles';
import { QUOTED, quotedStringsInSource, regionLines } from './quotedCopy';

/*
 * The mechanical half of rule 4 of the Voice section of
 * `docs/design/brief.md` (#1641, #1642): product copy speaks to its reader as
 * "you" and "your". An impersonal or passive form that hides the reader --
 * "what is owed", "the Client's invoices" on a screen the Client reads -- is
 * the failure that rule names. `copy.pronoun.usage.spec.ts` catches the
 * third-person pronoun; this catches the forms that dodge a pronoun by
 * naming the reader as a noun or by leaving the reader out.
 *
 * ## What it reads
 *
 * Markup lines and quoted script strings, comments already stripped, the
 * same walk as `copy.pronoun.usage.spec.ts` (`quotedCopy.ts`). A `.ts`
 * module has no markup, so it is read through its quoted strings alone.
 *
 * ## Who the reader is
 *
 * Whether a sentence is third person depends on who reads it, and a file's
 * path is the only reader signal a regex can use. Each rule therefore names
 * the files whose reader it knows:
 *
 * - **Client-read copy**: the files `clientRead.ts` lists, the one list
 *   this spec shares with `copy.domainNoun.usage.spec.ts` (#2341). The
 *   reader is the Client, so "the Client's" and "owed" are about the reader.
 * - **The signed-in person's own screen**: `routes/(person)/account`. The
 *   reader is a Staff member and the subject is the same Staff member.
 * - **Every screen and module**, for one form whose reader does not matter:
 *   "Staff" followed by a verb, the Practice's own people as a third-person
 *   actor (#2221). A Staff reader is "you", and a Client never meets the word.
 *
 * ## The override
 *
 * A line that has to keep an offending form puts `voice:ignore` and the
 * reason on the same line (`<!-- voice:ignore: ... -->` in markup, a trailing
 * comment in a module), the same shape as `spelling:ignore`. A marker with
 * no reason after it does not count.
 *
 * ## What it deliberately does not catch
 *
 * 1. **A shared component's own copy.** Its reader depends on where it is
 *    rendered, and `lib/components/**` other than `Portal*` is rendered on
 *    both sides, so no path settles it. A reviewer reads for it.
 * 2. **Every impersonal form outside the rules below.** A machine cannot
 *    judge whether a sentence sounds natural. The Voice section and code
 *    review cover the remainder.
 * 3. **Third person on a Staff screen about somebody else.** "The Client's
 *    birth plan" on a Staff screen is correct: the reader is Staff.
 * 4. **A `.ts` module outside the Client register modules `clientRead.ts` names**,
 *    except for the Staff-actor form, which every module is read for.
 */

const MARKER = 'voice:ignore';

// A marker only counts with a reason after it.
const MARKER_WITH_REASON = new RegExp(String.raw`${MARKER}\W*\w`);

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

interface Rule {
	readonly id: string;
	readonly pattern: RegExp;
	// Globs, relative to `app/`, of the files whose reader this rule knows.
	readonly globs: readonly string[];
	// The second-person form that replaces it, shown in the failure message.
	readonly instead: string;
}

const OWED: Rule = {
	id: 'impersonal "owed"',
	pattern: /\bowed\b/,
	globs: CLIENT_READ,
	instead: 'say "you owe" ("What you still owe", "You no longer owe this")'
};

const CLIENT_NOUN: Rule = {
	id: 'the Client named as a noun on a screen the Client reads',
	pattern: /\bClients?\b/,
	globs: CLIENT_READ,
	instead: 'say "you" and "your" ("your contract", not "the Client\'s contract")'
};

const STAFF_NOUN: Rule = {
	id: 'a Staff member named as a noun on their own account screen',
	pattern: /\bStaff members?\b/,
	globs: ['src/routes/(person)/account/**/*.svelte'],
	instead: 'say "you" and "your" ("your login", not "the Staff member\'s login")'
};

// #2221: "Invoicing is unavailable until Staff issues a new Contract", read
// by the Staff member who would issue it. "Staff" followed by a verb makes
// the Practice's own people the third-person actor of a sentence, and that
// is wrong on every screen: a Staff reader is "you", and a Client never meets
// the word at all (GLOSSARY.md). So unlike the rules above, no reader has to
// be known, and the rule reads every screen and module. Name the act ("send
// a new Contract") or the reader ("you") instead.
const STAFF_AS_ACTOR: Rule = {
	id: 'Staff named as the one who acts, in copy a person reads',
	pattern: /\bStaff\s+(?:issues?|sends?|creates?|records?|adds?|approves?|has|have|must|needs?|will|can|should|did|does)\b/,
	globs: ['src/**/*.svelte', 'src/lib/**/*.ts'],
	instead: 'name the act ("To invoice again, send a new Contract") or say "you"'
};

const RULES: readonly Rule[] = [OWED, CLIENT_NOUN, STAFF_NOUN, STAFF_AS_ACTOR];

interface Offense {
	file: string;
	line: number;
	rule: string;
	instead: string;
	text: string;
}

function quotedIn(text: string): string[] {
	return text
		.matchAll(QUOTED)
		.map((match) => (match[1] ?? match[2] ?? match[3])!)
		.toArray();
}

// A `{...}` expression in markup is code, not copy -- `{@render
// table(owed, ...)}` is not a sentence -- so only the quoted strings inside
// it are kept. Repeated until none is left, because a template literal's
// `${...}` nests one inside another.
function withoutExpressions(text: string): string {
	const stripped = text.replaceAll(/\{[^{}]*\}/g, (expression) => ` ${quotedIn(expression).join(' ')} `);
	return stripped === text ? text : withoutExpressions(stripped);
}

function copyCandidates(file: string, source: string): { line: number; text: string }[] {
	if (file.endsWith('.ts')) return quotedStringsInSource(source);
	return regionLines(source).flatMap(({ line, text, region }) => {
		if (region === 'style') return [];
		if (region === 'markup') return [{ line, text: withoutExpressions(text) }];
		return quotedIn(text).map((quoted) => ({ line, text: quoted }));
	});
}

function findOffensesInSource(file: string, source: string, rules: readonly Rule[]): Offense[] {
	const raw = source.split('\n');
	return copyCandidates(file, source).flatMap(({ line, text }) => {
		if (MARKER_WITH_REASON.test(raw[line - 1]!)) return [];
		return rules
			.filter((rule) => rule.pattern.test(text))
			.map((rule) => ({ file, line, rule: rule.id, instead: rule.instead, text }));
	});
}

// Read and scanned once, at module scope, so the cost of the whole-tree
// read is paid on import rather than charged against one `it`'s timeout
// (#1211).
// A spec is not copy: it quotes the forms these rules forbid, this one
// included, in order to test them. Nor is a route fixture, which holds what
// a person typed, or a type declaration.
const scanned = RULES.map((rule) => ({
	rule,
	files: globFiles([...rule.globs], { cwd: appRoot }).filter((file) => !/\.(?:spec|fixture|d)\.ts$/.test(file))
}));

const offenses = scanned.flatMap(({ rule, files }) =>
	files.flatMap((file) =>
		findOffensesInSource(file, readFileSync(new URL(file, `file://${appRoot}`), 'utf8'), [rule])
	)
);

describe('product copy speaks to its reader as "you"', () => {
	it('reads files for every rule', () => {
		// A glob that silently matched nothing would make the assertion
		// below pass while checking no copy at all.
		for (const { rule, files } of scanned) {
			expect(files.length, rule.id).toBeGreaterThan(0);
		}
	});

	it('finds no impersonal form where the reader is known', () => {
		expect(
			offenses.map(
				(offense) =>
					`${offense.file}:${offense.line} "${offense.text.trim()}" -- ${offense.rule}. ` +
					`Rule 4 of the Voice section of docs/design/brief.md: product copy speaks to its reader as "you" and "your", so ${offense.instead}. ` +
					`If the form has to stay, put ${MARKER} and the reason on the line.`
			)
		).toEqual([]);
	});
});

describe('findOffensesInSource', () => {
	const owed = OWED;
	const client = CLIENT_NOUN;

	it('flags "owed" in markup text', () => {
		const found = findOffensesInSource('fixture.svelte', '<p>What is owed</p>', [owed]);

		expect(found).toEqual([
			{ file: 'fixture.svelte', line: 1, rule: owed.id, instead: owed.instead, text: '<p>What is owed</p>' }
		]);
	});

	it('flags the Client as a noun inside a quoted string of a .ts module', () => {
		const source = ['const clientRow = 1;', 'export const A = "The Client plan";'].join('\n');

		expect(findOffensesInSource('fixture.ts', source, [client]).map((offense) => offense.line)).toEqual([2]);
	});

	it('flags a quoted string in a <script> block, not the code around it', () => {
		const source = ['<script>', "const clientLabel = 'No longer owed';", '</script>'].join('\n');

		expect(findOffensesInSource('fixture.svelte', source, [owed, client]).map((o) => o.rule)).toEqual([
			owed.id
		]);
	});

	it('ignores a form named only in a comment', () => {
		const source = ['<!-- owed -->', '<p>ok</p>', '<script>', '// the Client', '</script>'].join('\n');

		expect(findOffensesInSource('fixture.svelte', source, [owed, client])).toEqual([]);
	});

	it('ignores an identifier in a markup expression but reads a string inside one', () => {
		const code = '{@render table(owed, owedEmptyMessage)}';
		const attribute = '<Text text={`Still owed ${total}`} />';

		expect(findOffensesInSource('fixture.svelte', code, [owed])).toEqual([]);
		expect(findOffensesInSource('fixture.svelte', attribute, [owed])).toHaveLength(1);
	});

	it('ignores a <style> line', () => {
		const source = ['<style>', '.owed { color: red; }', '</style>'].join('\n');

		expect(findOffensesInSource('fixture.svelte', source, [owed])).toEqual([]);
	});

	it('honors the marker with a reason on the same line', () => {
		const line = `<p>The Client signed.</p> <!-- ${MARKER}: quotes a Client's own words -->`;

		expect(findOffensesInSource('fixture.svelte', line, [client])).toEqual([]);
	});

	it('does not honor a marker with no reason', () => {
		const line = `<p>The Client signed.</p> <!-- ${MARKER} -->`;

		expect(findOffensesInSource('fixture.svelte', line, [client])).toHaveLength(1);
	});

	it('does not honor a marker on a different line', () => {
		const source = [`<!-- ${MARKER}: not this one -->`, '<p>The Client signed.</p>'].join('\n');

		expect(findOffensesInSource('fixture.svelte', source, [client])).toHaveLength(1);
	});

	it('flags Staff named as the one who acts, and leaves Staff as a noun alone', () => {
		const source = [
			"export const A = 'Invoicing is unavailable until Staff issues a new Contract.';",
			"export const B = 'Staff';",
			"export const C = 'Staff member';",
			"export const D = 'Who is on your Staff';"
		].join('\n');

		expect(findOffensesInSource('fixture.ts', source, [STAFF_AS_ACTOR]).map((offense) => offense.line)).toEqual([1]);
	});

	it('matches whole words only, so "owe" and "Clientele" pass', () => {
		const source = '<p>What you still owe, Clientele</p>';

		expect(findOffensesInSource('fixture.svelte', source, [owed, client])).toEqual([]);
	});
});

const filesOf = (target: Rule) => scanned.find(({ rule }) => rule === target)!.files;

describe('the rules are scoped by who reads the file', () => {
	it('keeps the Staff-screen rule on the account screen alone', () => {
		expect(filesOf(STAFF_NOUN).every((file) => file.startsWith('src/routes/(person)/account/'))).toBe(true);
	});

	it('reads the Staff-actor rule on both sides, and never a spec', () => {
		const files = filesOf(STAFF_AS_ACTOR);

		expect(files).toContain('src/lib/invoice.ts');
		expect(files.some((file) => file.startsWith('src/routes/portal/'))).toBe(true);
		expect(files.some((file) => file.endsWith('.spec.ts'))).toBe(false);
	});

	it('reads the Client rules on every file a Client reads, the one list both copy specs share (#2341)', () => {
		for (const rule of [OWED, CLIENT_NOUN]) {
			const files = filesOf(rule);

			expect(files).toContain('src/lib/yourCare.ts');
			expect(files).toContain('src/lib/portalDoor.ts');
			expect(files).toContain('src/lib/components/organisms/SignContract.svelte');
			expect(files.some((file) => file.endsWith('.fixture.ts'))).toBe(false);
		}
	});

	it('keeps the Client rules off every Staff route', () => {
		for (const rule of [OWED, CLIENT_NOUN]) {
			expect(filesOf(rule).some((file) => /^src\/routes\/(practices|\(person\))\//.test(file))).toBe(false);
		}
	});
});
