/**
 * One of a Practice's own questions on its own page (#2193): a Plan
 * Template's field or a Client Field Template's field, edited one at a
 * time and saved as the whole list, since both writes replace the list
 * wholesale (`PUT .../plan-templates/{planType}`,
 * `PUT .../client-field-template`).
 *
 * Both lists share this logic because they should change together: the
 * place a question takes in its list, what its page refuses before the
 * boundary does, and how its answers box becomes a list of answers. What
 * differs (an archived Client field keeps its place and is not counted)
 * is the `archived` flag, which a Plan Template field never carries.
 */
import { isSelectType, type FieldType } from './planTemplate.js';
import type { FormError } from './components/molecules/ErrorSummary.svelte';

interface Listed {
	id: string;
	order: number;
	archived?: boolean;
}

function isAsked(question: Listed): boolean {
	return !question.archived;
}

function renumbered<F extends Listed>(fields: readonly F[]): F[] {
	return fields.map((field, order) => ({ ...field, order }));
}

/**
 * How many questions the list still asks: an archived one is not counted.
 */
export function activeQuestionCount(fields: readonly Listed[]): number {
	return fields.filter((field) => isAsked(field)).length;
}

/**
 * The question's place among the ones still asked, counted from 1; 0 for
 * a question the list does not hold.
 */
export function questionPosition(fields: readonly Listed[], id: string): number {
	return fields.filter((field) => isAsked(field)).findIndex((field) => field.id === id) + 1;
}

/**
 * Moves the question with `id` to `position` (from 1) among the questions
 * still asked. An archived question keeps its slot in the array, so its
 * stored order is not rewritten by a move it took no part in. A place out
 * of range, or a question the list does not hold, leaves the list as it is.
 */
export function placeQuestion<F extends Listed>(fields: readonly F[], id: string, position: number): F[] {
	const moved = fields.find((field) => field.id === id);
	const count = activeQuestionCount(fields);
	if (!moved || position < 1 || position > count || position === questionPosition(fields, id)) {
		return [...fields];
	}
	const rest = fields.filter((field) => field.id !== id);
	const askedRest = rest.filter((field) => isAsked(field));
	// Before the question now at that place, or after the last one asked
	// (there is one: a list of one asked question has only its own place).
	const before = askedRest[position - 1];
	const index = before ? rest.indexOf(before) : rest.indexOf(askedRest.at(-1)!) + 1;
	return renumbered([...rest.slice(0, index), moved, ...rest.slice(index)]);
}

/**
 * The list with `question` in it: in its own place when the list holds
 * its id, at the end when it is new.
 */
export function withQuestion<F extends Listed>(fields: readonly F[], question: F): F[] {
	const isHeld = fields.some((field) => field.id === question.id);
	return renumbered(isHeld ? fields.map((field) => (field.id === question.id ? question : field)) : [...fields, question]);
}

/**
 * The answers box as a list: one answer on each line, trimmed, and a
 * blank line is dropped rather than refused.
 */
export function optionsFromText(text: string): string[] {
	return text
		.split('\n')
		.map((line) => line.trim())
		.filter((line) => line !== '');
}

interface Draft {
	type: FieldType;
	label: string;
	options: readonly string[];
}

/**
 * What the question's page refuses before the boundary does, in GOV.UK's
 * error-message words ("Enter the …"). `refuseLabel` is the caller's own
 * rule about the words of a question -- the Client Field Template's, that
 * a question restating a structural fact is not one to add.
 */
export function questionErrors(
	draft: Draft,
	targets: { label: string; options: string },
	refuseLabel?: (label: string) => string | undefined
): FormError[] {
	const errors: FormError[] = [];
	const label = draft.label.trim();
	if (label === '') {
		errors.push({
			message: draft.type === 'section_header' ? 'Enter the heading' : 'Enter the question',
			targetId: targets.label
		});
	} else {
		const refusal = refuseLabel?.(label);
		if (refusal) {
			errors.push({ message: refusal, targetId: targets.label });
		}
	}
	if (isSelectType(draft.type) && draft.options.length === 0) {
		errors.push({ message: 'Enter the answers to choose from, one on each line', targetId: targets.options });
	}
	return errors;
}
