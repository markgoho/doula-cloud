/**
 * What intake has been told, as the summary page reads it (#466).
 *
 * `CheckAnswers` takes `AnswerSection[]` and renders rows; deciding
 * which rows exist, what an unanswered one says and where its Change
 * link goes is the route's job, so it is done here where a unit test can
 * read it.
 *
 * ## Every question gets a row, answered or not
 *
 * The save is free (ADR-0017, settled on #466), so a record with only a
 * given name in it is a real record. A summary that listed only the
 * answered questions would show that reader one row and no way to see
 * what else was asked -- the Change link on an empty row is how the rest
 * of the sequence stays reachable from the end of it.
 *
 * ## The headings and the rows are the journey's own
 *
 * Both come from `intakeJourney.ts`'s `STRUCTURAL_STEPS`, so a section
 * heading here is the same string the rail shows for that step and a
 * Change link goes to the same slug the rail links to. They were typed
 * out twice until a review of this ticket noticed.
 */

import type { AnswerSection } from './components/templates/CheckAnswers.svelte';
import type { Field } from './clientFieldTemplate.js';
import { formatCalendarDay } from './dates.js';
import {
	CHANGE_QUERY,
	sectionSlug,
	STRUCTURAL_STEPS,
	type IntakeSection
} from './intakeJourney.js';
import type { FieldValue, IntakeAnswers } from './intakeDraft.svelte.js';

/** What an unanswered row says. Not an em dash: this page is read aloud
 * as often as it is looked at, and a dash announces as nothing. */
export const NOT_ANSWERED = 'Not answered';

function shown(value: string): string {
	return value.trim() === '' ? NOT_ANSWERED : value.trim();
}

/** One Practice-defined answer, in words. A checkbox is a yes or a no
 * rather than a blank, because "not ticked" is an answer; a multi-select
 * with nothing chosen is genuinely unanswered. */
export function fieldValueText(field: Field, value: FieldValue | undefined): string {
	if (field.type === 'checkbox') return value === true ? 'Yes' : 'No';
	if (Array.isArray(value)) return value.length > 0 ? value.join(', ') : NOT_ANSWERED;
	return shown(typeof value === 'string' ? value : '');
}

/*
 * A date of birth is stored as "YYYY-MM-DD" and read here as words --
 * `dates.ts`, which every other screen in the app already goes through.
 * GOV.UK's Check answers pattern shows a date the way a person says it,
 * and a summary that read "1988-02-09" back to somebody who had just
 * typed 2, 9 and 1988 into three boxes would be showing storage rather
 * than an answer.
 */
function answerText(key: string, value: string): string {
	if (key !== 'dateOfBirth' || value.trim() === '') return shown(value);
	return formatCalendarDay(value);
}

function changeHref(basePath: string, slug: string): string {
	return `${basePath}/${slug}?${CHANGE_QUERY}`;
}

/**
 * The whole summary.
 *
 * Section headings are the rail's own step labels, so the summary reads
 * as the journey it summarises rather than as a second grouping of the
 * same facts.
 */
export function answerSections(
	answers: IntakeAnswers,
	sections: IntakeSection[],
	basePath: string
): AnswerSection[] {
	const structural: AnswerSection[] = STRUCTURAL_STEPS.map((step) => ({
		heading: step.label,
		answers: step.questions.map((question) => ({
			label: question.label,
			value: answerText(question.key, answers[question.key]),
			changes: question.changes,
			changeHref: changeHref(basePath, step.slug)
		}))
	}));

	const practiceDefined: AnswerSection[] = sections.map((section, index) => ({
		heading: section.heading,
		answers: section.fields.map((field) => ({
			label: field.label,
			value: fieldValueText(field, answers.fieldValues[field.id]),
			changes: field.label,
			changeHref: changeHref(basePath, sectionSlug(index))
		}))
	}));

	return [...structural, ...practiceDefined];
}
