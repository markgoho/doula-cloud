/**
 * A Practice's Plan Template: the field list it defines for its Care Plan
 * or Birth Plan (see docs/adr/0001-practice-defined-plan-templates.md).
 * This module holds the load/save orchestration and the add/remove/
 * reorder/validate logic for the practice-settings screen, decoupled from
 * SvelteKit and the DOM so it can be unit-tested directly.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './apiErrorMessage.js';

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

/** What a person reads for each field type (#1252): the name the kind of
 * answer question, the question page's "Kind of answer" select and the
 * question list's rows show (#2193). The one source of these words for
 * both the Plan Templates and the Client Fields, which share this
 * palette; the stored identifier never reaches the screen. */
export const FIELD_TYPE_NAMES: Readonly<Record<FieldType, { name: string }>> = {
	short_text: { name: 'Short answer' },
	long_text: { name: 'Long answer' },
	single_select: { name: 'Choose one' },
	multi_select: { name: 'Choose several' },
	checkbox: { name: 'Yes or no' },
	section_header: { name: 'Section heading' }
};

/** The field types as a select's options: the stored identifier as the
 * value, its human name as the label. */
export const FIELD_TYPE_OPTIONS: readonly { value: FieldType; label: string }[] = FIELD_TYPES.map((type) => ({
	value: type,
	label: FIELD_TYPE_NAMES[type].name
}));

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
		throw new Error(await apiErrorMessage(response));
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
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Removes the field with id, renumbering Order to match the resulting
 * array position. */
export function removeField(fields: Field[], id: string): Field[] {
	return normalizeOrder(fields.filter((f) => f.id !== id));
}

function normalizeOrder(fields: Field[]): Field[] {
	return fields.map((f, order) => ({ ...f, order }));
}

