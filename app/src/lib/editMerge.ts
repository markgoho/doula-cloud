/**
 * What "This is her" proposes to change, on the edit path (ADR-0017's
 * amendment, #814).
 *
 * Restates `intakeMerge.ts`'s rule for two full Records instead of
 * intake's answers-against-a-match shape, and makes it direction-aware:
 * unlike intake's save-time prompt, where the matched record always
 * survives, the edit path's `CollisionMatch.wouldSurvive` can go either
 * way -- an unattached record being edited can itself be the survivor
 * when the match it collided with is older. This must mirror
 * `api/internal/client/merge.go`'s `fold` exactly, in both directions: a
 * non-blank value from the absorbed side wins, a blank never overwrites.
 */

import type { ClientEditFields, CollisionMatch } from './client.js';
import { NOT_ANSWERED } from './intakeAnswers.js';
import { STRUCTURAL_QUESTIONS } from './intakeJourney.js';

/** One row of "what saving this would change" -- what is on file, and
 * what this side contributes instead. Same shape as intakeMerge.ts's
 * ProposedChange. */
export interface ProposedChange {
	label: string;
	onFile: string;
	typed: string;
}

function text(value: string): string {
	return value.trim();
}

/**
 * Every structural column a "This is her" answer on `match` would
 * change, direction-aware.
 *
 * When `match.wouldSurvive` is true, the match is `fold`'s survivor and
 * `fields` (the freshly typed values, not yet saved) is the absorbed
 * side: a row appears where a typed value is non-blank and differs from
 * what the match already holds. When `match.wouldSurvive` is false, the
 * record being edited survives and the match is absorbed into it
 * instead: a row appears where the match's own value is non-blank and
 * differs from what was typed, since that is the value `fold` would
 * write over it.
 *
 * An empty list means "This is her" would write nothing new -- what was
 * typed already agrees with the side that survives.
 */
export function proposedMergeChanges(fields: ClientEditFields, match: CollisionMatch): ProposedChange[] {
	const survivorSide = match.wouldSurvive ? match : fields;
	const absorbedSide = match.wouldSurvive ? fields : match;

	const rows: ProposedChange[] = [];
	for (const { key, label } of STRUCTURAL_QUESTIONS) {
		const onFile = text(survivorSide[key]);
		const typed = text(absorbedSide[key]);
		if (typed !== '' && typed !== onFile) {
			rows.push({ label, onFile: onFile === '' ? NOT_ANSWERED : onFile, typed });
		}
	}
	return rows;
}
