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
 * ## The rows are the journey's own
 *
 * They come from `intakeJourney.ts`'s `STRUCTURAL_STEPS`, so the address
 * row's label is the same string the rail shows for that step and a
 * Change link goes to the same slug the rail links to. They were typed
 * out twice until a review of this ticket noticed.
 */

import type { Answer, AnswerSection } from './components/templates/CheckAnswers.svelte';
import type { Field } from './clientFieldTemplate.js';
import { formatAddress, formatPhoneNumber } from './contactDetails.js';
import { formatCalendarDay } from './dates.js';
import {
	CHANGE_QUERY,
	sectionSlug,
	STRUCTURAL_STEPS,
	type IntakeSection,
	type StructuralStep
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
	if (value.trim() === '') return NOT_ANSWERED;
	if (key === 'dateOfBirth') return formatCalendarDay(value);
	// A US number in one form, any other as typed: `contactDetails.ts` (#2147).
	if (key === 'phone') return formatPhoneNumber(value);
	return shown(value);
}

function changeHref(basePath: string, slug: string): string {
	return `${basePath}/${slug}?${CHANGE_QUERY}`;
}

/*
 * The address is one answer to a person, asked as five boxes: GOV.UK's
 * Check answers shows it as one row with its lines joined, and one
 * Change, rather than five rows that each say "Address line 2: Not
 * answered" (#2033). Written on one line by `contactDetails.ts`, the way
 * the Client record's own Address row reads it: no comma before the ZIP
 * code (#2147).
 */
const ADDRESS_SLUG = 'address';

function stepAnswers(answers: IntakeAnswers, step: StructuralStep, basePath: string): Answer[] {
	const href = changeHref(basePath, step.slug);
	if (step.slug === ADDRESS_SLUG) {
		const address = formatAddress(answers);
		return [
			{
				label: step.label,
				value: address || NOT_ANSWERED,
				changes: step.label.toLowerCase(),
				changeHref: href
			}
		];
	}
	return step.questions.map((question) => ({
		label: question.label,
		value: answerText(question.key, answers[question.key]),
		changes: question.changes,
		changeHref: href
	}));
}

/**
 * The whole summary.
 *
 * The built-in questions are one group with no heading of its own (#2033):
 * each is one row, so a heading over each repeated its only row's label.
 * A Practice's own sections each keep their heading, the rail's own step
 * label for that page, because those rows do form groups.
 */
export function answerSections(
	answers: IntakeAnswers,
	sections: IntakeSection[],
	basePath: string,
	structuralSteps: readonly StructuralStep[] = STRUCTURAL_STEPS
): AnswerSection[] {
	const structural: AnswerSection = {
		answers: structuralSteps.flatMap((step) => stepAnswers(answers, step, basePath))
	};

	const practiceDefined: AnswerSection[] = sections.map((section, index) => ({
		heading: section.heading,
		answers: section.fields.map((field) => ({
			label: field.label,
			value: fieldValueText(field, answers.fieldValues[field.id]),
			changes: field.label,
			changeHref: changeHref(basePath, sectionSlug(index))
		}))
	}));

	return [structural, ...practiceDefined];
}
