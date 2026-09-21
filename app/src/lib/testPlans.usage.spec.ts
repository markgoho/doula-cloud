import { globSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * #1233: a test plan's Marks summary is arithmetic over its own Steps
 * table, and docs/test-plans/README.md's run-status table and Total row
 * are arithmetic over the plans -- all of it derivable from the files
 * themselves. Nothing read any of it before this. #685 found four plans
 * whose summary had drifted from its own Steps table; #1121 found two
 * more, by hand, a second time -- the build stayed green through both,
 * because a mark can be re-counted wrong and nothing notices.
 *
 * This spec is that read, in the mold of `spelling.usage.spec.ts` and
 * `adrNumbers.usage.spec.ts`: parse the Markdown tables the documents
 * already carry and recompute what a person was recomputing by hand.
 * Whether a mark is *honest* -- whether `automated (some.e2e.ts)` really
 * exercises the step the way the Persona would -- stays a desk-pass
 * judgment; this only checks that the numbers agree with each other.
 *
 * A "plan" is any file under docs/test-plans/ that carries a `## Marks`
 * heading. The nine persona plans do; `connect-onboarding.md` (a walk
 * recipe referenced from README's prose) and `README.md` itself do not,
 * and README is read separately below.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));
const repoRoot = path.join(appRoot, '..');
const testPlansDirectory = path.join(repoRoot, 'docs', 'test-plans');
const e2eDir = path.join(appRoot, 'e2e');

type MarkKind = 'automated' | 'manual' | 'blocked' | 'missing-feature';

const MARK_KINDS: readonly MarkKind[] = ['automated', 'manual', 'blocked', 'missing-feature'];

function isMarkKind(value: string): value is MarkKind {
	return (MARK_KINDS as readonly string[]).includes(value);
}

interface ParsedMark {
	readonly kind: MarkKind;
	// The spec filename (`automated`) or comma-separated gap IDs
	// (`missing-feature`); absent for `manual`/`blocked`.
	readonly detail: string | undefined;
}

interface Table {
	readonly header: readonly string[];
	readonly rows: readonly (readonly string[])[];
}

// -- Markdown table parsing, generic over any GFM table in these docs --

function splitRow(line: string): string[] {
	const inner = line.trim().replace(/^\|/, '').replace(/\|$/, '');
	return inner.split('|').map((cell) => cell.trim());
}

const SEPARATOR_ROW = /^\|?\s*:?-+:?\s*(?:\|\s*:?-+:?\s*)*\|?$/;

function isSeparatorRow(line: string): boolean {
	return SEPARATOR_ROW.test(line.trim());
}

// A header row, a separator row, then one or more `|`-led data rows.
// Parses every such block in `text`, in the order it appears.
function parseTables(text: string): Table[] {
	const lines = text.split('\n');
	const tables: Table[] = [];
	let index = 0;
	while (index < lines.length) {
		const line = lines[index] ?? '';
		const next = lines[index + 1];
		if (next === undefined || !isSeparatorRow(next) || !line.trimStart().startsWith('|')) {
			index++;
			continue;
		}
		const header = splitRow(line);
		const rows: string[][] = [];
		let dataIndex = index + 2;
		while (dataIndex < lines.length && (lines[dataIndex] ?? '').trimStart().startsWith('|')) {
			rows.push(splitRow(lines[dataIndex] ?? ''));
			dataIndex++;
		}
		tables.push({ header, rows });
		index = dataIndex;
	}
	return tables;
}

// The text strictly between two `## `/`### ` headings named by their exact
// text. Omitting `endName` runs to the end of the file -- README's final
// section (`### Gap issues`) has no heading after it.
function sectionBetween(text: string, startName: string, endName?: string): string {
	const lines = text.split('\n');
	const headingIndex = (name: string) =>
		lines.findIndex((line) => {
			const match = /^#{2,3}\s+(.*)$/.exec(line.trim());
			return match !== null && match[1] === name;
		});
	const start = headingIndex(startName);
	if (start === -1) return '';
	const end = endName === undefined ? -1 : headingIndex(endName);
	return lines.slice(start + 1, end === -1 ? undefined : end).join('\n');
}

// -- The four marks README.md defines ------------------------------------

// Matches from the start of a Mark cell: the backtick-quoted mark itself,
// ignoring anything after it (an issue link, e.g. `` `manual` [#261](...) ``).
// A `manual`/`blocked` mark takes no parenthetical; `automated`/
// `missing-feature` require one -- both are enforced below, not by the
// regex, so a shape mismatch (`` `automated` `` with nothing in it) is
// reported the same way as an unrecognized mark word.
const MARK_TOKEN = /^`(automated|manual|blocked|missing-feature)(?: \(([^)]*)\))?`/;

// `undefined` for a cell that is not one of the four marks README's
// "## The four marks" section defines, in the shape it defines them.
function parseMark(cell: string): ParsedMark | undefined {
	const match = MARK_TOKEN.exec(cell.trim());
	if (match === null) return undefined;
	const kind = match[1];
	if (!isMarkKind(kind)) return undefined;
	const detail = match[2];
	const requiresDetail = kind === 'automated' || kind === 'missing-feature';
	if (requiresDetail !== (detail !== undefined)) return undefined;
	return { kind, detail };
}

type Tally = Record<MarkKind, number>;

function emptyTally(): Tally {
	return { automated: 0, manual: 0, blocked: 0, 'missing-feature': 0 };
}

interface Classified {
	readonly tally: Tally;
	readonly marks: readonly ParsedMark[];
	// A cell that matched none of the four marks -- reported verbatim.
	readonly invalidCells: readonly string[];
}

// Every `| ... | Mark |` cell inside `stepsSection` -- both the plan's own
// Steps tables and, where the plan carries one (Priya's), the Permission
// boundary table, since its steps are counted in the Marks summary too
// ("5 of them the permission boundary"). Any table whose last header cell
// is not `Mark` (a Run log's `Result` column, a recipe's `Value` column)
// is not a Marks table and is skipped.
function classifyStepCells(stepsSection: string): Classified {
	const cells = parseTables(stepsSection)
		.filter((table) => table.header.at(-1) === 'Mark')
		.flatMap((table) => table.rows.map((row) => row.at(-1) ?? ''));
	const tally = emptyTally();
	const marks: ParsedMark[] = [];
	const invalidCells: string[] = [];
	for (const cell of cells) {
		const mark = parseMark(cell);
		if (mark === undefined) {
			invalidCells.push(cell);
			continue;
		}
		tally[mark.kind]++;
		marks.push(mark);
	}
	return { tally, marks, invalidCells };
}

// The `## Marks` summary table itself: `| Mark | Steps |`, where the
// `Steps` cell is a leading integer, sometimes followed by prose or a gap
// list ("9 steps over 6 gaps (...)"). A mark the table carries no row for
// (`blocked` on most plans) defaults to 0 by the caller, not here.
function parseMarksSummary(marksSection: string): Partial<Tally> {
	const table = parseTables(marksSection).find(
		(candidate) => candidate.header[0] === 'Mark' && candidate.header[1] === 'Steps'
	);
	if (table === undefined) return {};
	const summary: Partial<Tally> = {};
	for (const row of table.rows) {
		const kind = (row[0] ?? '').replaceAll('`', '').trim();
		const countMatch = /^(\d+)/.exec((row[1] ?? '').trim());
		if (countMatch !== null && isMarkKind(kind)) summary[kind] = Number(countMatch[1]);
	}
	return summary;
}

function summaryOffenses(plan: string, tally: Tally, summary: Partial<Tally>): string[] {
	return MARK_KINDS.filter((kind) => (summary[kind] ?? 0) !== tally[kind]).map(
		(kind) =>
			`${plan}: the Marks summary says \`${kind}\` is ${summary[kind] ?? 0}, but the Steps table holds ${tally[kind]}`
	);
}

function invalidMarkOffenses(plan: string, invalidCells: readonly string[]): string[] {
	return invalidCells.map(
		(cell) =>
			`${plan}: the Mark cell "${cell}" is not one of the four marks README.md's "## The four marks" section defines`
	);
}

// -- A single plan's Steps + Marks, read off disk ------------------------

interface Plan {
	readonly file: string;
	readonly tally: Tally;
	readonly summary: Partial<Tally>;
	readonly automatedSpecs: readonly string[];
	readonly missingFeatureGapIds: readonly string[];
	readonly offenses: readonly string[];
}

function readPlan(file: string, text: string): Plan | undefined {
	// `## Marks` is what makes a file under docs/test-plans/ a plan rather
	// than a walk recipe (connect-onboarding.md) -- the only other kind of
	// file the directory holds besides README.md, which is read separately.
	if (!/^## Marks\s*$/m.test(text)) return undefined;
	const { tally, marks, invalidCells } = classifyStepCells(sectionBetween(text, 'Steps', 'Marks'));
	const summary = parseMarksSummary(sectionBetween(text, 'Marks', 'Run log'));
	const automatedSpecs = marks
		.filter((mark) => mark.kind === 'automated')
		.map((mark) => mark.detail ?? '');
	const missingFeatureGapIds = marks
		.filter((mark) => mark.kind === 'missing-feature')
		.flatMap((mark) => (mark.detail ?? '').split(',').map((id) => id.trim()));
	const offenses = [...summaryOffenses(file, tally, summary), ...invalidMarkOffenses(file, invalidCells)];
	return { file, tally, summary, automatedSpecs, missingFeatureGapIds, offenses };
}

function missingSpecOffenses(plan: Plan, e2eSpecs: ReadonlySet<string>): string[] {
	return plan.automatedSpecs
		.filter((spec) => !e2eSpecs.has(spec))
		.map((spec) => `${plan.file}: \`automated (${spec})\` names a spec that does not exist under app/e2e/`);
}

const GAP_ID = /^[A-Z]{2,3}-G\d+$/;

function unrecognizedGapIdOffenses(plan: Plan, recognizedGapIds: ReadonlySet<string>): string[] {
	return plan.missingFeatureGapIds
		.filter((id) => !recognizedGapIds.has(id))
		.map(
			(id) =>
				`${plan.file}: \`missing-feature (${id})\` cites a gap ID absent from README.md's gap table`
		);
}

// -- README.md: the gap-ID vocabulary, the run-status table, the Total --

// Every gap ID README.md owns an answer for: the primary `| Gap | Issue |
// Parent |` table, and the "Not filed here, and why" table, whose first
// column lists several IDs per row (e.g. practice-owner.md's RA-G4 lives
// only there, folded into #225).
function recognizedGapIds(readmeText: string): Set<string> {
	const ids = new Set<string>();
	const tables = parseTables(sectionBetween(readmeText, 'Gap issues'));
	for (const table of tables) {
		if (table.header[0] !== 'Gap') continue;
		for (const row of table.rows) {
			const tokens = (row[0] ?? '').split(',');
			for (const token of tokens) {
				const id = token.trim();
				if (GAP_ID.test(id)) ids.add(id);
			}
		}
	}
	return ids;
}

function planFileFromCell(cell: string): string | undefined {
	return /\[([\w.-]+\.md)]/.exec(cell)?.[1];
}

function numberFromCell(cell: string): number {
	const match = /(\d+)/.exec(cell);
	return match === null ? 0 : Number(match[1]);
}

// The `| Plan | Persona | \`automated\` | \`manual\` | \`blocked\` |
// \`missing-feature\` | ` table under "## The run, and the gap issues" --
// one row per plan, a `**Total**` row last. Column order matches
// MARK_KINDS, starting at index 2.
function runStatusTable(readmeText: string): Table | undefined {
	const section = sectionBetween(readmeText, 'The run, and the gap issues', 'Gap issues');
	return parseTables(section).find((table) => table.header[0] === 'Plan' && table.header[1] === 'Persona');
}

function isTotalRow(row: readonly string[]): boolean {
	return (row[0] ?? '').replaceAll('*', '').trim() === 'Total';
}

// One offense per (plan, mark) pair where README's run-status row
// disagrees with that plan's own Marks summary -- not with a fresh
// recount of its Steps table, so this fails on exactly the drift AC2
// describes even if AC1's check were somehow silenced.
function runStatusOffenses(table: Table, plans: readonly Plan[]): string[] {
	const byFile = new Map(plans.map((plan) => [plan.file, plan]));
	const offenses: string[] = [];
	for (const row of table.rows) {
		if (isTotalRow(row)) continue;
		const file = planFileFromCell(row[0] ?? '');
		const plan = file === undefined ? undefined : byFile.get(file);
		if (plan === undefined) continue;
		for (const [index, kind] of MARK_KINDS.entries()) {
			const readmeCount = numberFromCell(row[2 + index] ?? '');
			const summaryCount = plan.summary[kind] ?? 0;
			if (readmeCount !== summaryCount) {
				offenses.push(
					`README.md: the run-status row for ${file} says \`${kind}\` is ${readmeCount}, but ${file}'s Marks summary says ${summaryCount}`
				);
			}
		}
	}
	return offenses;
}

// The Total row against the sum of every plan row above it -- arithmetic
// entirely inside README.md, independent of any plan file.
function totalRowOffenses(table: Table): string[] {
	const totalRow = table.rows.find((row) => isTotalRow(row));
	if (totalRow === undefined) return ["README.md: the run-status table has no 'Total' row"];
	const planRows = table.rows.filter((row) => planFileFromCell(row[0] ?? '') !== undefined);
	const offenses: string[] = [];
	for (const [index, kind] of MARK_KINDS.entries()) {
		const columnIndex = 2 + index;
		const sum = planRows.reduce((total, row) => total + numberFromCell(row[columnIndex] ?? ''), 0);
		const totalCount = numberFromCell(totalRow[columnIndex] ?? '');
		if (sum !== totalCount) {
			offenses.push(
				`README.md: the Total row's \`${kind}\` is ${totalCount}, but the rows above it sum to ${sum}`
			);
		}
	}
	return offenses;
}

// -- Read the real tree, once, at module scope (spelling.usage.spec.ts's
// rationale applies here too: pay this cost on import, not inside a
// timed `it`) ------------------------------------------------------------

const planFiles = globSync('*.md', { cwd: testPlansDirectory })
	.filter((file) => file !== 'README.md')
	.toSorted((a, b) => a.localeCompare(b));

const plans = planFiles
	.map((file) => readPlan(file, readFileSync(path.join(testPlansDirectory, file), 'utf8')))
	.filter((plan): plan is Plan => plan !== undefined);

const e2eSpecs = new Set(globSync('*.e2e.ts', { cwd: e2eDir }));

const readmeText = readFileSync(path.join(testPlansDirectory, 'README.md'), 'utf8');
const gapIds = recognizedGapIds(readmeText);
const runStatus = runStatusTable(readmeText);

const offenses: string[] = [
	...plans.flatMap((plan) => plan.offenses),
	...plans.flatMap((plan) => missingSpecOffenses(plan, e2eSpecs)),
	...plans.flatMap((plan) => unrecognizedGapIdOffenses(plan, gapIds)),
	...(runStatus === undefined
		? ["README.md: no run-status table found under '## The run, and the gap issues'"]
		: [...runStatusOffenses(runStatus, plans), ...totalRowOffenses(runStatus)])
];

// -- Fixtures, shared by the unit tests below -----------------------------

const FIXTURE_STEPS_SECTION = [
	'### Stage 1',
	'',
	'| Step | Action | Expected result | Mark |',
	'| --- | --- | --- | --- |',
	'| 1.1 | Do a thing | It does the thing | `automated (signup-form.e2e.ts)` |',
	'| 1.2 | Do another | It refuses | `manual` |',
	'| 1.3 | Look for it | Not built | `missing-feature (TB-G1)` |',
	''
].join('\n');

const FIXTURE_PLAN = [
	'# Fixture — test plan',
	'',
	'## Preconditions',
	'',
	'None.',
	'',
	'## Steps',
	'',
	FIXTURE_STEPS_SECTION,
	'## Marks',
	'',
	'| Mark | Steps |',
	'| --- | --- |',
	'| `automated` | 1 |',
	'| `manual` | 1 |',
	'| `missing-feature` | 1 ([TB-G1](https://github.com/markgoho/doula-cloud/issues/284)) |',
	'',
	'## Run log',
	''
].join('\n');

function fakePlan(file: string, summary: Partial<Tally>): Plan {
	return { file, tally: emptyTally(), summary, automatedSpecs: [], missingFeatureGapIds: [], offenses: [] };
}

describe('parseTables', () => {
	it('parses several tables in one document, skipping the prose between them', () => {
		const text = [
			'Some prose before any table.',
			'',
			'| A | B |',
			'| --- | --- |',
			'| 1 | 2 |',
			'| 3 | 4 |',
			'',
			'More prose, then a second table.',
			'',
			'| X | Y | Z |',
			'| --- | --- | --- |',
			'| a | b | c |'
		].join('\n');

		expect(parseTables(text)).toEqual([
			{
				header: ['A', 'B'],
				rows: [
					['1', '2'],
					['3', '4']
				]
			},
			{ header: ['X', 'Y', 'Z'], rows: [['a', 'b', 'c']] }
		]);
	});

	it('finds no table in prose with no pipes at all', () => {
		expect(parseTables('Nothing here looks like a table.')).toEqual([]);
	});
});

describe('sectionBetween', () => {
	const text = [
		'# Doc',
		'## Steps',
		'step content',
		'## Marks',
		'marks content',
		'## Run log',
		'log content'
	].join('\n');

	it('reads the text strictly between two named headings', () => {
		expect(sectionBetween(text, 'Steps', 'Marks')).toBe('step content');
	});

	it('reads to end of file when no end heading is given', () => {
		expect(sectionBetween(text, 'Run log')).toBe('log content');
	});

	it('reads to end of file when the named end heading does not exist', () => {
		expect(sectionBetween(text, 'Marks', 'Nonexistent')).toBe('marks content\n## Run log\nlog content');
	});

	it('returns empty text when the start heading does not exist', () => {
		expect(sectionBetween(text, 'Nonexistent')).toBe('');
	});
});

describe('parseMark', () => {
	it('parses each of the four marks in the shape README.md defines', () => {
		expect(parseMark('`manual`')).toEqual({ kind: 'manual', detail: undefined });
		expect(parseMark('`blocked`')).toEqual({ kind: 'blocked', detail: undefined });
		expect(parseMark('`automated (signup-form.e2e.ts)`')).toEqual({
			kind: 'automated',
			detail: 'signup-form.e2e.ts'
		});
		expect(parseMark('`missing-feature (TB-G1)`')).toEqual({ kind: 'missing-feature', detail: 'TB-G1' });
	});

	it('ignores a trailing issue link after the mark itself', () => {
		expect(
			parseMark('`missing-feature (TB-G1)` [#284](https://github.com/markgoho/doula-cloud/issues/284)')
		).toEqual({ kind: 'missing-feature', detail: 'TB-G1' });
	});

	it('rejects a mark word that is not one of the four', () => {
		expect(parseMark('`retired`')).toBeUndefined();
	});

	it('rejects automated/missing-feature with no detail, and manual/blocked with one', () => {
		expect(parseMark('`automated`')).toBeUndefined();
		expect(parseMark('`missing-feature`')).toBeUndefined();
		expect(parseMark('`manual (some detail)`')).toBeUndefined();
		expect(parseMark('`blocked (some detail)`')).toBeUndefined();
	});

	it('rejects text with no backtick-quoted mark at all', () => {
		expect(parseMark('some prose, no mark')).toBeUndefined();
	});
});

describe('classifyStepCells', () => {
	it('tallies each mark once and reports a cell matching none of the four', () => {
		const withOneInvalid = FIXTURE_STEPS_SECTION.replace('`manual`', '`retired`');
		const { tally, invalidCells } = classifyStepCells(withOneInvalid);
		expect(tally).toEqual({ automated: 1, manual: 0, blocked: 0, 'missing-feature': 1 });
		expect(invalidCells).toEqual(['`retired`']);
	});

	it('only reads tables whose last header cell is Mark, e.g. skips a Run log', () => {
		const withRunLogTable = [
			FIXTURE_STEPS_SECTION,
			'| Step | Spec | Result |',
			'| --- | --- | --- |',
			'| 1.1 | signup-form.e2e.ts | pass |'
		].join('\n');
		expect(classifyStepCells(withRunLogTable).tally).toEqual({
			automated: 1,
			manual: 1,
			blocked: 0,
			'missing-feature': 1
		});
	});
});

describe('parseMarksSummary', () => {
	it('reads the leading integer off each row, ignoring trailing prose', () => {
		const section = [
			'| Mark | Steps |',
			'| --- | --- |',
			'| `automated` | 1 |',
			'| `manual` | 1 |',
			'| `missing-feature` | 1 steps over 1 gap ([TB-G1](https://github.com/markgoho/doula-cloud/issues/284)) |'
		].join('\n');
		expect(parseMarksSummary(section)).toEqual({ automated: 1, manual: 1, 'missing-feature': 1 });
	});

	it('returns an empty summary when the Marks table itself is absent', () => {
		expect(parseMarksSummary('no table here')).toEqual({});
	});
});

describe('summaryOffenses', () => {
	it('names the plan, the mark and both numbers when they disagree', () => {
		const tally: Tally = { automated: 1, manual: 2, blocked: 0, 'missing-feature': 1 };
		const summary: Partial<Tally> = { automated: 1, manual: 3, 'missing-feature': 1 };
		expect(summaryOffenses('example.md', tally, summary)).toEqual([
			'example.md: the Marks summary says `manual` is 3, but the Steps table holds 2'
		]);
	});

	it('defaults a mark the summary carries no row for to 0', () => {
		expect(summaryOffenses('example.md', emptyTally(), {})).toEqual([]);
	});
});

describe('invalidMarkOffenses', () => {
	it('names the plan and the offending cell', () => {
		expect(invalidMarkOffenses('example.md', ['`retired`'])).toEqual([
			'example.md: the Mark cell "`retired`" is not one of the four marks README.md\'s "## The four marks" section defines'
		]);
	});
});

describe('readPlan', () => {
	it("reads a plan's tally, summary, automated specs and gap IDs off disk", () => {
		const plan = readPlan('fixture.md', FIXTURE_PLAN);
		expect(plan?.tally).toEqual({ automated: 1, manual: 1, blocked: 0, 'missing-feature': 1 });
		expect(plan?.summary).toEqual({ automated: 1, manual: 1, 'missing-feature': 1 });
		expect(plan?.automatedSpecs).toEqual(['signup-form.e2e.ts']);
		expect(plan?.missingFeatureGapIds).toEqual(['TB-G1']);
		expect(plan?.offenses).toEqual([]);
	});

	it('is not a plan without a Marks heading, e.g. the connect-onboarding.md recipe', () => {
		expect(readPlan('connect-onboarding.md', '# A recipe\n\nNo Marks heading here.')).toBeUndefined();
	});

	it('reports a summary/Steps disagreement through the same offenses this ticket asks for', () => {
		const drifted = FIXTURE_PLAN.replace('| `manual` | 1 |', '| `manual` | 2 |');
		expect(readPlan('fixture.md', drifted)?.offenses).toEqual([
			'fixture.md: the Marks summary says `manual` is 2, but the Steps table holds 1'
		]);
	});
});

describe('missingSpecOffenses', () => {
	it('names a spec an automated mark cites that does not exist under app/e2e/', () => {
		const plan = readPlan('fixture.md', FIXTURE_PLAN);
		if (plan === undefined) throw new Error('fixture plan failed to parse');
		expect(missingSpecOffenses(plan, new Set())).toEqual([
			'fixture.md: `automated (signup-form.e2e.ts)` names a spec that does not exist under app/e2e/'
		]);
		expect(missingSpecOffenses(plan, new Set(['signup-form.e2e.ts']))).toEqual([]);
	});
});

describe('unrecognizedGapIdOffenses', () => {
	it('names a gap ID absent from the recognized set', () => {
		const plan = readPlan('fixture.md', FIXTURE_PLAN);
		if (plan === undefined) throw new Error('fixture plan failed to parse');
		expect(unrecognizedGapIdOffenses(plan, new Set())).toEqual([
			"fixture.md: `missing-feature (TB-G1)` cites a gap ID absent from README.md's gap table"
		]);
		expect(unrecognizedGapIdOffenses(plan, new Set(['TB-G1']))).toEqual([]);
	});

	it('splits a comma-separated gap list into one ID per entry', () => {
		const twoGaps = FIXTURE_PLAN.replace('`missing-feature (TB-G1)`', '`missing-feature (TB-G1, TB-G2)`');
		expect(readPlan('fixture.md', twoGaps)?.missingFeatureGapIds).toEqual(['TB-G1', 'TB-G2']);
	});
});

describe('recognizedGapIds', () => {
	it("reads the primary Gap table and splits the Not-filed table's comma lists", () => {
		const readme = [
			'### Gap issues',
			'',
			'| Gap | Issue | Parent |',
			'| --- | --- | --- |',
			'| TB-G1 | [#284](https://github.com/markgoho/doula-cloud/issues/284) | [x](x) |',
			'',
			'**Not filed here, and why:**',
			'',
			'| Gap | Where it lives instead |',
			'| --- | --- |',
			'| RA-G4, PR-G1 | [x](https://github.com/markgoho/doula-cloud/issues/225) owns these |'
		].join('\n');
		expect(recognizedGapIds(readme)).toEqual(new Set(['TB-G1', 'RA-G4', 'PR-G1']));
	});
});

describe('planFileFromCell and numberFromCell', () => {
	it('extracts the linked plan filename, or undefined for a cell with none', () => {
		expect(planFileFromCell('[evaluator-doula.md](evaluator-doula.md)')).toBe('evaluator-doula.md');
		expect(planFileFromCell('**Total**')).toBeUndefined();
	});

	it('extracts the leading integer, or 0 for a cell with none', () => {
		expect(numberFromCell('**40**')).toBe(40);
		expect(numberFromCell('')).toBe(0);
	});
});

describe('runStatusTable, isTotalRow, runStatusOffenses and totalRowOffenses', () => {
	const readmeSection = [
		'## The run, and the gap issues',
		'',
		'| Plan | Persona | `automated` | `manual` | `blocked` | `missing-feature` |',
		'| --- | --- | --- | --- | --- | --- |',
		'| [fixture.md](fixture.md) | Fixture Persona | 1 | 2 | 0 | 1 |',
		'| **Total** | | **1** | **2** | **0** | **1** |',
		'',
		'### Gap issues',
		''
	].join('\n');

	it('finds the run-status table under its own heading', () => {
		expect(runStatusTable(readmeSection)?.header).toEqual([
			'Plan',
			'Persona',
			'`automated`',
			'`manual`',
			'`blocked`',
			'`missing-feature`'
		]);
	});

	it('returns undefined when the section carries no such table', () => {
		expect(
			runStatusTable('## The run, and the gap issues\n\nno table here\n\n### Gap issues\n')
		).toBeUndefined();
	});

	it("reports a run-status row that disagrees with the plan's own Marks summary", () => {
		const table = runStatusTable(readmeSection);
		if (table === undefined) throw new Error('fixture run-status table failed to parse');
		const plan = fakePlan('fixture.md', { automated: 1, manual: 3, blocked: 0, 'missing-feature': 1 });
		expect(runStatusOffenses(table, [plan])).toEqual([
			"README.md: the run-status row for fixture.md says `manual` is 2, but fixture.md's Marks summary says 3"
		]);
	});

	it('skips the Total row, and a row naming a plan not passed in', () => {
		const table = runStatusTable(readmeSection);
		if (table === undefined) throw new Error('fixture run-status table failed to parse');
		expect(runStatusOffenses(table, [])).toEqual([]);
	});

	it('reports the Total row disagreeing with the sum of the rows above it', () => {
		const table = runStatusTable(readmeSection.replace('**2**', '**5**'));
		if (table === undefined) throw new Error('fixture run-status table failed to parse');
		expect(totalRowOffenses(table)).toEqual([
			"README.md: the Total row's `manual` is 5, but the rows above it sum to 2"
		]);
	});

	it('reports a missing Total row', () => {
		const noTotal: Table = {
			header: ['Plan', 'Persona', '`automated`', '`manual`', '`blocked`', '`missing-feature`'],
			rows: [['[fixture.md](fixture.md)', 'Fixture Persona', '1', '2', '0', '1']]
		};
		expect(totalRowOffenses(noTotal)).toEqual(["README.md: the run-status table has no 'Total' row"]);
	});
});

describe('docs/test-plans/ arithmetic, read off the real tree (#1233)', () => {
	it('finds all nine persona plans, so an empty sweep cannot pass silently', () => {
		expect(plans.map((plan) => plan.file)).toEqual([
			'contractor-doula.md',
			'employed-doula.md',
			'evaluator-doula.md',
			'first-time-client.md',
			'loss-client.md',
			'non-doula-admin.md',
			'practice-owner.md',
			'returning-postpartum-client.md',
			'solo-birth-doula.md'
		]);
	});

	it('excludes connect-onboarding.md, which carries no Marks heading', () => {
		expect(planFiles).toContain('connect-onboarding.md');
		expect(plans.some((plan) => plan.file === 'connect-onboarding.md')).toBe(false);
	});

	it('reads the real app/e2e/ and README.md gap tables, so neither check is vacuous', () => {
		expect(e2eSpecs.size).toBeGreaterThan(10);
		expect(gapIds.size).toBeGreaterThan(10);
	});

	it('finds the real run-status table', () => {
		expect(runStatus).not.toBeUndefined();
	});

	it(
		"agrees with itself: every plan's Marks summary matches its own Steps table, README's run-status " +
			"table and Total match the plans, every mark is one of the four, every automated spec exists " +
			'under app/e2e/, and every missing-feature gap ID is one README.md owns',
		() => {
			expect(offenses).toEqual([]);
		}
	);
});
