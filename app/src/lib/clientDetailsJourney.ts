/**
 * A Client's other details, added from her record (#1610, ADR-0017's
 * amendment of 2026-10-02).
 *
 * Intake asks for the name; the date of birth, the email address, the
 * phone number, the address and the Practice's own sections are added
 * afterward, as one journey of question pages that saves as one edit
 * through `api/internal/client/edit.go`'s full-record `PUT`. The rules
 * that decide what that one edit sends are here, pure, so a spec can
 * read them without a page in front of them.
 *
 * ## A blank never replaces a value on file
 *
 * The journey adds facts. Taking one away is a correction, and the Edit
 * form is the path for a correction. So a question left blank keeps what
 * the record holds, and a fact the journey never asks -- the three name
 * columns, a value under a field the Practice has archived -- is always
 * the one on file, whatever a draft mirrored from an earlier visit says.
 */

import type { ClientEditFields } from './client.js';
import type { ClientDetail, ClientRecord } from './clientDetail.js';
import { displayName } from './clientDetail.js';
import type { FieldValue, IntakeAnswers } from './intakeDraft.svelte.js';
import { DETAILS_STEPS, type TextColumn } from './intakeJourney.js';

/**
The columns the journey asks, in the order it asks them.
*/
const ASKED_COLUMNS: readonly TextColumn[] = DETAILS_STEPS.flatMap((step) =>
	step.questions.map((question) => question.key)
);

/** The four parts of an address a person has. The second line is the
 * one that may not exist, which is why it is not here. */
const ADDRESS_PARTS = ['addressLine1', 'addressLocality', 'addressRegion', 'addressPostalCode'] as const;

const SECTION_HEADER = 'section_header';

function storedValues(record: ClientRecord): Record<string, FieldValue> {
	const values = record.fieldValues;
	if (values === null || typeof values !== 'object') return {};
	return { ...(values as Record<string, FieldValue>) };
}

/** Whether an answer says nothing. A box left unticked says no, which is
 * an answer, so only an empty string or an empty choice is blank. */
function isBlank(value: unknown): boolean {
	if (value === undefined || value === null) return true;
	if (typeof value === 'string') return value.trim() === '';
	if (Array.isArray(value)) return value.length === 0;
	return false;
}

/**
The record on file, as the answers each question page starts from.
*/
export function answersOnFile(record: ClientRecord): IntakeAnswers {
	return {
		givenName: record.givenName,
		familyName: record.familyName,
		preferredName: record.preferredName,
		email: record.email,
		phone: record.phone,
		addressLine1: record.addressLine1,
		addressLine2: record.addressLine2,
		addressLocality: record.addressLocality,
		addressRegion: record.addressRegion,
		addressPostalCode: record.addressPostalCode,
		dateOfBirth: record.dateOfBirth,
		fieldValues: storedValues(record)
	};
}

/**
 * The one edit the check page sends: the record on file, with each
 * answer the journey was given laid over it. `askedFieldIds` are the
 * Practice's active questions, the only Practice-defined values the
 * journey may write.
 */
export function detailsToSave(
	record: ClientRecord,
	answers: IntakeAnswers,
	askedFieldIds: readonly string[]
): IntakeAnswers & ClientEditFields {
	const saved = answersOnFile(record);
	for (const column of ASKED_COLUMNS) {
		if (!isBlank(answers[column])) saved[column] = answers[column];
	}
	for (const fieldId of askedFieldIds) {
		const value = answers.fieldValues[fieldId];
		if (value !== undefined && !isBlank(value)) saved.fieldValues[fieldId] = value;
	}
	return saved;
}

/**
 * Whether every question the journey asks has an answer on file: the
 * date of birth, the email address, the phone number, an address, and
 * each question the Practice still asks. What decides whether the link
 * says "Add" or "Change".
 */
export function hasEveryDetail(record: ClientDetail): boolean {
	const structural = [record.dateOfBirth, record.email, record.phone, ...ADDRESS_PARTS.map((part) => record[part])];
	if (structural.some((value) => isBlank(value))) return false;
	return record.resolvedFields
		.filter((field) => field.type !== SECTION_HEADER && field.note === undefined)
		.every((field) => !isBlank(field.value));
}

/** The words of the link that opens the journey, on her record and on
 * her Engagement's page. It names her, so it reads on its own. */
export function detailsLinkLabel(record: ClientDetail): string {
	const verb = hasEveryDetail(record) ? 'Change' : 'Add';
	return `${verb} ${displayName(record)}'s details`;
}

/** The query a link on an Engagement's page carries, so the journey
 * returns there (#1610). */
export const ENGAGEMENT_PARAMETER = 'engagement';

/**
An id, and nothing that could make a path of its own.
*/
const ID_PATTERN = /^[\w-]+$/;

/** The Engagement whose page opened the journey, read off the query
 * string. Only an id is accepted, so the URL can name an Engagement but
 * can never name an address for Back to go to. */
export function engagementOrigin(search: Pick<URLSearchParams, 'get'>): string | undefined {
	const value = search.get(ENGAGEMENT_PARAMETER) ?? '';
	return ID_PATTERN.test(value) ? value : undefined;
}

/** Whether the journey can be offered at all. An erased record has no
 * key to seal an edit under (ADR-0027), and a merged one is a tombstone
 * (ADR-0040); `edit.go` refuses both, so neither shows the link. */
export function canAddDetails(record: ClientDetail): boolean {
	return record.erasedAt === undefined && record.mergedInto === undefined;
}
