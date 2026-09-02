import { readFileSync } from 'node:fs';

/*
 * Shared by every spec that greps quoted string literals across the app
 * for a banned word or pattern -- first `formErrors.usage.spec.ts` (#467),
 * now `copy.pronoun.usage.spec.ts` (#463). Both ask "what quoted text does
 * this file carry, once its comments are gone" and differ only in what
 * they then look for in that text, so the walk itself belongs in one
 * place rather than two copies that would drift.
 */

/*
 * Strips comments, then returns what is left of each line.
 *
 * Block comments (`<!-- -->` and slash-star) are tracked across lines. A
 * `//` line comment is only recognised where the line *starts* with it,
 * which is how every one in this repo is written, and which is what stops
 * a `https://` inside a string being read as the start of a comment.
 */
export function withoutComments(source: string): string[] {
	const lines = source.split('\n');
	const out: string[] = [];
	let openBlock: '-->' | '*/' | undefined;

	for (const raw of lines) {
		let text = raw;

		if (openBlock) {
			const close = text.indexOf(openBlock);
			if (close === -1) {
				out.push('');
				continue;
			}
			text = text.slice(close + openBlock.length);
			openBlock = undefined;
		}

		if (text.trimStart().startsWith('//')) {
			out.push('');
			continue;
		}

		// Whole comments on one line go first, so the "did one open and stay
		// open" check below only ever sees a genuinely unterminated marker.
		text = text.replaceAll(/<!--.*?-->/gs, '').replaceAll(/\/\*.*?\*\//gs, '');

		const htmlOpen = text.lastIndexOf('<!--');
		const blockOpen = text.lastIndexOf('/*');
		if (htmlOpen !== -1 && htmlOpen > blockOpen) {
			openBlock = '-->';
			text = text.slice(0, htmlOpen);
		} else if (blockOpen !== -1) {
			openBlock = '*/';
			text = text.slice(0, blockOpen);
		}

		out.push(text);
	}
	return out;
}

const QUOTED = /'([^'\\\n]*)'|"([^"\\\n]*)"|`([^`\\]*)`/g;

export interface QuotedLine {
	line: number;
	text: string;
}

/**
 * Every quoted string literal in `source`, comments already stripped, one
 * entry per match. Pure and file-independent, so a test can hand it a
 * literal string rather than a path on disk -- `quotedStrings` below is the
 * thin file-reading wrapper the specs actually call.
 */
export function quotedStringsInSource(source: string): QuotedLine[] {
	const found: QuotedLine[] = [];
	const lines = withoutComments(source);

	for (const [index, line] of lines.entries()) {
		for (const match of line.matchAll(QUOTED)) {
			// One of the three alternatives always matched something, so one
			// of the three groups is always defined -- there is no fourth
			// case for a trailing `?? ''` to cover.
			const literal = (match[1] ?? match[2] ?? match[3])!;
			found.push({ line: index + 1, text: literal });
		}
	}
	return found;
}

/**
 * Every quoted string literal in `file`, comments already stripped, one
 * entry per match.
 */
export function quotedStrings(file: string, appRoot: string): QuotedLine[] {
	return quotedStringsInSource(readFileSync(new URL(file, `file://${appRoot}`), 'utf8'));
}
