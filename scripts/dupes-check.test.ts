import { describe, expect, test } from 'bun:test';
import type { Clone } from './dupes';
import {
  HINT,
  inScope,
  markerProblems,
  newClones,
  overlaps,
  parseDiff,
  render,
  renderClone,
} from './dupes-check';

const clone = (
  a: [string, number, number],
  b: [string, number, number]
): Clone => ({
  format: 'go',
  lines: a[2] - a[1] + 1,
  firstFile: { name: a[0], start: a[1], end: a[2] },
  secondFile: { name: b[0], start: b[1], end: b[2] },
});

describe('parseDiff', () => {
  const diff = [
    'diff --git a/api/a.go b/api/a.go',
    '--- a/api/a.go',
    '+++ b/api/a.go',
    '@@ -3 +3 @@ func x() {',
    '@@ -10,2 +10,4 @@',
    '@@ -20,3 +22,0 @@',
    'diff --git a/api/gone.go b/api/gone.go',
    '--- a/api/gone.go',
    '+++ /dev/null',
    '@@ -1,5 +0,0 @@',
    'diff --git a/app/src/b.ts b/app/src/b.ts',
    '--- /dev/null',
    '+++ b/app/src/b.ts',
    '@@ -0,0 +1,7 @@',
  ].join('\n');

  test('reads the new side of each hunk', () => {
    expect(parseDiff(diff).get('api/a.go')).toEqual([
      { start: 3, end: 3 },
      { start: 10, end: 13 },
    ]);
    expect(parseDiff(diff).get('app/src/b.ts')).toEqual([{ start: 1, end: 7 }]);
  });

  test('a pure deletion changes no line', () => {
    expect(parseDiff(diff).has('api/gone.go')).toBe(false);
  });

  test('an empty diff is no change', () => {
    expect(parseDiff('').size).toBe(0);
  });
});

describe('overlaps', () => {
  const ranges = [{ start: 10, end: 12 }];
  test.each([
    [1, 9, false],
    [1, 10, true],
    [11, 11, true],
    [12, 20, true],
    [13, 20, false],
  ])('%i-%i is %p', (start, end, expected) => {
    expect(overlaps(ranges, start, end)).toBe(expected);
  });

  test('a file with no changed lines never overlaps', () => {
    expect(overlaps(undefined, 1, 100)).toBe(false);
  });
});

describe('newClones', () => {
  const changed = parseDiff(
    ['+++ b/api/new.go', '@@ -0,0 +50,5 @@'].join('\n')
  );

  test('fails when the first fragment is touched', () => {
    const hit = clone(['api/new.go', 52, 60], ['api/old.go', 1, 9]);
    expect(newClones([hit], changed)).toEqual([hit]);
  });

  test('fails when only the second fragment is touched', () => {
    const hit = clone(['api/old.go', 1, 9], ['api/new.go', 40, 50]);
    expect(newClones([hit], changed)).toEqual([hit]);
  });

  test('passes a clone that sits wholly on unchanged lines', () => {
    const miss = clone(['api/new.go', 1, 49], ['api/old.go', 1, 49]);
    expect(newClones([miss], changed)).toEqual([]);
  });
});

describe('inScope', () => {
  const paths = ['app/src', 'api'];
  test.each([
    ['api/internal/x.go', true],
    ['app/src/lib/a.svelte', true],
    ['app/src/lib/a.css', true],
    ['app/src/lib/a.md', false],
    ['site/src/a.ts', false],
    ['apiary/x.go', false],
  ])('%s is %p', (file, expected) => {
    expect(inScope(file, paths)).toBe(expected);
  });
});

describe('markerProblems', () => {
  test.each([
    [
      'go',
      '// jscpd:ignore-start -- the two differ by purpose',
      '// jscpd:ignore-end',
    ],
    [
      'ts',
      '// jscpd:ignore-start -- shared shape, own purposes',
      '// jscpd:ignore-end',
    ],
    [
      'svelte',
      '<!-- jscpd:ignore-start -- markup mirrors the Staff form -->',
      '<!-- jscpd:ignore-end -->',
    ],
    [
      'css',
      '/* jscpd:ignore-start -- token ramp, one per theme */',
      '/* jscpd:ignore-end */',
    ],
  ])('%s: a reason passes', (_, start, end) => {
    expect(markerProblems('f', `a\n${start}\nb\n${end}\n`)).toEqual([]);
  });

  test.each([
    '// jscpd:ignore-start',
    '// jscpd:ignore-start --',
    '// jscpd:ignore-start -- ',
    '// jscpd:ignore-start because it is fine',
    '<!-- jscpd:ignore-start -->',
    '<!-- jscpd:ignore-start -- -->',
    '/* jscpd:ignore-start */',
    '/* jscpd:ignore-start -- */',
  ])('%s fails for a missing reason', (start) => {
    const found = markerProblems('f.go', `x\n${start}\nb\njscpd:ignore-end\n`);
    expect(found).toHaveLength(1);
    expect(found[0]).toMatchObject({ file: 'f.go', line: 2 });
    expect(found[0]?.message).toContain('no reason');
  });

  test('a start with no end after it fails', () => {
    const found = markerProblems('f.go', '// jscpd:ignore-start -- why\nx\n');
    expect(found).toEqual([
      {
        file: 'f.go',
        line: 1,
        message: 'jscpd:ignore-start has no jscpd:ignore-end after it',
      },
    ]);
  });

  test('a closed region and a later reopened one are tracked apart', () => {
    const text = [
      '// jscpd:ignore-start -- one',
      '// jscpd:ignore-end',
      '// jscpd:ignore-start -- two',
    ].join('\n');
    expect(markerProblems('f.go', text).map((f) => f.line)).toEqual([3]);
  });

  test('a file without markers has no problems', () => {
    expect(markerProblems('f.go', 'package x\n')).toEqual([]);
  });
});

describe('render', () => {
  const hit = clone(['api/a.go', 5, 20], ['api/b.go', 60, 75]);

  test('names both locations and ranges, and the hint', () => {
    expect(renderClone(hit)).toBe(
      '  api/a.go:5-20  duplicates  api/b.go:60-75  (16 lines, go)'
    );
    const out = render([hit], []);
    expect(out).toContain('1 clone overlaps lines');
    expect(out).toContain('api/a.go:5-20');
    expect(out).toContain('api/b.go:60-75');
    expect(out).toContain(HINT);
    expect(HINT).toContain('jscpd:ignore-start -- <reason>');
  });

  test('pluralizes and reports marker problems', () => {
    const out = render(
      [hit, hit],
      [
        { file: 'a.go', line: 3, message: 'm' },
        { file: 'b.go', line: 4, message: 'm' },
      ]
    );
    expect(out).toContain('2 clones overlap');
    expect(out).toContain('2 jscpd marker problems:');
    expect(out).toContain('  a.go:3  m');
  });

  test('markers alone print no clone section', () => {
    const out = render([], [{ file: 'a.go', line: 3, message: 'm' }]);
    expect(out).toBe('1 jscpd marker problem:\n  a.go:3  m');
  });
});
