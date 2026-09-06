import { describe, expect, it } from 'vitest';
import type { Field } from './clientFieldTemplate.js';
import { answerSections, fieldValueText, NOT_ANSWERED } from './intakeAnswers.js';
import { blankAnswers } from './intakeDraft.svelte.js';

const basePath = '/practices/p1/clients/new';

function field(partial: Partial<Field> & { id: string }): Field {
	return { type: 'short_text', label: partial.id, order: 0, archived: false, ...partial };
}

describe('fieldValueText', () => {
	it('reads a text answer as itself', () => {
		expect(fieldValueText(field({ id: 'a' }), 'Peanuts')).toBe('Peanuts');
	});

	it('reads an unanswered text question as unanswered', () => {
		expect(fieldValueText(field({ id: 'a' }), '  ')).toBe(NOT_ANSWERED);
		expect(fieldValueText(field({ id: 'a' }), undefined)).toBe(NOT_ANSWERED);
	});

	it('joins the choices of a multi-select', () => {
		expect(fieldValueText(field({ id: 'a', type: 'multi_select' }), ['Photos', 'Email'])).toBe(
			'Photos, Email'
		);
	});

	it('reads a multi-select with nothing chosen as unanswered', () => {
		expect(fieldValueText(field({ id: 'a', type: 'multi_select' }), [])).toBe(NOT_ANSWERED);
	});

	// An unticked checkbox is an answer, not a blank.
	it('reads a checkbox as a yes or a no', () => {
		expect(fieldValueText(field({ id: 'a', type: 'checkbox' }), true)).toBe('Yes');
		expect(fieldValueText(field({ id: 'a', type: 'checkbox' }), undefined)).toBe('No');
	});
});

describe('answerSections', () => {
	it('lists every structural question, answered or not', () => {
		const sections = answerSections({ ...blankAnswers(), givenName: 'Sarah' }, [], basePath);

		expect(sections.map((section) => section.heading)).toEqual([
			'Name',
			'Date of birth',
			'Email address',
			'Phone number',
			'Address'
		]);
		expect(sections[0].answers[0]).toEqual({
			label: 'Given name',
			value: 'Sarah',
			changes: 'given name',
			changeHref: `${basePath}/name?from=check`
		});
		expect(sections[0].answers[1].value).toBe(NOT_ANSWERED);
	});

	it('reaches all twelve structural columns but the id', () => {
		const rows = answerSections(blankAnswers(), [], basePath).flatMap((section) => section.answers);

		expect(rows).toHaveLength(11);
	});

	// GOV.UK's Check answers shows a date the way a person says it, and
	// `dates.ts` is where every other screen in the app formats one.
	it('reads the date of birth back in words, not as it is stored', () => {
		const sections = answerSections(
			{ ...blankAnswers(), dateOfBirth: '1988-02-09' },
			[],
			basePath
		);

		expect(sections[1].answers[0].value).toBe('Feb 9, 1988');
	});

	it('leaves an unanswered date of birth unanswered rather than formatting a blank', () => {
		const sections = answerSections(blankAnswers(), [], basePath);

		expect(sections[1].answers[0].value).toBe(NOT_ANSWERED);
	});

	it('sends each Change link back to the question that asked it', () => {
		const sections = answerSections(blankAnswers(), [], basePath);

		expect(sections[4].answers[4].changeHref).toBe(`${basePath}/address?from=check`);
		expect(sections[1].answers[0].changeHref).toBe(`${basePath}/date-of-birth?from=check`);
	});

	it('adds a section per Practice-named page, each Change link its own', () => {
		const sections = answerSections(
			{ ...blankAnswers(), fieldValues: { allergies: 'Peanuts' } },
			[
				{ heading: 'Health', fields: [field({ id: 'allergies', label: 'Allergies' })] },
				{ heading: 'Birth', fields: [field({ id: 'plan', label: 'Birth plan' })] }
			],
			basePath
		);

		expect(sections).toHaveLength(7);
		expect(sections[5]).toEqual({
			heading: 'Health',
			answers: [
				{
					label: 'Allergies',
					value: 'Peanuts',
					changes: 'Allergies',
					changeHref: `${basePath}/sections/0?from=check`
				}
			]
		});
		expect(sections[6].answers[0].changeHref).toBe(`${basePath}/sections/1?from=check`);
	});
});
