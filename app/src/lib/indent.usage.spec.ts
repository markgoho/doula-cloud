import { globSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * #1232's check behind .editorconfig's [app/**] override: app/ is
 * tab-indented, so a line whose indentation starts with a space is the
 * one failure mode that override exists to catch -- an editor that still
 * applied the old bare [*] 2-space rule on save.
 *
 * The rule stops there on purpose. Once a line's indentation begins with
 * a tab, anything after it -- more tabs, or the handful of spaces this
 * tree already uses to hang-align a wrapped union type or object literal
 * under an opening brace -- is a choice this check has no opinion about.
 * A fuller opinion is what @stylistic/eslint-plugin's indent rule tried
 * to hold during this ticket's own investigation, and it disagreed with
 * several of those hand-aligned lines rather than finding a real
 * space-indented file; app/ has no Prettier of its own either (#1120
 * gave that to the two tooling trees only), so this narrow spec is the
 * gate instead. See docs/testing.md's "Formatting" section for the
 * decision.
 *
 * The one thing that legitimately starts a line with spaces today is a
 * continuation line inside a block comment (/* ... * / in a <script> or
 * <style> block, <!-- ... --> in markup), where prose wraps aligned
 * under the opener rather than under a tab stop -- #1120's own reformat
 * commit found and hand-fixed the same shape in two tooling-tree files.
 * findViolations' openComment is the state this spec keeps, line to
 * line, to recognize one. IGNORE_MARKER is the same per-line escape hatch as
 * spelling:ignore and motion:ignore, for a case this two-token scan
 * can't tell apart from a real violation -- a multi-line template
 * literal whose embedded content is deliberately not tab-indented, for
 * instance.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const IGNORE_MARKER = 'indent:ignore';
const SELF = 'src/lib/indent.usage.spec.ts';

interface Violation {
	readonly file: string;
	readonly line: number;
	readonly text: string;
}

type CommentToken = '/*' | '<!--';

const COMMENT_TOKENS: readonly [CommentToken, string][] = [
	['/*', '*/'],
	['<!--', '-->']
];

function isCommentClosedBy(token: CommentToken, text: string): boolean {
	const closeToken = COMMENT_TOKENS.find(([open]) => open === token)?.[1];
	return closeToken !== undefined && text.includes(closeToken);
}

// A comment opens on this line only if its close token is absent, or comes
// before its open token (a leftover close from an already-open comment).
function opens(text: string): CommentToken | undefined {
	for (const [open, close] of COMMENT_TOKENS) {
		const openIndex = text.indexOf(open);
		if (openIndex === -1) continue;
		const closeIndex = text.indexOf(close);
		if (closeIndex === -1 || closeIndex < openIndex) return open;
	}
	return undefined;
}

function findViolations(file: string, source: string): Violation[] {
	const violations: Violation[] = [];
	let openComment: CommentToken | undefined;
	const lines = source.split('\n');
	for (const [index, text] of lines.entries()) {
		if (openComment === undefined && !text.includes(IGNORE_MARKER) && /^ +[^ ]/.test(text)) {
			violations.push({ file, line: index + 1, text });
		}
		openComment =
			openComment !== undefined && isCommentClosedBy(openComment, text) ? undefined : (openComment ?? opens(text));
	}
	return violations;
}

const appFiles = globSync('src/**/*.{svelte,ts}', { cwd: appRoot }).filter((file) => file !== SELF);
const e2eFiles = globSync('e2e/**/*.ts', { cwd: appRoot });
const scriptFiles = globSync('scripts/**/*.ts', { cwd: appRoot });
const rootFiles = globSync('*.{ts,js}', { cwd: appRoot });
const allFiles = [...appFiles, ...e2eFiles, ...scriptFiles, ...rootFiles];

const violations = allFiles.flatMap((file) => findViolations(file, readFileSync(path.join(appRoot, file), 'utf8')));

describe('app/ indents with tabs (#1232)', () => {
	it('reads the whole app/ tree', () => {
		expect(allFiles.length).toBeGreaterThan(500);
	});

	it('finds no line whose indentation starts with a space', () => {
		expect(violations).toEqual([]);
	});
});
