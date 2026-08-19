/**
 * A Practice's Plan Template: the field list it defines for its Care Plan
 * or Birth Plan (see docs/adr/0001-practice-defined-plan-templates.md).
 * This module holds the load/save orchestration and the add/remove/
 * reorder/validate logic for the practice-settings screen, decoupled from
 * SvelteKit and the DOM so it can be unit-tested directly.
 */

/** The field-type palette from ADR-0001 -- the only kinds of field a Plan
 * Template may contain. Values match the Go BFF's `plan_templates.fields`
 * JSONB shape exactly (api/internal/plans/template.go); there is no
 * camelCase translation layer for enum-like values, only for field names. */
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

/** Reports whether type requires an Options list -- single/multi-select
 * only. Exported so the settings UI can conditionally render an options
 * editor for the field being edited. */
export function isSelectType(type: FieldType): boolean {
	return SELECT_FIELD_TYPES.has(type);
}

export interface Field {
	id: string;
	type: FieldType;
	label: string;
	options?: string[];
	order: number;
}

export interface Template {
	planType: string;
	fields: Field[];
}

/** A minimal fetch-shaped function, injected rather than imported, so
 * load/save can be unit-tested without mocking the global fetch or
 * SvelteKit's `$app` modules -- the route wires this to `#lib/api.js`'s
 * `apiFetchWithSession`. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

function templatePath(practiceId: string, planType: string): string {
	return `/api/practices/${practiceId}/plan-templates/${planType}`;
}

/** Loads a Practice's Plan Template for planType. Throws with the
 * response body text on a non-2xx response, mirroring the error-surfacing
 * convention used by invite/+page.svelte and clients/new/+page.svelte. */
export async function loadTemplate(
	fetcher: Fetcher,
	practiceId: string,
	planType: string
): Promise<Template> {
	const response = await fetcher(templatePath(practiceId, planType));
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Replaces a Practice's Plan Template for planType with fields, in the
 * array order given -- Order is recomputed server-side from that
 * position, so callers never need to set it themselves before saving. */
export async function saveTemplate(
	fetcher: Fetcher,
	practiceId: string,
	planType: string,
	fields: Field[]
): Promise<Template> {
	const response = await fetcher(templatePath(practiceId, planType), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ planType, fields })
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Appends a new blank field of type, returning a new array (fields is
 * never mutated) with Order left as array position -- normalizeOrder, or
 * the server on save, keeps Order in sync after removeField/moveField. */
export function addField(fields: Field[], id: string, type: FieldType): Field[] {
	const field: Field = { id, type, label: '', order: fields.length };
	if (isSelectType(type)) {
		field.options = [];
	}
	return [...fields, field];
}

/** Removes the field with id, renumbering Order to match the resulting
 * array position. */
export function removeField(fields: Field[], id: string): Field[] {
	return normalizeOrder(fields.filter((f) => f.id !== id));
}

/** Moves the field with id one position toward the front ('up') or back
 * ('down') of the array -- the plain, non-drag-and-drop reorder control
 * ADR-0001 calls for. A move past either end is a no-op, not an error,
 * since the UI disables the control at the boundary. */
export function moveField(fields: Field[], id: string, direction: 'up' | 'down'): Field[] {
	const index = fields.findIndex((f) => f.id === id);
	const target = direction === 'up' ? index - 1 : index + 1;
	if (index === -1 || target < 0 || target >= fields.length) {
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

/** Validates fields against the same rules the Go BFF enforces
 * server-side (api/internal/plans/template.go's normalizeFields) --
 * mirrored here purely for early UX feedback before a round trip; the
 * server remains the authority. Returns an error message, or undefined if
 * fields is valid. */
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
