/**
 * A Practice's Client Field Template: the extra facts an Owner or Admin
 * defines that a Client record holds beyond ADR-0017's twelve structural
 * columns (docs/adr/0017-twelve-columns-a-practice-defined-layer-and-an-
 * engagement-that-is-asked-for.md). Sibling of planTemplate.ts, with one
 * deliberate difference: removing a field here always archives it -- see
 * archiveField/unarchiveField -- because a Client's values are read live
 * against this template, never snapshotted, so a field can never
 * disappear from the array once created.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './apiErrorMessage.js';
import type { FieldType } from './planTemplate.js';

/* The same six field types as a Plan Template (GLOSSARY.md, "Client
 * Field Template"), so one palette and one set of human names (#1252),
 * not a copy of each. */
export { FIELD_TYPES, FIELD_TYPE_NAMES, FIELD_TYPE_OPTIONS, isSelectType, type FieldType } from './planTemplate.js';

/** A section_header is a heading a Client reads, never a question the
 * Client answers -- fieldCountWarning excludes it from the
 * asked-question count on that basis. */
const SECTION_HEADER_TYPE: FieldType = 'section_header';

export interface Field {
	id: string;
	type: FieldType;
	label: string;
	options?: string[];
	order: number;
	archived: boolean;
}

export interface Template {
	fields: Field[];
}

function templatePath(practiceId: string): string {
	return `/api/practices/${practiceId}/client-field-template`;
}

/** Loads a Practice's Client Field Template. Throws with the response
 * body text on a non-2xx response. Never 404s -- a new Practice's
 * template is empty by default, not missing. */
export async function loadTemplate(fetcher: Fetcher, practiceId: string): Promise<Template> {
	const response = await fetcher(templatePath(practiceId));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Replaces a Practice's Client Field Template with fields, in the array
 * order given -- Order is recomputed server-side from that position. */
export async function saveTemplate(
	fetcher: Fetcher,
	practiceId: string,
	fields: Field[]
): Promise<Template> {
	const response = await fetcher(templatePath(practiceId), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ fields })
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Sets archived=true on the field with id -- ADR-0017's "removing a
 * field archives it, never deletes it": a Client who already holds a
 * value under this id keeps it, and it stays live-read on her record. */
export function archiveField(fields: Field[], id: string): Field[] {
	return fields.map((f) => (f.id === id ? { ...f, archived: true } : f));
}

/**
 * Sets archived=false, restoring the field to the form.
 */
export function unarchiveField(fields: Field[], id: string): Field[] {
	return fields.map((f) => (f.id === id ? { ...f, archived: false } : f));
}

/** The trimmed, lowercased labels structuralFieldRefusal refuses as restating one
 * of ADR-0017's twelve structural columns, mapped to the structural fact's
 * own name -- mirrors the Go BFF's structuralFieldNames
 * (api/internal/clientfieldtemplate/validate.go) for early UX feedback and
 * for the same wording ("a Practice asking for a middle name is not
 * asking for a custom field; it is telling us the structural name has the
 * wrong shape"); the server remains the authority. */
const STRUCTURAL_FIELD_NAMES: ReadonlyMap<string, string> = new Map([
	['given name', 'given name'],
	['first name', 'given name'],
	['family name', 'family name'],
	['last name', 'family name'],
	['surname', 'family name'],
	['preferred name', 'preferred name'],
	['name', 'name'],
	['email', 'email'],
	['email address', 'email'],
	['phone', 'phone'],
	['phone number', 'phone'],
	['telephone', 'phone'],
	['address', 'address'],
	['street address', 'address'],
	['address line 1', 'address'],
	['address line 2', 'address'],
	['city', 'address'],
	['locality', 'address'],
	['state', 'address'],
	['region', 'address'],
	['zip', 'address'],
	['zip code', 'address'],
	['postal code', 'address'],
	['date of birth', 'date of birth'],
	['dob', 'date of birth'],
	['birth date', 'date of birth'],
	['birthday', 'date of birth']
]);

/** The refusal of a label that restates one of the structural facts, or
 * undefined for any other label, which a Client field's own question page
 * asks before its save (#2193). */
export function structuralFieldRefusal(label: string): string | undefined {
	const structural = STRUCTURAL_FIELD_NAMES.get(label.trim().toLowerCase());
	return structural
		? `"${label}" is already on every Client record as the ${structural} field, so it is not a field to add here.`
		: undefined;
}

/** ADR-0017 puts no cap on the field list, and the intake form's Tesler
 * ruling (#432) means no field is ever deleted just to shorten it -- so
 * past this point an Owner is told the cost instead of being blocked.
 * Counts only what a Client is actually asked: archived fields are no
 * longer collected, and a section_header is a heading, not a question. */
export const FIELD_COUNT_WARNING_THRESHOLD = 20;

/** Returns a warning message once the template asks more than
 * FIELD_COUNT_WARNING_THRESHOLD questions, or undefined below that. */
export function fieldCountWarning(fields: Field[]): string | undefined {
	const askedCount = fields.filter((f) => !f.archived && f.type !== SECTION_HEADER_TYPE).length;
	if (askedCount <= FIELD_COUNT_WARNING_THRESHOLD) {
		return undefined;
	}
	return `This template asks a Client ${askedCount} questions beyond the standard ones. Each one makes intake longer, so archive any you no longer need before adding more.`;
}

