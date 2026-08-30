import { globSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * The wording half of #467.
 *
 * GOV.UK's error-message rules are four words long and absolute: no
 * "please", no "valid", no "invalid", no "required". They exist because
 * each of those words fails the same test -- say what went wrong *and
 * what to do about it*. "Email is required" names a rule; "Enter your
 * email address" names the next action. "Enter a valid email address"
 * tells somebody her answer is wrong and nothing about what a right one
 * looks like.
 *
 * A rule written in a document is a rule until the first hurry. This is
 * the same shape as `tokens.usage.spec.ts` and `motion.spec.ts`: it runs
 * in the unit suite, which `scripts/hooks/pre-commit` runs in full, so a
 * banned word fails a commit rather than reaching a person.
 *
 * ## What it reads, and what it deliberately does not
 *
 * Every quoted string in every component and route -- which is where
 * user-facing words live. Comments are stripped first, because this
 * repo's rationale is written in prose and prose has to be able to name
 * the words it bans (this file being the extreme case).
 *
 * `formErrors.ts` is *not* read here: it holds Identity Platform's own
 * error codes, and `auth/invalid-credential` is an identifier we do not
 * own. Its messages are checked by `formErrors.spec.ts` instead, one
 * assertion per returned message, which is stricter than a grep.
 *
 * Server prose is out of reach of both. A 4xx body is written by the BFF
 * and shown as-is, because it is the only thing that knows why it
 * refused -- see the resolution comment on #467 for the API-side gap.
 */

const BANNED = ['please', 'valid', 'invalid', 'required'];

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const sourceFiles = globSync('src/{lib/components,routes}/**/*.svelte', { cwd: appRoot });

interface Offence {
	file: string;
	line: number;
	found: string;
	text: string;
}

/*
 * Strips comments, then returns what is left of each line.
 *
 * Block comments (`<!-- -->` and slash-star) are tracked across lines. A
 * `//` line comment is only recognised where the line *starts* with it,
 * which is how every one in this repo is written, and which is what stops
 * a `https://` inside a string being read as the start of a comment.
 */
function withoutComments(source: string): string[] {
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

function findOffences(file: string): Offence[] {
	const found: Offence[] = [];
	const lines = withoutComments(readFileSync(new URL(file, `file://${appRoot}`), 'utf8'));

	for (const [index, line] of lines.entries()) {
		for (const match of line.matchAll(QUOTED)) {
			const literal = match[1] ?? match[2] ?? match[3] ?? '';
			for (const word of BANNED) {
				if (new RegExp(String.raw`\b` + word + String.raw`\b`, 'i').test(literal)) {
					found.push({ file, line: index + 1, found: word, text: literal });
				}
			}
		}
	}
	return found;
}

describe('GOV.UK error wording', () => {
	it('reads every component and route', () => {
		// A glob that silently matched nothing would make every assertion
		// below pass while checking no words at all.
		expect(sourceFiles.length).toBeGreaterThan(50);
	});

	for (const word of BANNED) {
		it(`no user-facing string says "${word}"`, () => {
			const offences = sourceFiles
				.flatMap((file) => findOffences(file))
				.filter((offence) => offence.found === word);

			expect(
				offences.map((offence) => `${offence.file}:${offence.line} "${offence.text}"`)
			).toEqual([]);
		});
	}
});
