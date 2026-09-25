/**
 * Tests for the data a Practice's page at doula.cloud/p/<slug> is built
 * from (#441).
 *
 * No database. The only thing that talks to Postgres is one function in
 * sync-practice-pages.ts that hands rows to these. What the page does
 * with the data -- and that it escapes whatever she typed -- is proved by
 * site/src/routes/p/[slug]/practice-page.svelte.spec.ts.
 */

import { describe, expect, test } from 'bun:test';
import { mkdtemp, readFile, readdir, writeFile, mkdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import type { PracticePage } from '../site/src/lib/practicePage';
import { renderPage, toPage, writePages } from './sync-practice-pages';

const page: PracticePage = {
  slug: 'rochester-doulas',
  name: 'Rochester Doulas',
  serviceDescription: 'Birth and postpartum support across Monroe County.',
  cancellationPolicy:
    'Cancel more than 30 days before the due date for a full refund.',
  supportName: 'Maya Chen',
  supportEmail: 'maya@rochesterdoulas.com',
  publishedAt: '2026-08-29T14:00:00.000Z',
};

describe('renderPage', () => {
  test("carries every field Stripe's website standard asks for", () => {
    // #382 read these off Stripe's own written requirements: the
    // business name, a description of the services, a support contact,
    // and a refund or cancellation position.
    expect(JSON.parse(renderPage(page))).toEqual(page);
  });

  test('keeps her words exactly, whatever characters are in them', () => {
    const hostile: PracticePage = {
      ...page,
      name: 'The "Best" Doulas: #1',
      cancellationPolicy:
        '<script>alert("x")</script> * 50% [refund](javascript:void 0)\nSecond line.',
    };
    expect(JSON.parse(renderPage(hostile))).toEqual(hostile);
  });
});

describe('toPage', () => {
  const row = {
    slug: 'rochester-doulas',
    name: 'Rochester Doulas',
    service_description: page.serviceDescription,
    cancellation_policy: page.cancellationPolicy,
    support_name: 'Maya Chen',
    support_email: 'maya@rochesterdoulas.com',
    published_at: new Date('2026-08-29T14:00:00.000Z'),
  };

  test('reads a complete row', () => {
    expect(toPage(row)).toEqual(page);
  });

  test('refuses a row that would publish an incomplete page', () => {
    // A hosted row without its two facts is impossible (00045's CHECK)
    // and a Practice with no Owner is impossible too -- so this is data
    // corruption, and publishing half a page would hide it behind a
    // Stripe review that then fails for a reason nobody can see.
    expect(() => toPage({ ...row, support_email: null })).toThrow(
      /support_email/
    );
    expect(() => toPage({ ...row, cancellation_policy: null })).toThrow(
      /refusing to publish an incomplete page/
    );
  });

  test('falls back to now when a page has somehow never been published', () => {
    const dated = toPage({ ...row, published_at: null });
    expect(Date.parse(dated.publishedAt)).not.toBeNaN();
  });
});

describe('writePages', () => {
  test('writes one file per page, named for the slug, and removes what is no longer published', async () => {
    const root = path.join(
      await mkdtemp(path.join(tmpdir(), 'pages-')),
      'practice-pages'
    );

    // A page from an earlier build, for a Practice who has since gone
    // back to her own website. Nothing reconciles it away -- the
    // directory is rebuilt, which is the only way generated output
    // forgets.
    await mkdir(root, { recursive: true });
    await writeFile(path.join(root, 'gone-away.json'), 'stale', 'utf8');

    await writePages([{ ...page, name: 'Genesee Birth Collective' }], root);

    // The slug, not the name, names the file: renaming the Practice must
    // not move the URL Stripe holds (#382).
    expect(await readdir(root)).toEqual(['rochester-doulas.json']);
    const written = JSON.parse(
      await readFile(path.join(root, 'rochester-doulas.json'), 'utf8')
    ) as PracticePage;
    expect(written.supportEmail).toBe('maya@rochesterdoulas.com');
  });

  test('empties the directory when nothing is published', async () => {
    const root = path.join(
      await mkdtemp(path.join(tmpdir(), 'pages-')),
      'practice-pages'
    );
    await writePages([page], root);
    await writePages([], root);
    expect(await readdir(root)).toEqual([]);
  });
});
