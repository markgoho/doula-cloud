/**
 * What a "This is her" answer would change (ADR-0017, and its amendment
 * on #814), written once for intake's save-time prompt and the edit
 * path's.
 *
 * The rule: a non-blank value from the absorbed side that differs from
 * what the surviving side holds becomes a proposed change, and a blank
 * never overwrites. This must mirror `api/internal/client/merge.go`'s
 * `fold` exactly -- `survivor` and `absorbed` here are its two
 * arguments.
 *
 * What each column is called comes from `intakeJourney.ts`'s own table,
 * the same one the summary's rows read, so a change proposed here is
 * named the way the reader was asked for it.
 */

import type { ClientEditFields, CollisionMatch } from './client.js';
import { NOT_ANSWERED } from './intakeAnswers.js';
import { STRUCTURAL_QUESTIONS, type TextColumn } from './intakeJourney.js';

/** One row of "what saving this would change" -- what the surviving
 * side holds, and what the absorbed side contributes instead. */
export interface ProposedChange {
	label: string;
	onFile: string;
	typed: string;
}

/** One side of a merge: the structural columns, whichever shape carries
 * them -- intake's answers, an edit form's fields, or a match. */
type Side = Readonly<Record<TextColumn, string>>;

/**
 * Every structural column `absorbed` would change on `survivor`. An
 * empty list means the absorbed side already agrees with the side that
 * survives, so there is nothing to confirm.
 *
 * Intake's save-time prompt calls this directly: there the match always
 * survives and the typed answers are absorbed into it.
 */
export function proposedChanges(survivor: Side, absorbed: Side): ProposedChange[] {
	const rows: ProposedChange[] = [];
	for (const { key, label } of STRUCTURAL_QUESTIONS) {
		const onFile = survivor[key].trim();
		const typed = absorbed[key].trim();
		if (typed !== '' && typed !== onFile) {
			rows.push({ label, onFile: onFile === '' ? NOT_ANSWERED : onFile, typed });
		}
	}
	return rows;
}

/**
 * `proposedChanges` for the edit path, where the survivor can be either
 * side: an unattached record being edited can itself survive when the
 * match it collided with is older. `match.wouldSurvive` is decided
 * server-side; when it is false the match is absorbed into `fields`
 * (the freshly typed values, not yet saved) instead of the reverse.
 */
export function proposedMergeChanges(
	fields: ClientEditFields,
	match: CollisionMatch
): ProposedChange[] {
	return match.wouldSurvive ? proposedChanges(match, fields) : proposedChanges(fields, match);
}
