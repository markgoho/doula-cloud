/**
 * Tests for the read-back export (#1080). Fixture-based: small,
 * hand-built documents exercise renderExport's behavior without
 * touching the real (1 MB) `.pen` file. The one test that reads the
 * real file lives at the bottom, and is the CI drift guard: it fails
 * loudly, naming the fix command, when doula-cloud.pen and
 * doula-cloud.export.md disagree.
 */

import { describe, expect, test } from 'bun:test';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import {
  EXPORT_COMMAND,
  renderExport,
  type PenDocument,
} from './design-export';

function doc(children: PenDocument['children']): PenDocument {
  return { children };
}

describe('renderExport', () => {
  test('names the artboard and shows its text content in order', () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'a1',
          name: 'Sign in',
          children: [
            {
              type: 'text',
              id: 't1',
              name: 'Heading',
              content: 'Welcome back',
            },
            {
              type: 'text',
              id: 't2',
              name: 'Subheading',
              content: 'Sign in to continue',
            },
          ],
        },
      ])
    );

    const headingIndex = out.indexOf('Welcome back');
    const subheadingIndex = out.indexOf('Sign in to continue');
    expect(out).toContain('## Sign in');
    expect(headingIndex).toBeGreaterThan(-1);
    expect(subheadingIndex).toBeGreaterThan(headingIndex);
  });

  test('marks a reusable artboard so a component reads differently from a screen', () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'a1',
          name: 'QuickCard',
          reusable: true,
          children: [],
        },
      ])
    );

    expect(out).toContain('## QuickCard (reusable)');
  });

  test("nests a frame's children under it in document order", () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'a1',
          name: 'Screen',
          children: [
            {
              type: 'frame',
              id: 'f1',
              name: 'Row',
              children: [{ type: 'text', id: 't1', content: 'Cell' }],
            },
          ],
        },
      ])
    );

    expect(out).toContain('frame "Row"');
    const rowIndex = out.indexOf('frame "Row"');
    const cellIndex = out.indexOf('Cell');
    expect(cellIndex).toBeGreaterThan(rowIndex);
  });

  test('resolves a ref to the reusable component it instantiates, by name', () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'comp1',
          name: 'StaffTopBar',
          reusable: true,
          children: [],
        },
        {
          type: 'frame',
          id: 'a1',
          name: 'Screen',
          children: [{ type: 'ref', id: 'r1', name: 'Top Bar', ref: 'comp1' }],
        },
      ])
    );

    expect(out).toContain('ref "Top Bar" -> component "StaffTopBar"');
  });

  test('falls back to the raw id when a ref points at nothing in the document', () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'a1',
          name: 'Screen',
          children: [
            { type: 'ref', id: 'r1', name: 'Ghost', ref: 'missing-id' },
          ],
        },
      ])
    );

    expect(out).toContain('ref "Ghost" -> component "missing-id"');
  });

  test("resolves each descendant override to the target's own name, not its id", () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'comp1',
          name: 'QuickCard',
          reusable: true,
          children: [
            {
              type: 'text',
              id: 'label1',
              name: 'Label',
              content: 'placeholder',
            },
          ],
        },
        {
          type: 'frame',
          id: 'a1',
          name: 'Screen',
          children: [
            {
              type: 'ref',
              id: 'r1',
              name: 'Clients',
              ref: 'comp1',
              descendants: { label1: { content: 'Clients' } },
            },
          ],
        },
      ])
    );

    expect(out).toContain('Label (text): content: "Clients"');
    expect(out).not.toContain('label1');
  });

  test('surfaces a token reference distinctly from a literal value', () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'a1',
          name: 'Screen',
          children: [
            {
              type: 'text',
              id: 't1',
              content: 'Hello',
              fill: '$text',
              fontSize: 16,
            },
          ],
        },
      ])
    );

    expect(out).toContain('fill: $text');
    expect(out).not.toContain('fontSize');
  });

  test("shows an icon's name and library", () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'a1',
          name: 'Screen',
          children: [
            { type: 'icon', id: 'i1', icon: 'users', library: 'lucide' },
          ],
        },
      ])
    );

    expect(out).toContain('icon "users" (lucide)');
  });

  test("falls back to the node's type, plus its id, when a node has no name", () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'a1',
          name: 'Screen',
          children: [{ type: 'frame', id: 'f9', children: [] }],
        },
      ])
    );

    expect(out).toContain('frame (f9)');
  });

  test('marks a node disabled by default as hidden', () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'a1',
          name: 'Screen',
          children: [
            {
              type: 'text',
              id: 't1',
              name: 'Error',
              content: 'Required',
              enabled: false,
            },
          ],
        },
      ])
    );

    expect(out).toContain('text: "Required" (hidden)');
  });

  test('shows a theme axis pinned on a ref or frame', () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'comp1',
          name: 'CloudMark',
          reusable: true,
          children: [],
        },
        {
          type: 'frame',
          id: 'a1',
          name: 'Screen',
          children: [
            {
              type: 'ref',
              id: 'r1',
              name: 'Mark',
              ref: 'comp1',
              theme: { size: 'sm' },
            },
          ],
        },
      ])
    );

    expect(out).toContain('(theme: size=sm)');
  });

  test('surfaces a token nested one level inside an object property', () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'a1',
          name: 'Screen',
          children: [
            {
              type: 'frame',
              id: 'f1',
              name: 'Field',
              strokeWidth: { top: '$border-thin' },
              children: [],
            },
          ],
        },
      ])
    );

    expect(out).toContain('strokeWidth.top: $border-thin');
  });

  test("stringifies an object-valued override instead of printing '[object Object]'", () => {
    const out = renderExport(
      doc([
        {
          type: 'frame',
          id: 'comp1',
          name: 'QuickCard',
          reusable: true,
          children: [
            { type: 'frame', id: 'target1', name: 'Icon', children: [] },
          ],
        },
        {
          type: 'frame',
          id: 'a1',
          name: 'Screen',
          children: [
            {
              type: 'ref',
              id: 'r1',
              name: 'Clients',
              ref: 'comp1',
              descendants: { target1: { theme: { size: 'sm' } } },
            },
          ],
        },
      ])
    );

    expect(out).toContain('theme: {"size":"sm"}');
    expect(out).not.toContain('[object Object]');
  });

  test('carries a generated-file header naming the fix command', () => {
    const out = renderExport(doc([]));
    expect(out).toContain(EXPORT_COMMAND);
    expect(out.toLowerCase()).toContain('generated');
  });
});

describe('the committed export', () => {
  test('matches what regenerating from the real .pen file produces', async () => {
    const root = path.resolve(import.meta.dirname, '..');
    const penPath = path.join(root, 'docs/design/doula-cloud.pen');
    const exportPath = path.join(root, 'docs/design/doula-cloud.export.md');

    const parsed = JSON.parse(await readFile(penPath, 'utf8')) as PenDocument;
    const fresh = renderExport(parsed);
    const committed = await readFile(exportPath, 'utf8');

    if (fresh !== committed) {
      throw new Error(
        'docs/design/doula-cloud.export.md is stale relative to doula-cloud.pen. ' +
          `Run \`${EXPORT_COMMAND}\` and commit the result.`
      );
    }
  });
});
