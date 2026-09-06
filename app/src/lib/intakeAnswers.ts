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
 */

import type { AnswerSection } from './components/templates/CheckAnswers.svelte';
import type { Field } from './clientFieldTemplate.js';
import type { IntakeSection } from './intakeJourney.js';
import type { FieldValue, IntakeAnswers } from './intakeDraft.svelte.js';

/** What an unanswered row says. Not an em dash: this page is read aloud
 * as often as it is looked at, and a dash announces as nothing. */
export const NOT_ANSWERED = 'Not answered';

/** The query the Change round trip carries, and the value it carries.
 * A question page reading it sends Continue back to the summary instead
 * of on to the next question. */
export const FROM_CHECK = 'from=check';

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

function changeHref(basePath: string, slug: string): string {
	return `${basePath}/${slug}?${FROM_CHECK}`;
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
	const structural: AnswerSection[] = [
		{
			heading: 'Name',
			answers: [
				{ label: 'Given name', value: shown(answers.givenName), changes: 'given name', changeHref: changeHref(basePath, 'name') },
				{ label: 'Family name', value: shown(answers.familyName), changes: 'family name', changeHref: changeHref(basePath, 'name') },
				{ label: 'Preferred name', value: shown(answers.preferredName), changes: 'preferred name', changeHref: changeHref(basePath, 'name') }
			]
		},
		{
			heading: 'Date of birth',
			answers: [
				{ label: 'Date of birth', value: shown(answers.dateOfBirth), changes: 'date of birth', changeHref: changeHref(basePath, 'date-of-birth') }
			]
		},
		{
			heading: 'Email address',
			answers: [
				{ label: 'Email address', value: shown(answers.email), changes: 'email address', changeHref: changeHref(basePath, 'email') }
			]
		},
		{
			heading: 'Phone number',
			answers: [
				{ label: 'Phone number', value: shown(answers.phone), changes: 'phone number', changeHref: changeHref(basePath, 'phone') }
			]
		},
		{
			heading: 'Address',
			answers: [
				{ label: 'Address line 1', value: shown(answers.addressLine1), changes: 'address line 1', changeHref: changeHref(basePath, 'address') },
				{ label: 'Address line 2', value: shown(answers.addressLine2), changes: 'address line 2', changeHref: changeHref(basePath, 'address') },
				{ label: 'City', value: shown(answers.addressLocality), changes: 'city', changeHref: changeHref(basePath, 'address') },
				{ label: 'State', value: shown(answers.addressRegion), changes: 'state', changeHref: changeHref(basePath, 'address') },
				{ label: 'ZIP code', value: shown(answers.addressPostalCode), changes: 'ZIP code', changeHref: changeHref(basePath, 'address') }
			]
		}
	];

	const practiceDefined: AnswerSection[] = sections.map((section, index) => ({
		heading: section.heading,
		answers: section.fields.map((field) => ({
			label: field.label,
			value: fieldValueText(field, answers.fieldValues[field.id]),
			changes: field.label,
			changeHref: changeHref(basePath, `sections/${index}`)
		}))
	}));

	return [...structural, ...practiceDefined];
}
