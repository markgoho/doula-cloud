#!/usr/bin/env bun
// `bun run dupes`: one jscpd scan, one scope, one table (#2743).
//
// The scope lives in `.jscpd.json` at the repo root (paths, formats,
// minTokens, ignores) and nowhere else, so every measurement of duplication
// reads the same files. This script runs jscpd with that config, sorts each
// clone into production or test, prints the table and writes the same
// numbers as JSON to `.jscpd-report/dupes.json` for a later CI comparison.
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

export const FORMATS = ['typescript', 'svelte', 'css', 'go'] as const;
export type Column = (typeof FORMATS)[number] | 'other';
export type Kind = 'production' | 'test';
export type Cell = { clones: number; lines: number };
export type Summary = Record<Kind, Record<Column | 'total', Cell>>;

/** The parts of a jscpd JSON report that this script reads. */
export type Clone = {
  format: string;
  lines: number;
  firstFile: { name: string; start: number; end: number };
  secondFile: { name: string; start: number; end: number };
};

const TEST_SUFFIX =
  /(_test\.go|\.spec\.ts|\.test\.ts|\.fixture\.ts|\.testing\.ts|Fixture\.ts|_fake\.go)$/;
const TEST_BASENAME = /^(fake\.go|[Tt]est[A-Z][^/]*|vitest\.[^/]*\.setup\.ts)$/;
const TEST_SEGMENTS = new Set([
  'e2e',
  'testdata',
  'testing',
  'fixtures',
  'harness',
  'vitest-examples',
  'testdb',
  'authntest',
  'apierrtest',
  'reqtest',
  'fixture',
]);

/** True when a file exists only to test, fake or stage something. */
export function isTestFile(file: string): boolean {
  const parts = file.split('/');
  const base = parts[parts.length - 1] ?? '';
  return (
    TEST_SUFFIX.test(base) ||
    TEST_BASENAME.test(base) ||
    parts.slice(0, -1).some((part) => TEST_SEGMENTS.has(part))
  );
}

/** A clone is test when either side is a test file. */
export function kindOf(clone: Clone): Kind {
  return isTestFile(clone.firstFile.name) || isTestFile(clone.secondFile.name)
    ? 'test'
    : 'production';
}

/** jscpd labels Svelte markup "markup"; the repo calls it svelte. */
export function columnOf(format: string): Column {
  const name = format === 'markup' ? 'svelte' : format;
  return (FORMATS as readonly string[]).includes(name)
    ? (name as Column)
    : 'other';
}

const emptyCell = (): Cell => ({ clones: 0, lines: 0 });

function emptyRow(): Record<Column | 'total', Cell> {
  return {
    typescript: emptyCell(),
    svelte: emptyCell(),
    css: emptyCell(),
    go: emptyCell(),
    other: emptyCell(),
    total: emptyCell(),
  };
}

export function summarize(clones: Clone[]): Summary {
  const summary: Summary = { production: emptyRow(), test: emptyRow() };
  for (const clone of clones) {
    const row = summary[kindOf(clone)];
    for (const cell of [row[columnOf(clone.format)], row.total]) {
      cell.clones += 1;
      cell.lines += clone.lines;
    }
  }
  return summary;
}

export function renderTable(summary: Summary): string {
  const columns: (Column | 'total')[] = [...FORMATS, 'total'];
  if (summary.production.other.clones + summary.test.other.clones > 0) {
    columns.splice(FORMATS.length, 0, 'other');
  }
  const header = ['', ...columns.map((c) => `${c} clones / lines`)];
  const rows: string[][] = [header];
  for (const kind of ['production', 'test'] as const) {
    rows.push([
      kind,
      ...columns.map(
        (c) => `${summary[kind][c].clones} / ${summary[kind][c].lines}`
      ),
    ]);
  }
  const both = (c: Column | 'total', key: keyof Cell) =>
    summary.production[c][key] + summary.test[c][key];
  rows.push([
    'both',
    ...columns.map((c) => `${both(c, 'clones')} / ${both(c, 'lines')}`),
  ]);
  const widths = header.map((_, i) =>
    Math.max(...rows.map((r) => (r[i] ?? '').length))
  );
  const line = (r: string[]) =>
    `| ${r.map((cell, i) => cell.padEnd(widths[i] ?? 0)).join(' | ')} |`;
  const rule = `|${widths.map((w) => '-'.repeat(w + 2)).join('|')}|`;
  const [head, ...body] = rows;
  return [line(head ?? []), rule, ...body.map(line)].join('\n');
}

export type Config = { path: string[]; output: string; ignore: string[] };

export const repoRoot = path.resolve(import.meta.dir, '..');

export function loadConfig(root: string): Config {
  return JSON.parse(
    fs.readFileSync(path.join(root, '.jscpd.json'), 'utf8')
  ) as Config;
}

/** Runs jscpd with the committed config; throws when it does not exit 0. */
export function scan(root: string): Clone[] {
  const config = loadConfig(root);
  const jscpd = path.join(root, 'node_modules', '.bin', 'jscpd');
  // jscpd 4.3 ignores `path` in the config file, so the paths go on the
  // command line from the same file.
  const run = spawnSync(
    jscpd,
    ['--config', path.join(root, '.jscpd.json'), ...config.path],
    { cwd: root, stdio: ['ignore', 'ignore', 'inherit'] }
  );
  if (run.status !== 0) {
    throw new Error(`jscpd exited ${run.status ?? run.signal}`);
  }
  const report = JSON.parse(
    fs.readFileSync(path.join(root, config.output, 'jscpd-report.json'), 'utf8')
  ) as { duplicates: Clone[] };
  return report.duplicates;
}

function main(): number {
  let clones: Clone[];
  try {
    clones = scan(repoRoot);
  } catch (error) {
    console.error((error as Error).message);
    return 1;
  }
  const summary = summarize(clones);
  fs.writeFileSync(
    path.join(repoRoot, loadConfig(repoRoot).output, 'dupes.json'),
    `${JSON.stringify(summary, null, 2)}
`
  );
  console.log(renderTable(summary));
  return 0;
}

if (import.meta.main) {
  process.exit(main());
}
