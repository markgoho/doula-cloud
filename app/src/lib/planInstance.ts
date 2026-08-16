/**
 * A Client's filled-out Care Plan or Birth Plan for an Engagement (see
 * docs/adr/0001-practice-defined-plan-templates.md). This module holds the
 * load/create/save orchestration for the Engagement view's Care Plan and
 * Birth Plan sections, decoupled from SvelteKit and the DOM so it can be
 * unit-tested directly -- the same shape as planTemplate.ts.
 */
import type { Field, FieldType } from './planTemplate.js';
import { isSelectType } from './planTemplate.js';

export type { Field, FieldType };
export { isSelectType };

/** A Plan Instance's filled-in values, keyed by field id. Value shape
 * depends on the field's type: string for short_text/long_text/
 * single_select, string[] for multi_select, boolean for checkbox. A
 * section_header field never has an answer. Matches the Go BFF's
 * `plans.Answers` shape exactly (api/internal/plans/instance.go). */
export type Answers = Record<string, unknown>;

export interface Instance {
	engagementId: string;
	planType: string;
	fields: Field[];
	answers: Answers;
}

/** A minimal fetch-shaped function, injected rather than imported -- see
 * planTemplate.ts's Fetcher for why. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

function instancePath(practiceId: string, engagementId: string, planType: string): string {
	return `/api/practices/${practiceId}/engagements/${engagementId}/plans/${planType}`;
}

/** Loads the Plan Instance for engagementId + planType, or null if none
 * has been created yet (a 404 from GetInstanceHandler) -- callers use that
 * to show a "Create" control instead of the fill-out form. Throws with the
 * response body text on any other non-2xx response. */
export async function loadInstance(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	planType: string
): Promise<Instance | null> {
	const response = await fetcher(instancePath(practiceId, engagementId, planType));
	if (response.status === 404) {
		return null;
	}
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Creates the Plan Instance for engagementId + planType, snapshotting the
 * Practice's current Plan Template fields server-side. Throws with the
 * response body text on a non-2xx response (e.g. the Practice has no
 * template for planType yet). */
export async function createInstance(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	planType: string
): Promise<Instance> {
	const response = await fetcher(instancePath(practiceId, engagementId, planType), { method: 'POST' });
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Replaces the full Answers map of the Plan Instance for engagementId +
 * planType -- PUT is a full replacement, not a merge, so callers must pass
 * every answer they want kept, not just the one that changed. */
export async function saveAnswers(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	planType: string,
	answers: Answers
): Promise<Instance> {
	const response = await fetcher(instancePath(practiceId, engagementId, planType), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ answers })
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Sets or clears the answer for fieldId within answers, returning a new
 * object (answers is never mutated) -- an empty string or an empty array
 * clears the answer entirely, so a field left blank doesn't round-trip as
 * a stored blank value. */
export function setAnswer(answers: Answers, fieldId: string, value: unknown): Answers {
	const isEmpty = value === '' || (Array.isArray(value) && value.length === 0);
	const next = { ...answers };
	if (isEmpty) {
		delete next[fieldId];
	} else {
		next[fieldId] = value;
	}
	return next;
}

/** Toggles option within the multi_select answer currently stored for
 * fieldId, returning a new Answers object. */
export function toggleMultiSelectOption(answers: Answers, fieldId: string, option: string): Answers {
	const current = Array.isArray(answers[fieldId]) ? (answers[fieldId] as string[]) : [];
	const next = current.includes(option) ? current.filter((o) => o !== option) : [...current, option];
	return setAnswer(answers, fieldId, next);
}
