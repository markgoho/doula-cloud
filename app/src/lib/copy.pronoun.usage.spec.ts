import { globSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { quotedStrings } from './quotedCopy';

/*
 * The mechanical half of #463's writing rule (`docs/design/brief.md`,
 * "Voice"): product copy names a Client, a Staff member or anyone else by
 * the domain noun or their own first name, never by a gendered pronoun.
 *
 * ## What it reads
 *
 * Every quoted string in every component and route, using the same
 * comment-stripped walk as `formErrors.usage.spec.ts` (`quotedCopy.ts`),
 * so a pronoun named only inside a doc comment -- this file's own rationale
 * included -- is not an offence. That is deliberate, not a gap: a comment
 * is this repo's documented voice (CLAUDE.md), a separate register from
 * what a screen says out loud, and #463 draws that line explicitly.
 *
 * `routes/style-guide/**` is excluded. Auditing it turned up thirteen
 * pre-existing hits, all in component-catalogue example props rather than
 * a real screen a Practice ever sees -- tracked on #464 rather than fixed
 * here, which would have widened this ticket well past a copy fix.
 * Widen this glob to cover it once #464 lands.
 *
 * ## What it deliberately does not catch
 *
 * Two limits, named rather than hidden:
 *
 * 1. **Singular "they/them" is not checked.** The brief's rule forbids
 *    every pronoun, not only a gendered one, but "they" is indistinguishable
 *    from the ordinary plural by a word-boundary regex -- flagging it would
 *    drown every legitimate plural in the app in false positives. A reviewer
 *    has to read for this one; nothing here can.
 * 2. **A quoted third party is not exempted.** A Client's own words --
 *    "her partner and her mother" as something a Client wrote about her own
 *    household -- would trip this the same as house copy about a Client
 *    nobody has met. No occurrence of that shape exists in the covered glob
 *    today, so the exemption is not built until a real one forces the
 *    question.
 */

const PRONOUNS = ['she', 'her', 'hers', 'herself', 'he', 'him', 'his', 'himself'];

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const sourceFiles = globSync('src/{lib/components,routes}/**/*.svelte', { cwd: appRoot }).filter(
	(file) => !file.startsWith('src/routes/style-guide/')
);

interface Offence {
	file: string;
	line: number;
	found: string;
	text: string;
}

function findOffences(file: string): Offence[] {
	const found: Offence[] = [];

	for (const { line, text: literal } of quotedStrings(file, appRoot)) {
		for (const word of PRONOUNS) {
			if (new RegExp(String.raw`\b` + word + String.raw`\b`, 'i').test(literal)) {
				found.push({ file, line, found: word, text: literal });
			}
		}
	}
	return found;
}

describe('product copy names a Client or Staff member, never a pronoun', () => {
	it('reads every component and route outside the style guide', () => {
		// A glob that silently matched nothing would make every assertion
		// below pass while checking no copy at all.
		expect(sourceFiles.length).toBeGreaterThan(50);
	});

	it('uses no gendered pronoun in a quoted string', () => {
		const offences = sourceFiles.flatMap((file) => findOffences(file));

		expect(offences.map((offence) => `${offence.file}:${offence.line} "${offence.text}"`)).toEqual(
			[]
		);
	});
});
