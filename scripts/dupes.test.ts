import { describe, expect, test } from 'bun:test';
import {
  columnOf,
  isTestFile,
  kindOf,
  renderTable,
  summarize,
  type Clone,
} from './dupes';

const clone = (format: string, lines: number, a: string, b: string): Clone => ({
  format,
  lines,
  firstFile: { name: a, start: 1, end: lines },
  secondFile: { name: b, start: 1, end: lines },
});

describe('isTestFile', () => {
  test.each([
    'api/internal/visit/visit_test.go',
    'app/src/lib/client.spec.ts',
    'app/src/lib/x.test.ts',
    'app/e2e/login.ts',
    'api/internal/x/testdata/a.json',
    'app/src/testing/routePage.ts',
    'app/src/routes/(signed-out)/login/page.fixture.ts',
    'app/src/routes/routeFixture.ts',
    'app/src/lib/components/molecules/MenuButton.testing.ts',
    'app/src/lib/components/atoms/harness/LinkInSentence.svelte',
    'app/src/lib/testOutcome.ts',
    'app/src/lib/TestLoaded.svelte',
    'app/src/vitest.browser.setup.ts',
    'api/internal/testdb/seed.go',
    'api/internal/authntest/account.go',
    'api/internal/mail/fake.go',
    'api/internal/payments/stripe_fake.go',
  ])('%s is test', (file) => {
    expect(isTestFile(file)).toBe(true);
  });

  test.each([
    'api/internal/visit/visit.go',
    'api/internal/activitypage/latest.go',
    'app/src/lib/client.ts',
    'app/src/lib/testament.ts',
    'app/src/routes/practices/[practiceId]/+page.svelte',
  ])('%s is production', (file) => {
    expect(isTestFile(file)).toBe(false);
  });
});

describe('summarize', () => {
  const clones = [
    clone('go', 10, 'api/a.go', 'api/b.go'),
    clone('go', 7, 'api/a.go', 'api/b_test.go'),
    clone('markup', 20, 'app/src/a.svelte', 'app/src/b.svelte'),
    clone('typescript', 5, 'app/src/a.spec.ts', 'app/src/b.spec.ts'),
    clone('weird', 3, 'api/a.go', 'api/c.go'),
  ];

  test('a clone is test when either side is', () => {
    expect(kindOf(clones[1] as Clone)).toBe('test');
    expect(kindOf(clones[0] as Clone)).toBe('production');
  });

  test('markup is counted as svelte and unknown formats as other', () => {
    expect(columnOf('markup')).toBe('svelte');
    expect(columnOf('weird')).toBe('other');
  });

  test('sums clones and lines per kind and format', () => {
    const summary = summarize(clones);
    expect(summary.production.go).toEqual({ clones: 1, lines: 10 });
    expect(summary.production.svelte).toEqual({ clones: 1, lines: 20 });
    expect(summary.production.total).toEqual({ clones: 3, lines: 33 });
    expect(summary.test.go).toEqual({ clones: 1, lines: 7 });
    expect(summary.test.typescript).toEqual({ clones: 1, lines: 5 });
    expect(summary.test.total).toEqual({ clones: 2, lines: 12 });
  });

  test('the table shows an other column only when it has clones', () => {
    expect(renderTable(summarize(clones))).toContain('other clones / lines');
    expect(renderTable(summarize(clones.slice(0, 4)))).not.toContain('other');
  });
});
