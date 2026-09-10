import { globSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * #1053's fifth AC as a gate, in the mold of `spelling.usage.spec.ts` and
 * `roles.usage.spec.ts`: an ADR is cited by its number -- "ADR-0013",
 * "ADR-0026" -- across `CLAUDE.md`, `docs/`, code comments and issue
 * bodies, so a number that names two decisions makes every one of those
 * citations ambiguous.
 *
 * It has happened once. #1025 and #1026 merged twenty-three minutes apart
 * on 2026-09-08 and both took 0033, because nothing looked. Neither PR
 * could have seen the other: each picked the next free number against
 * trunk, and trunk was right for both of them at the moment they looked.
 * That is why this is a check rather than a line of advice -- the mistake
 * is invisible to the person making it and only exists after the merge.
 *
 * The check lives in `app/` because `.github/workflows/ci.yml` puts no
 * `paths:` filter on the `app` job, so it runs on a docs-only PR. That is
 * the only kind of PR that can cause this, and a gate that skipped it
 * would gate nothing.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));
const adrRoot = path.join(appRoot, '..', 'docs', 'adr');

// A four-digit number, then a kebab-case slug. The number is captured; it
// is the whole identity of the document as far as a citation is concerned.
const ADR_FILENAME = /^(\d{4})-[a-z0-9]+(?:-[a-z0-9]+)*\.md$/;

const adrFiles = globSync('*.md', { cwd: adrRoot }).toSorted((a, b) => a.localeCompare(b));

describe('ADR numbering', () => {
	it('finds the ADRs at all, so an empty sweep cannot pass silently', () => {
		expect(adrFiles.length).toBeGreaterThan(0);
	});

	it.each(adrFiles)('%s is a four-digit number and a kebab-case slug', (file) => {
		expect(file).toMatch(ADR_FILENAME);
	});

	it('gives every ADR a number no other ADR has', () => {
		const byNumber = new Map<string, string[]>();
		for (const file of adrFiles) {
			const number = ADR_FILENAME.exec(file)?.[1];
			if (number === undefined) continue;
			byNumber.set(number, [...(byNumber.get(number) ?? []), file]);
		}

		const collisions: string[] = [];
		for (const [number, files] of byNumber) {
			if (files.length > 1) collisions.push(`${number}: ${files.join(', ')}`);
		}

		expect(
			collisions,
			`Two ADRs cannot share a number -- "ADR-${collisions[0]?.slice(0, 4) ?? '0000'}" would name two decisions. Give the later one the next free number and repoint every citation of it (see #1053 for what that involves).`
		).toEqual([]);
	});
});
