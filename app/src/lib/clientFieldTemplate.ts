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

export const FIELD_TYPES = [
	'short_text',
	'long_text',
	'single_select',
	'multi_select',
	'checkbox',
	'section_header'
] as const;

export type FieldType = (typeof FIELD_TYPES)[number];

const SELECT_FIELD_TYPES: ReadonlySet<FieldType> = new Set(['single_select', 'multi_select']);

export function isSelectType(type: FieldType): boolean {
	return SELECT_FIELD_TYPES.has(type);
}

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

/** A minimal fetch-shaped function, injected rather than imported --
 * the same seam planTemplate.ts's Fetcher uses. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

function templatePath(practiceId: string): string {
	return `/api/practices/${practiceId}/client-field-template`;
}

/** Loads a Practice's Client Field Template. Throws with the response
 * body text on a non-2xx response. Never 404s -- a new Practice's
 * template is empty by default, not missing. */
export async function loadTemplate(fetcher: Fetcher, practiceId: string): Promise<Template> {
	const response = await fetcher(templatePath(practiceId));
	if (!response.ok) {
		throw new Error(await response.text());
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
		throw new Error(await response.text());
	}
	return response.json();
}

/**
 * Appends a new blank, unarchived field of type.
 */
export function addField(fields: Field[], id: string, type: FieldType): Field[] {
	const field: Field = { id, type, label: '', order: fields.length, archived: false };
	if (isSelectType(type)) {
		field.options = [];
	}
	return [...fields, field];
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

/** Moves the field with id one position toward the front ('up') or back
 * ('down') of the array, skipping past any field whose archived state
 * differs from the moved field's. An active field reorders only among
 * active fields -- which is how it crosses from one Practice-named
 * section into another, since a section is just the run of active fields
 * between two section_header entries -- and an archived field reorders
 * only among the "Archived fields" list. A move past either end, or past
 * the last field sharing its state, is a no-op. */
export function moveField(fields: Field[], id: string, direction: 'up' | 'down'): Field[] {
	const index = fields.findIndex((f) => f.id === id);
	if (index === -1) {
		return fields;
	}
	const archived = fields[index].archived;
	const step = direction === 'up' ? -1 : 1;
	let target = index + step;
	while (target >= 0 && target < fields.length && fields[target].archived !== archived) {
		target += step;
	}
	if (target < 0 || target >= fields.length) {
		return fields;
	}
	const reordered = [...fields];
	const moved = reordered[index];
	reordered[index] = reordered[target];
	reordered[target] = moved;
	return normalizeOrder(reordered);
}

function normalizeOrder(fields: Field[]): Field[] {
	return fields.map((f, order) => ({ ...f, order }));
}

/** The trimmed, lowercased labels validateFields blocks as restating one
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

/** ADR-0017 puts no cap on the field list, and the intake form's Tesler
 * ruling (#432) means no field is ever deleted just to shorten it -- so
 * past this point an Owner is told the cost instead of being blocked.
 * Counts only what a Client is actually asked: archived fields are no
 * longer collected, and a section_header is a heading, not a question. */
export const FIELD_COUNT_WARNING_THRESHOLD = 20;

/** Returns a warning message once the template asks more than
 * FIELD_COUNT_WARNING_THRESHOLD questions, or undefined below that. */
export function fieldCountWarning(fields: Field[]): string | undefined {
	const askedCount = fields.filter((f) => !f.archived && f.type !== 'section_header').length;
	if (askedCount <= FIELD_COUNT_WARNING_THRESHOLD) {
		return undefined;
	}
	return `This template asks a Client ${askedCount} questions beyond the standard ones. Every field here adds to how long intake takes -- consider archiving any that are no longer worth asking before adding more.`;
}

/** Validates fields against the same rules the Go BFF enforces
 * server-side, mirrored here purely for early feedback before a round
 * trip. Returns an error message, or undefined if fields is valid. */
export function validateFields(fields: Field[]): string | undefined {
	const seenIDs = new Set<string>();
	for (const f of fields) {
		if (f.id === '') {
			return 'field id is required';
		}
		if (seenIDs.has(f.id)) {
			return `duplicate field id: ${f.id}`;
		}
		seenIDs.add(f.id);

		if (f.label === '') {
			return 'field label is required';
		}
		const structural = STRUCTURAL_FIELD_NAMES.get(f.label.trim().toLowerCase());
		if (structural) {
			return `"${f.label}" is already on every Client record as the structural ${structural} field -- edit that field instead of adding a duplicate.`;
		}

		if (isSelectType(f.type)) {
			if (!f.options || f.options.length === 0) {
				return `field ${f.id} requires at least one option`;
			}
			if (f.options.includes('')) {
				return `field ${f.id} has a blank option`;
			}
		} else if (f.options && f.options.length > 0) {
			return `field ${f.id} of type ${f.type} may not have options`;
		}
	}
	return undefined;
}
