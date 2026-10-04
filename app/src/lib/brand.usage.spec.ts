import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { globFiles } from './globFiles';

/*
 * #1463's gate. ADR-0050 settled the product's name as one word, DoulaCloud,
 * in every place a person reads it, and this spec fails the build on the
 * two-word form coming back: in the app, its end-to-end specs, the
 * marketing site, the BFF, the hand-written documents, and the scripts,
 * workflows and Terraform beside them.
 *
 * Identifiers and slugs are not the name, so `doula-cloud` (the repo, the
 * GCP project, the `.pen` file) is never a match: the check is the exact
 * two words, capital D and capital C. It also reads a name that a hard
 * wrap split across two lines, "Doula" ending one line and "Cloud" opening
 * the next after any comment leader, since a Go or Svelte comment wraps
 * at a column and the split form is the same name.
 *
 * Out of the sweep, as #1463 draws it: `docs/research/` (evidence, often a
 * verbatim quotation), `api/db/migrations/` (goose never re-runs an
 * applied file, and its SQL is not in the Go glob anyway). Unlike the
 * spelling gate, this one does read `docs/design/doula-cloud.export.md`,
 * the generated read-back of the `.pen` file: it cannot carry a marker,
 * but it has no reason to, and a two-word name there means the canvas
 * still draws it. The fix is on the canvas. A verbatim quotation anywhere else
 * stays by putting `brand:ignore` and the reason on the line, the same
 * shape as `spelling:ignore`; in Markdown the marker is an HTML comment,
 * `<!-- brand:ignore: the reason -->`.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));
const repoRoot = path.join(appRoot, '..');

const IGNORE_MARKER = 'brand:ignore';

// The two words are built rather than written, so this file does not
// fail its own check and a grep of the repo for them finds real ones.
const TWO_WORDS = ['Doula', 'Cloud'].join(' ');
const ENDS_WITH_FIRST = /\bDoula\s*$/;
// The leaders a wrapped comment or Markdown line can open with: Go and
// TypeScript line and block comments, shell and YAML, SQL, HTML, and a
// Markdown blockquote.
const OPENS_WITH_SECOND = /^\s*(?:\/\/|\/\*|#|\*|--|<!--|>)?\s*Cloud\b/;

interface Offense {
	file: string;
	line: number;
}

// The marker exempts the line it is on. A split name spans two lines, so a
// marker on either of them exempts it.
function findTwoWordNames(file: string, source: string): Offense[] {
	const lines = source.split('\n');
	return lines.flatMap((text, index) => {
		const next = lines[index + 1] ?? '';
		const isMarked = text.includes(IGNORE_MARKER) || next.includes(IGNORE_MARKER);
		const isOnOneLine = text.includes(TWO_WORDS) && !text.includes(IGNORE_MARKER);
		const isSplit = ENDS_WITH_FIRST.test(text) && OPENS_WITH_SECOND.test(next) && !isMarked;
		return isOnOneLine || isSplit ? [{ file, line: index + 1 }] : [];
	});
}

const repoFiles = [
	...globFiles('app/src/**/*.{svelte,ts,js,css,svg,html,md}', { cwd: repoRoot }).filter(
		(file) => file !== 'app/src/lib/brand.usage.spec.ts'
	),
	...globFiles('app/static/**/*.{webmanifest,html,svg,json}', { cwd: repoRoot }),
	...globFiles('app/e2e/**/*.{ts,js}', { cwd: repoRoot }),
	...globFiles('site/src/**/*.{svelte,ts,js,css,svg,html,md}', { cwd: repoRoot }),
	...globFiles('api/**/*.go', { cwd: repoRoot }),
	...globFiles('scripts/**/*.{ts,js}', { cwd: repoRoot }),
	...globFiles('.github/**/*.{yml,yaml,md}', { cwd: repoRoot }),
	...globFiles('terraform/**/*.tf', { cwd: repoRoot }),
	...globFiles('docs/**/*.{md,html}', { cwd: repoRoot }).filter(
		(file) => !file.startsWith('docs/research/')
	),
	'GLOSSARY.md',
	'README.md',
	'CLAUDE.md',
	'CONTRIBUTING.md',
	'firebase.json'
];

// Read and scanned once, at module scope, for the reason
// spelling.usage.spec.ts gives (#1211): under the full suite's contention a
// walk of this many files can outlast one `it`'s timeout.
const offenses = repoFiles.flatMap((file) =>
	findTwoWordNames(file, readFileSync(path.join(repoRoot, file), 'utf8'))
);

describe('the product is named DoulaCloud, one word (ADR-0050)', () => {
	it('reads every tree #1463 names', () => {
		for (const prefix of ['app/src/', 'app/static/', 'app/e2e/', 'site/src/', 'api/', 'docs/']) {
			expect(repoFiles.some((file) => file.startsWith(prefix)), prefix).toBe(true);
		}
		expect(repoFiles.some((file) => file.startsWith('docs/research/'))).toBe(false);
	});

	it('finds the two words on one line, and not the one-word name or a slug', () => {
		expect(findTwoWordNames('a.md', `Welcome to ${TWO_WORDS}.`)).toEqual([{ file: 'a.md', line: 1 }]);
		expect(findTwoWordNames('a.md', 'Welcome to DoulaCloud, at doula-cloud.')).toEqual([]);
	});

	it('finds the two words split across a hard-wrapped comment', () => {
		expect(findTwoWordNames('a.go', '// renders as "Doula\n// Cloud", never "System"')).toEqual([
			{ file: 'a.go', line: 1 }
		]);
		expect(findTwoWordNames('a.go', '// a contractor Doula\n// attaches to the Engagement')).toEqual([]);
		expect(findTwoWordNames('a.md', '> the Doula \n> Cloud team')).toEqual([{ file: 'a.md', line: 1 }]);
	});

	it('leaves a line that carries the marker', () => {
		expect(findTwoWordNames('a.md', `"${TWO_WORDS}", two words. <!-- ${IGNORE_MARKER}: quoted -->`)).toEqual(
			[]
		);
		expect(findTwoWordNames('a.go', `// "Doula\n// Cloud" // ${IGNORE_MARKER}: quoted`)).toEqual([]);
	});

	it('finds no two-word name anywhere a person reads it', () => {
		expect(
			offenses.map(
				(offense) =>
					`${offense.file}:${offense.line} writes the product's name as two words -- write DoulaCloud, or put ${IGNORE_MARKER} and a reason on the line`
			)
		).toEqual([]);
	});
});
