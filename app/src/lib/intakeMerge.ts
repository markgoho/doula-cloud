/**
 * What happens when the save-time duplicate check finds the Client
 * already on file (ADR-0017's "This is her").
 *
 * Carried forward from #497 rather than re-decided -- its match copy and
 * match behaviour are settled -- and moved out of the route on #466,
 * because the per-step sequence has two callers for it and neither is a
 * good place to keep the rule.
 *
 * The rule itself: nothing typed on a blank field overwrites what is
 * already on file, and a field typed the same as what is on file is not
 * a change worth showing anyone. Only a non-blank, genuinely different
 * value becomes a proposed edit.
 *
 * What each column is called comes from `intakeJourney.ts`'s own table,
 * the same one the summary's rows read, so a change proposed here is
 * named the way the reader was asked for it.
 */

import type { ClientEditFields, ClientMatch } from './client.js';
import { NOT_ANSWERED } from './intakeAnswers.js';
import type { IntakeAnswers } from './intakeDraft.svelte.js';
import { STRUCTURAL_QUESTIONS } from './intakeJourney.js';

/** One row of "what saving this would change" -- what is on file, and
 * what intake was told instead. */
export interface ProposedChange {
	label: string;
	onFile: string;
	typed: string;
}

function text(value: string): string {
	return value.trim();
}

/** Every structural column intake was told something new about. An
 * empty list means what was typed is already on file, so there is
 * nothing to confirm and nothing to save. */
export function proposedChanges(answers: IntakeAnswers, match: ClientMatch): ProposedChange[] {
	const rows: ProposedChange[] = [];
	for (const { key, label } of STRUCTURAL_QUESTIONS) {
		const typed = text(answers[key]);
		const onFile = text(match[key]);
		if (typed !== '' && typed !== onFile) {
			rows.push({ label, onFile: onFile === '' ? NOT_ANSWERED : onFile, typed });
		}
	}
	return rows;
}

/**
 * The full-object PUT `edit.go` expects.
 *
 * Everything the matched Client already holds rides through unchanged
 * unless intake typed something different -- including `fieldValues`,
 * which an edit that did not round-trip would silently wipe (#495's
 * hazard). Where intake collected a Practice-defined answer of its own,
 * it is layered over what is on file rather than replacing the map, so a
 * field this Practice asks today does not erase one it asked last year.
 */
export function mergedEditFields(answers: IntakeAnswers, match: ClientMatch): ClientEditFields {
	const onFileValues =
		match.fieldValues !== null && typeof match.fieldValues === 'object'
			? (match.fieldValues as Record<string, unknown>)
			: {};
	const merged: ClientEditFields = {
		givenName: text(answers.givenName) || match.givenName,
		familyName: text(answers.familyName) || match.familyName,
		preferredName: text(answers.preferredName) || match.preferredName,
		email: text(answers.email) || match.email,
		phone: text(answers.phone) || match.phone,
		addressLine1: text(answers.addressLine1) || match.addressLine1,
		addressLine2: text(answers.addressLine2) || match.addressLine2,
		addressLocality: text(answers.addressLocality) || match.addressLocality,
		addressRegion: text(answers.addressRegion) || match.addressRegion,
		addressPostalCode: text(answers.addressPostalCode) || match.addressPostalCode,
		dateOfBirth: text(answers.dateOfBirth) || match.dateOfBirth,
		fieldValues: { ...onFileValues, ...answers.fieldValues }
	};
	return merged;
}
