/**
 * Rendering `docs/design/doula-cloud.pen` into a plain-text, read-only
 * derivative that a session without Pen.app can read (#1080).
 *
 * Pure on purpose, the same split as practice-page.ts / sync-practice-
 * pages.ts: everything here is a value in and a string out, so the
 * export is testable against small fixtures without touching the real
 * (1 MB) `.pen` file on disk. export-design.ts does the reading and
 * writing.
 *
 * No Bun APIs are used in this file. It is imported by the test as well
 * as by the CLI script.
 */

export type PenNode = {
  type?: string;
  id?: string;
  name?: string;
  reusable?: boolean;
  children?: PenNode[];
  content?: string;
  ref?: string;
  descendants?: Record<string, Record<string, unknown>>;
  theme?: Record<string, string>;
  icon?: string;
  library?: string;
  [key: string]: unknown;
};

export type PenDocument = {
  children: PenNode[];
  [key: string]: unknown;
};

/** The command that regenerates this file, named in every failure that finds it stale. */
export const EXPORT_COMMAND = 'bun run design:export';

/**
 * Properties that describe a node's own identity or structure rather
 * than a styled value. Excluded from token-reference detection: `ref`
 * is a pointer to another node, not a design token, even though it is
 * also a bare string.
 */
const STRUCTURAL_KEYS = new Set([
  'type',
  'id',
  'name',
  'reusable',
  'children',
  'ref',
  'descendants',
  'theme',
  'icon',
  'library',
]);

/** Every id in the document, mapped to its own node, regardless of nesting depth. */
function buildIndex(document: PenDocument): Map<string, PenNode> {
  const index = new Map<string, PenNode>();
  const visit = (node: PenNode): void => {
    if (node.id) index.set(node.id, node);
    for (const child of node.children ?? []) visit(child);
  };
  for (const artboard of document.children) visit(artboard);
  return index;
}

/** A node's own name, or its type plus id when it has none to diff against. */
function label(node: PenNode): string {
  if (node.name) return node.name;
  if (node.id) return `${node.type ?? 'node'} (${node.id})`;
  return node.type ?? 'node';
}

/**
 * The `$`-prefixed `tokens.css` names a node's styled values point at.
 * Import is byte-exact (docs/design/workflow.md), so the token's own
 * name here is that same name, unprefixed.
 *
 * A per-edge property (`strokeWidth: {top: "$border-thin"}`, the same
 * shape CSS `border-top-width` uses) stores its token one level down,
 * so a plain-object value is walked one level for token strings too.
 */
function tokenRefs(node: PenNode): [string, string][] {
  const refs: [string, string][] = [];
  for (const [key, value] of Object.entries(node)) {
    if (STRUCTURAL_KEYS.has(key)) continue;
    if (typeof value === 'string' && value.startsWith('$')) {
      refs.push([key, value]);
    } else if (value && typeof value === 'object' && !Array.isArray(value)) {
      for (const [subKey, subValue] of Object.entries(value)) {
        if (typeof subValue === 'string' && subValue.startsWith('$')) {
          refs.push([`${key}.${subKey}`, subValue]);
        }
      }
    }
  }
  return refs;
}

/** An override's value, safe against an object value that would otherwise print `[object Object]`. */
function formatOverrideValue(value: unknown): string {
  if (
    typeof value === 'string' ||
    (value !== null && typeof value === 'object')
  ) {
    return JSON.stringify(value);
  }
  return String(value);
}

/** `{size: "sm"}` -> `size=sm`, for the theme axis pinned on a ref or a component master. */
function formatTheme(theme: Record<string, string>): string {
  return Object.entries(theme)
    .map(([axis, value]) => `${axis}=${value}`)
    .join(', ');
}

/** One line per descendant override, the target resolved to its own name and type. */
function renderDescendants(
  node: PenNode,
  index: Map<string, PenNode>,
  indent: string
): string[] {
  const lines: string[] = [];
  for (const [targetId, overrides] of Object.entries(node.descendants ?? {})) {
    const target = index.get(targetId);
    const targetLabel = target
      ? `${label(target)} (${target.type ?? 'node'})`
      : targetId;
    const overrideText = Object.entries(overrides)
      .map(([prop, value]) => `${prop}: ${formatOverrideValue(value)}`)
      .join('; ');
    lines.push(`${indent}- override ${targetLabel}: ${overrideText}`);
  }
  return lines;
}

function renderTokenSuffix(node: PenNode): string {
  const refs = tokenRefs(node);
  if (refs.length === 0) return '';
  return ` [${refs.map(([key, value]) => `${key}: ${value}`).join('; ')}]`;
}

/** " (hidden)" for a node disabled by default -- present in the tree, absent on screen. */
function renderHiddenSuffix(node: PenNode): string {
  return node.enabled === false ? ' (hidden)' : '';
}

/** " (theme: size=sm)" for an instance or a component master pinned to a theme axis. */
function renderThemeSuffix(node: PenNode): string {
  return node.theme ? ` (theme: ${formatTheme(node.theme)})` : '';
}

/** One node, and (except across a `ref`) its children, as an indented bullet list. */
function renderNode(
  node: PenNode,
  index: Map<string, PenNode>,
  depth: number
): string[] {
  const indent = '  '.repeat(depth);
  const lines: string[] = [];

  switch (node.type) {
    case 'text': {
      lines.push(
        `${indent}- text: ${JSON.stringify(node.content ?? '')}${renderHiddenSuffix(node)}${renderTokenSuffix(node)}`
      );
      break;
    }
    case 'icon': {
      lines.push(
        `${indent}- icon "${node.icon ?? '?'}" (${node.library ?? '?'})${renderHiddenSuffix(node)}${renderTokenSuffix(node)}`
      );
      break;
    }
    case 'ref': {
      const target = node.ref ? index.get(node.ref) : undefined;
      const targetName = target ? label(target) : (node.ref ?? '?');
      lines.push(
        `${indent}- ref "${label(node)}" -> component "${targetName}"${renderThemeSuffix(node)}`
      );
      lines.push(...renderDescendants(node, index, `${indent}  `));
      break;
    }
    case 'path': {
      lines.push(`${indent}- path "${label(node)}"${renderTokenSuffix(node)}`);
      break;
    }
    default: {
      // frame, or anything else with children: a structural node.
      lines.push(
        `${indent}- frame "${label(node)}"${renderHiddenSuffix(node)}${renderThemeSuffix(node)}${renderTokenSuffix(node)}`
      );
      for (const child of node.children ?? [])
        lines.push(...renderNode(child, index, depth + 1));
    }
  }

  return lines;
}

/** One `## Name` section: an artboard header plus its region tree. */
function renderArtboard(
  artboard: PenNode,
  index: Map<string, PenNode>
): string {
  const reusableSuffix = artboard.reusable ? ' (reusable)' : '';
  const heading = `${label(artboard)}${reusableSuffix}${renderThemeSuffix(artboard)}`;
  const body = (artboard.children ?? []).flatMap((child) =>
    renderNode(child, index, 0)
  );
  return [`## ${heading}`, '', ...body].join('\n');
}

/**
 * The whole read-back: a header naming the source and the regenerate
 * command, then one section per top-level artboard, in document order.
 *
 * An artboard is a top-level entry of the `.pen` file's own `children`
 * array, identified by its `name` — every artboard in
 * `doula-cloud.pen` is named and the names are unique, so this needs no
 * id fallback at that level (a nested, unnamed node still gets one; see
 * `label`).
 *
 * A `ref` is pointed at, never inlined: its target's own regions render
 * once, under the target's own artboard section, so an edit inside a
 * reusable component (`QuickCard`, `StaffTopBar`, ...) diffs in that one
 * place rather than at every screen that instantiates it.
 */
export function renderExport(document: PenDocument): string {
  const index = buildIndex(document);
  const sections = document.children.map((artboard) =>
    renderArtboard(artboard, index)
  );

  const header = [
    '# Design export',
    '',
    'Generated from `docs/design/doula-cloud.pen`. Do not hand-edit this file:',
    `run \`${EXPORT_COMMAND}\` to regenerate it, and commit the result alongside`,
    'the `.pen` change it was generated from. See docs/design/workflow.md for',
    'what this export is for, and what it is not.',
    '',
  ].join('\n');

  return [header, ...sections, ''].join('\n\n');
}
