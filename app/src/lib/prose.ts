/*
 * #776's mechanism: unwrap hard-wrapped markdown prose back to one line
 * per paragraph, list item and blockquote, without touching a fenced code
 * block, a table, a heading, front matter, a link reference definition, or
 * a deliberate hard break (trailing two spaces or a trailing backslash).
 * CLAUDE.md states the rule this enforces -- a single `\n` inside a
 * markdown paragraph is a soft break that GitHub's renderer reflows, so a
 * hard-wrapped file only reads as truncated to a raw viewer (`gh issue
 * view`, a diff, the API).
 *
 * `unwrapProse` is a source-level line transform, not a parse-and-
 * reserialize through a full markdown AST: a reserializer normalizes
 * bullet characters, blank-line counts and spacing it was never asked to
 * touch, which would turn a prose-only fix into a reformat of the whole
 * tree. This walks CommonMark's own block grammar (fence, ATX heading,
 * thematic break, link reference definition, table, blockquote, list
 * item, paragraph) just far enough to know which lines are one
 * paragraph's wrap and which are a different construct, and changes only
 * the wrapped ones.
 *
 * `unwrapProse` is also the gate: `prose.usage.spec.ts` asserts it is a
 * no-op across every in-scope file, in the mold of `adrNumbers.usage.
 * spec.ts` and `spelling.usage.spec.ts` -- a PR that reintroduces a
 * wrapped paragraph makes `unwrapProse` change the file again and fails
 * the same check the #776 migration itself ran. Unlike those two, the
 * logic lives in its own module rather than inside the spec: the one-off
 * migration that unwrapped the existing tree needed to call it from a
 * plain script, outside the Vitest runner that importing a `*.spec.ts`
 * file would have pulled in.
 *
 * One construct is GitHub's, not CommonMark's: a GitHub Alert
 * (`> [!WARNING]`, `> [!NOTE]`, ...) only renders as the colored callout
 * if its `[!TYPE]` marker sits alone on the blockquote's first line.
 * CommonMark itself sees no reason not to join that line into the
 * sentence after it -- they're lazy continuation of one paragraph, no
 * blank line between them -- and joining them is exactly what silently
 * broke `docs/research/stripe-connect-platform-fee-norms.md`'s warning
 * box during #776's own migration (caught by code review, not by the
 * rendered-HTML diff, since the `marked` renderer used for that diff
 * doesn't implement GitHub's Alert extension either). `ALERT_MARKER_RE`
 * is handled the same way a heading is: always its own line.
 */

const FENCE_RE = /^ {0,3}(`{3,}|~{3,})/;
const FENCE_CLOSE_RE = /^ {0,3}(`{3,}|~{3,})\s*$/;
const HEADING_RE = /^ {0,3}#{1,6}(?:\s|$)/;
const THEMATIC_BREAK_RE = /^ {0,3}(?:-\s*){3,}$|^ {0,3}(?:\*\s*){3,}$|^ {0,3}(?:_\s*){3,}$/;
const LINK_REF_DEF_RE = /^\[[^\]]+\]:\s*\S/;
const ALERT_MARKER_RE = /^\[!(?:NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*$/i;
const BLOCKQUOTE_RE = /^ {0,3}>( ?)(.*)$/;
const LIST_ITEM_RE = /^(\s*)([-*+]|\d{1,9}[.)])(\s+)(.*)$/;
const TABLE_DELIMITER_RE = /^\|?\s*:?-+:?\s*(?:\|\s*:?-+:?\s*)*\|?$/;

function isBlank(line: string): boolean {
	return line.trim() === '';
}

function leadingSpaces(line: string): number {
	// A `*` quantifier always matches (zero repetitions is enough), so this
	// exec can never return null -- asserted rather than `?? 0`-guarded, so
	// coverage never has to reach a branch the regex cannot take.
	return /^ */.exec(line)![0].length;
}

function isFenceClose(line: string, fenceChar: string, fenceLength: number): boolean {
	const match = FENCE_CLOSE_RE.exec(line);
	return match !== null && match[1][0] === fenceChar && match[1].length >= fenceLength;
}

function isTableRow(line: string): boolean {
	return line.includes('|');
}

function isTableStart(lines: readonly string[], index: number): boolean {
	const next = lines[index + 1];
	return next !== undefined && isTableRow(lines[index]) && TABLE_DELIMITER_RE.test(next.trim());
}

function nextNonBlank(lines: readonly string[], from: number): number {
	let index = from;
	while (index < lines.length && isBlank(lines[index])) index++;
	return index;
}

// The lines a fence swallows verbatim after its opening line, up to and
// including whichever line closes it (or to the end of the file, if none
// does). Its own single loop, rather than nested inside `unwrapLines`'s,
// so the `break` that ends it is not a nested-loop break.
function scanFence(
	lines: readonly string[],
	startIndex: number,
	fenceChar: string,
	fenceLength: number
): { fenceLines: string[]; nextIndex: number } {
	const fenceLines: string[] = [];
	let index = startIndex;
	while (index < lines.length) {
		fenceLines.push(lines[index]);
		const isClosed = isFenceClose(lines[index], fenceChar, fenceLength);
		index++;
		if (isClosed) break;
	}
	return { fenceLines, nextIndex: index };
}

// A blockquote's own lines, with the `>` marker (and the one space after
// it, if there was one) already stripped off each.
function collectBlockquoteInner(
	lines: readonly string[],
	startIndex: number
): { inner: string[]; nextIndex: number } {
	const inner: string[] = [];
	let index = startIndex;
	while (index < lines.length) {
		const match = BLOCKQUOTE_RE.exec(lines[index]);
		if (!match) break;
		inner.push(match[2]);
		index++;
	}
	return { inner, nextIndex: index };
}

// A list item's own lines after its marker line, dedented by the
// marker's content column -- including a blank line that separates two
// paragraphs of a loose item, but not one that ends the item.
function collectListItemInner(
	lines: readonly string[],
	startIndex: number,
	contentColumn: number
): { inner: string[]; nextIndex: number } {
	const inner: string[] = [];
	let index = startIndex;
	while (index < lines.length) {
		if (isBlank(lines[index])) {
			const lookahead = nextNonBlank(lines, index);
			if (lookahead < lines.length && leadingSpaces(lines[lookahead]) >= contentColumn) {
				inner.push('');
				index++;
				continue;
			}
			break;
		}
		if (leadingSpaces(lines[index]) < contentColumn) break;
		inner.push(lines[index].slice(contentColumn));
		index++;
	}
	return { inner, nextIndex: index };
}

// Every construct a paragraph's own wrap must stop at, whether it is
// meeting that construct fresh or meeting it as a would-be continuation
// line. Table is checked last because it alone needs the next line too.
function isOtherBlockStart(lines: readonly string[], index: number): boolean {
	const line = lines[index];
	return (
		FENCE_RE.test(line) ||
		HEADING_RE.test(line) ||
		THEMATIC_BREAK_RE.test(line) ||
		LINK_REF_DEF_RE.test(line) ||
		ALERT_MARKER_RE.test(line) ||
		BLOCKQUOTE_RE.test(line) ||
		LIST_ITEM_RE.test(line) ||
		isTableStart(lines, index)
	);
}

// One paragraph's worth of source lines, joined into as few output lines
// as a deliberate hard break demands -- one for a paragraph with none,
// one per segment for a paragraph that has them. A hard break is a line
// (other than the paragraph's last) ending in two or more spaces or a
// single trailing backslash; that marker is preserved on the joined line
// it closes, never absorbed as ordinary wrap.
function joinParagraph(lines: readonly string[]): string[] {
	const out: string[] = [];
	let segment: string[] = [];
	for (const [index, raw] of lines.entries()) {
		const isLast = index === lines.length - 1;
		const hasSpaceBreak = !isLast && / {2,}$/.test(raw);
		const hasBackslashBreak = !isLast && !hasSpaceBreak && /(?:^|[^\\])\\$/.test(raw);
		if (hasSpaceBreak) {
			segment.push(raw.trim());
			out.push(`${segment.join(' ')}  `);
			segment = [];
		} else if (hasBackslashBreak) {
			segment.push(raw.replace(/\\$/, '').trim());
			out.push(`${segment.join(' ')}\\`);
			segment = [];
		} else {
			segment.push(raw.trim());
		}
	}
	/* v8 ignore next -- joinParagraph's only caller gathers at least one
	 * line before calling it (the while loop that builds `paragraph` checks
	 * its condition, already known true, before its first iteration), and
	 * the last line always reaches the plain `else` above (isLast rules out
	 * both break branches for it), so segment can never be empty here. */
	if (segment.length > 0) out.push(segment.join(' '));
	return out;
}

// Re-walks CommonMark's own block grammar far enough to tell a
// paragraph's wrap from every other construct, recursing into a
// blockquote's or a list item's own content with the quote marker or the
// item's indent stripped, so a paragraph nested inside either is
// unwrapped the same way a top-level one is.
function unwrapLines(lines: readonly string[]): string[] {
	const out: string[] = [];
	let index = 0;
	while (index < lines.length) {
		const line = lines[index];

		if (isBlank(line)) {
			out.push(line);
			index++;
			continue;
		}

		const fenceMatch = FENCE_RE.exec(line);
		if (fenceMatch) {
			const fenceChar = fenceMatch[1][0];
			const fenceLength = fenceMatch[1].length;
			const { fenceLines, nextIndex } = scanFence(lines, index + 1, fenceChar, fenceLength);
			out.push(line, ...fenceLines);
			index = nextIndex;
			continue;
		}

		if (
			HEADING_RE.test(line) ||
			THEMATIC_BREAK_RE.test(line) ||
			LINK_REF_DEF_RE.test(line) ||
			ALERT_MARKER_RE.test(line)
		) {
			out.push(line);
			index++;
			continue;
		}

		if (isTableStart(lines, index)) {
			out.push(line, lines[index + 1]);
			index += 2;
			while (index < lines.length && !isBlank(lines[index]) && isTableRow(lines[index])) {
				out.push(lines[index]);
				index++;
			}
			continue;
		}

		const quoteMatch = BLOCKQUOTE_RE.exec(line);
		if (quoteMatch) {
			const { inner, nextIndex } = collectBlockquoteInner(lines, index);
			index = nextIndex;
			for (const unwrapped of unwrapLines(inner)) {
				out.push(unwrapped === '' ? '>' : `> ${unwrapped}`);
			}
			continue;
		}

		const itemMatch = LIST_ITEM_RE.exec(line);
		if (itemMatch) {
			const [, indent, marker, spacing, tail] = itemMatch;
			const contentColumn = indent.length + marker.length + spacing.length;
			const { inner: rest, nextIndex } = collectListItemInner(lines, index + 1, contentColumn);
			const inner = [tail, ...rest];
			index = nextIndex;
			// `inner` always starts with the marker's own tail, so it is never
			// empty, and `unwrapLines` maps a non-empty input to a non-empty
			// output (every branch pushes before advancing) -- `unwrapped[0]`
			// needs no `?? ''` fallback coverage cannot reach.
			const unwrapped = unwrapLines(inner);
			out.push(`${indent}${marker}${spacing}${unwrapped[0]}`);
			for (const contLine of unwrapped.slice(1)) {
				out.push(contLine === '' ? '' : ' '.repeat(contentColumn) + contLine);
			}
			continue;
		}

		const paragraph: string[] = [];
		while (index < lines.length && !isBlank(lines[index]) && !isOtherBlockStart(lines, index)) {
			paragraph.push(lines[index]);
			index++;
		}
		out.push(...joinParagraph(paragraph));
	}
	return out;
}

// YAML front matter, recognized only at the very top of the file (as in
// `docs/design/directions/*.DESIGN.md`), passed through untouched -- it
// is data, not prose, and a `---` inside it is not a thematic break.
function splitFrontMatter(lines: readonly string[]): { frontMatter: string[]; body: string[] } {
	if (lines[0] !== '---') return { frontMatter: [], body: [...lines] };
	for (let index = 1; index < lines.length; index++) {
		if (lines[index] === '---') {
			return { frontMatter: lines.slice(0, index + 1), body: lines.slice(index + 1) };
		}
	}
	return { frontMatter: [], body: [...lines] };
}

export function unwrapProse(source: string): string {
	const hasTrailingNewline = source.endsWith('\n');
	const allLines = source.split('\n');
	const lines = hasTrailingNewline ? allLines.slice(0, -1) : allLines;
	const { frontMatter, body } = splitFrontMatter(lines);
	const result = [...frontMatter, ...unwrapLines(body)];
	return result.join('\n') + (hasTrailingNewline ? '\n' : '');
}
