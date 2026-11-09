#!/usr/bin/env bun
// `bun run dupes:check`: fail when the lines a PR adds or changes duplicate
// code that is already there (#2743). No baseline: the existing clones are
// not this PR's to fix, only the ones it touches.
//
// The scan is `scan()` from dupes.ts, so the scope stays in `.jscpd.json`.
// A clone fails when either fragment overlaps a changed line. The escape
// hatch is jscpd's own `jscpd:ignore-start` ... `jscpd:ignore-end`, and the
// start line has to carry `-- <reason>`; a marker without one fails the
// check wherever it sits, so it cannot be dodged by leaving the line alone.
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { loadConfig, repoRoot, scan, type Clone } from './dupes';

export type Range = { start: number; end: number };
export type ChangedLines = Map<string, Range[]>;
export type Finding = { file: string; line: number; message: string };

const SOURCE_EXTENSIONS = ['.go', '.ts', '.svelte', '.css'];

export const HINT =
  'Extract the shared piece (fixture.Scene, reqtest.Expect, pagecursor, routePage ...) or, when it must stay apart, wrap it in `jscpd:ignore-start -- <reason>` ... `jscpd:ignore-end` (see docs/testing.md, "Measuring duplication").';

/** The new-side line ranges of every hunk in `git diff --unified=0`. */
export function parseDiff(diff: string): ChangedLines {
  const changed: ChangedLines = new Map();
  let file: string | null = null;
  for (const line of diff.split('\n')) {
    if (line.startsWith('+++ ')) {
      const target = line.slice(4).replace(/^"|"$/g, '');
      file = target.startsWith('b/') ? target.slice(2) : null;
      continue;
    }
    const hunk = /^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@/.exec(line);
    if (!hunk || file === null) continue;
    const start = Number(hunk[1]);
    const count = hunk[2] === undefined ? 1 : Number(hunk[2]);
    if (count === 0) continue;
    const ranges = changed.get(file) ?? [];
    ranges.push({ start, end: start + count - 1 });
    changed.set(file, ranges);
  }
  return changed;
}

export function overlaps(
  ranges: Range[] | undefined,
  start: number,
  end: number
) {
  return (ranges ?? []).some((r) => r.start <= end && start <= r.end);
}

/** The clones with a fragment that overlaps a changed line. */
export function newClones(clones: Clone[], changed: ChangedLines): Clone[] {
  return clones.filter(
    (c) =>
      overlaps(
        changed.get(c.firstFile.name),
        c.firstFile.start,
        c.firstFile.end
      ) ||
      overlaps(
        changed.get(c.secondFile.name),
        c.secondFile.start,
        c.secondFile.end
      )
  );
}

/** True when a changed file is one the jscpd scope reads. */
export function inScope(file: string, paths: string[]): boolean {
  return (
    SOURCE_EXTENSIONS.some((ext) => file.endsWith(ext)) &&
    paths.some((p) => file.startsWith(`${p}/`))
  );
}

const MARKER = /jscpd:ignore-(start|end)(.*)$/;

/**
 * Marker problems in one file: a start with no reason after `--` (a comment
 * closer such as `-->` or `*\/` does not count as one), and a start that no
 * end follows, which jscpd silently does not honor.
 */
export function markerProblems(file: string, text: string): Finding[] {
  const findings: Finding[] = [];
  let open: number | null = null;
  text.split('\n').forEach((raw, index) => {
    const found = MARKER.exec(raw);
    if (!found) return;
    const line = index + 1;
    if (found[1] === 'end') {
      open = null;
      return;
    }
    const rest = (found[2] ?? '').replace(/\s*(?:-->|\*\/)\s*$/, '');
    const reason = /^\s*--\s*(\S.*)$/.exec(rest)?.[1];
    if (!reason) {
      findings.push({
        file,
        line,
        message:
          'jscpd:ignore-start has no reason; write `jscpd:ignore-start -- <why this stays apart>`',
      });
    }
    open ??= line;
  });
  if (open !== null) {
    findings.push({
      file,
      line: open,
      message: 'jscpd:ignore-start has no jscpd:ignore-end after it',
    });
  }
  return findings;
}

export function renderClone(c: Clone): string {
  const where = (f: Clone['firstFile']) => `${f.name}:${f.start}-${f.end}`;
  return `  ${where(c.firstFile)}  duplicates  ${where(c.secondFile)}  (${c.lines} lines, ${c.format})`;
}

export function render(failing: Clone[], markers: Finding[]): string {
  const out: string[] = [];
  if (failing.length > 0) {
    out.push(
      `${failing.length === 1 ? '1 clone overlaps' : `${failing.length} clones overlap`} lines this change adds or changes:`,
      ...failing.map(renderClone),
      '',
      HINT
    );
  }
  if (markers.length > 0) {
    if (out.length > 0) out.push('');
    out.push(
      `${markers.length} jscpd marker problem${markers.length === 1 ? '' : 's'}:`,
      ...markers.map((m) => `  ${m.file}:${m.line}  ${m.message}`)
    );
  }
  return out.join('\n');
}

function git(args: string[]): string {
  const run = spawnSync('git', ['-c', 'core.quotepath=off', ...args], {
    cwd: repoRoot,
    encoding: 'utf8',
    maxBuffer: 1 << 28,
  });
  if (run.status !== 0) {
    throw new Error(`git ${args.join(' ')} failed: ${run.stderr.trim()}`);
  }
  return run.stdout;
}

function main(): number {
  const base = process.argv[2] ?? 'origin/trunk';
  const config = loadConfig(repoRoot);
  let changed: ChangedLines;
  let files: string[];
  try {
    const mergeBase = git(['merge-base', base, 'HEAD']).trim();
    changed = parseDiff(git(['diff', '--unified=0', mergeBase]));
    files = git(['ls-files', '--', ...config.path])
      .split('\n')
      .filter((f) => inScope(f, config.path));
  } catch (error) {
    console.error(
      `${(error as Error).message}\nThe check needs ${base} and enough history to find the merge base (CI: fetch-depth: 0).`
    );
    return 1;
  }

  const markers = files.flatMap((f) =>
    markerProblems(f, fs.readFileSync(path.join(repoRoot, f), 'utf8'))
  );
  const touched = [...changed.keys()].some((f) => inScope(f, config.path));
  let failing: Clone[] = [];
  if (touched) {
    try {
      failing = newClones(scan(repoRoot), changed);
    } catch (error) {
      console.error((error as Error).message);
      return 1;
    }
  }

  if (failing.length === 0 && markers.length === 0) {
    console.log(
      touched
        ? 'dupes:check: no clone overlaps a changed line.'
        : 'dupes:check: no scanned code changed.'
    );
    return 0;
  }
  console.error(render(failing, markers));
  return 1;
}

if (import.meta.main) {
  process.exit(main());
}
