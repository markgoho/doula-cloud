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
 * It has happened twice. #1025 and #1026 merged twenty-three minutes
 * apart on 2026-09-08 and both took 0033, because nothing looked. Neither
 * PR could have seen the other: each picked the next free number against
 * trunk, and trunk was right for both of them at the moment they looked.
 * That is why this is a check rather than a line of advice -- the mistake
 * is invisible to the person making it.
 *
 * The second time was this check's own PR. #1184 landed 0037 while #1053
 * was in flight holding 0037 too, and this spec turned #1053's PR red
 * before it could merge -- the first collision caught by anything other
 * than a person noticing months later. The renumber to 0038 cost one
 * rebase instead of the reference sweep the rest of this ticket was.
 *
 * **Where it catches the collision.** On the PR, when the number is
 * already taken on trunk. In the #1025/#1026 shape -- two PRs branched
 * from the same trunk, each adding a number free at the moment it looked
 * -- neither PR holds a duplicate, so both go green and the duplicate
 * first exists on trunk. `Require PR on trunk` sets
 * `strict_required_status_checks_policy: false`, so the second PR is
 * never forced to re-run against the merged first one. Trunk's own run
 * then fails, `.github/workflows/trunk-red.yml` opens the alarm issue,
 * and the repair happens the same day instead of months later in a
 * reader's head. `docs/agents/domain.md` carries the other half: take
 * the number as late as you can, which is what shrinks that window.
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

// An index of the directory is not a decision and carries no number, so it
// is not held to either rule. Nothing else in `docs/adr/` is exempt.
const NOT_AN_ADR = new Set(['README.md']);

interface Collision {
	readonly number: string;
	readonly files: readonly string[];
}

function findCollisions(files: readonly string[]): Collision[] {
	const byNumber = Map.groupBy(files, (file) => ADR_FILENAME.exec(file)?.[1]);
	const collisions: Collision[] = [];
	for (const [number, sharing] of byNumber) {
		// A file with no number at all fails the filename rule below rather
		// than being reported here as everything colliding with everything.
		if (number !== undefined && sharing.length > 1) collisions.push({ number, files: sharing });
	}
	return collisions;
}

function describeCollisions(collisions: readonly Collision[]): string {
	return collisions.map(({ number, files }) => `ADR-${number}: ${files.join(', ')}`).join('; ');
}

const adrFiles = globSync('*.md', { cwd: adrRoot })
	.filter((file) => !NOT_AN_ADR.has(file))
	.toSorted((a, b) => a.localeCompare(b));

describe('findCollisions', () => {
	it('reports the number two ADRs share, and both files that share it', () => {
		expect(
			findCollisions([
				'0032-billing-is-credits-payments-is-getting-paid.md',
				'0033-overdue-is-derived-and-notifies-nobody.md',
				'0033-staff-login-deletion-is-immediate-and-redacts-the-person.md'
			])
		).toEqual([
			{
				number: '0033',
				files: [
					'0033-overdue-is-derived-and-notifies-nobody.md',
					'0033-staff-login-deletion-is-immediate-and-redacts-the-person.md'
				]
			}
		]);
	});

	it('reports nothing when every number is its own', () => {
		expect(
			findCollisions([
				'0033-staff-login-deletion-is-immediate-and-redacts-the-person.md',
				'0038-overdue-is-derived-and-notifies-nobody.md'
			])
		).toEqual([]);
	});

	it('leaves a file carrying no number to the filename rule', () => {
		expect(findCollisions(['no-number-at-all.md', 'nor-here.md'])).toEqual([]);
	});
});

describe('ADR numbering', () => {
	it('finds the ADRs at all, so an empty sweep cannot pass silently', () => {
		expect(adrFiles.length).toBeGreaterThan(0);
	});

	it.each(adrFiles)('%s is a four-digit number and a kebab-case slug', (file) => {
		expect(file).toMatch(ADR_FILENAME);
	});

	it('gives every ADR a number no other ADR has', () => {
		const collisions = findCollisions(adrFiles);

		expect(
			describeCollisions(collisions),
			'Two ADRs cannot share a number -- it would name two decisions, and every citation of it becomes a guess. Give the later one the next free number and repoint every citation of it (see #1053 for what that involves).'
		).toBe('');
	});
});
