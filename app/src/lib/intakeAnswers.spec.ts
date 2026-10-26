import { describe, expect, it } from 'vitest';
import type { Field } from './clientFieldTemplate.js';
import { answerSections, fieldValueText, NOT_ANSWERED } from './intakeAnswers.js';
import { blankAnswers } from './intakeDraft.svelte.js';
import { DETAILS_STEPS } from './intakeJourney.js';

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

function rowLabels(answers: ReturnType<typeof answerSections>[number]['answers']): string[] {
	return answers.map((answer) => answer.label);
}

describe('answerSections', () => {
	// #2033: one row each, so a heading over each repeated its only row.
	it('puts every built-in question in one group with no heading, answered or not', () => {
		const sections = answerSections({ ...blankAnswers(), givenName: 'Sarah' }, [], basePath);

		expect(sections).toHaveLength(1);
		expect(sections[0].heading).toBeUndefined();
		expect(rowLabels(sections[0].answers)).toEqual([
			'Given name',
			'Family name',
			'Preferred name',
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

	// #1610: the details journey never asks the name, so it has no row.
	it('lists only the steps a details journey asks', () => {
		const sections = answerSections(blankAnswers(), [], basePath, DETAILS_STEPS);

		expect(rowLabels(sections[0].answers)).toEqual(['Date of birth', 'Email address', 'Phone number', 'Address']);
		expect(sections[0].answers[0].changeHref).toBe(`${basePath}/date-of-birth?from=check`);
	});

	// #2033: GOV.UK shows an address as one row with its lines joined.
	it('reads the address as one row, its answered lines joined, with one Change', () => {
		const sections = answerSections(
			{
				...blankAnswers(),
				addressLine1: '12 Lake Avenue',
				addressLocality: 'Rochester',
				addressRegion: 'NY',
				addressPostalCode: '14608'
			},
			[],
			basePath,
			DETAILS_STEPS
		);

		expect(sections[0].answers.at(-1)).toEqual({
			label: 'Address',
			value: '12 Lake Avenue, Rochester, NY, 14608',
			changes: 'address',
			changeHref: `${basePath}/address?from=check`
		});
	});

	it('reads an address with no line answered as unanswered', () => {
		const sections = answerSections(blankAnswers(), [], basePath, DETAILS_STEPS);

		expect(sections[0].answers.at(-1)?.value).toBe(NOT_ANSWERED);
	});

	// GOV.UK's Check answers shows a date the way a person says it, and
	// `dates.ts` is where every other screen in the app formats one.
	it('reads the date of birth back in words, not as it is stored', () => {
		const sections = answerSections({ ...blankAnswers(), dateOfBirth: '1988-02-09' }, [], basePath, DETAILS_STEPS);

		expect(sections[0].answers[0].value).toBe('Feb 9, 1988');
	});

	it('leaves an unanswered date of birth unanswered rather than formatting a blank', () => {
		const sections = answerSections(blankAnswers(), [], basePath, DETAILS_STEPS);

		expect(sections[0].answers[0].value).toBe(NOT_ANSWERED);
	});

	it('sends each Change link back to the question that asked it', () => {
		const sections = answerSections(blankAnswers(), [], basePath, DETAILS_STEPS);

		expect(sections[0].answers.map((answer) => answer.changeHref)).toEqual([
			`${basePath}/date-of-birth?from=check`,
			`${basePath}/email?from=check`,
			`${basePath}/phone?from=check`,
			`${basePath}/address?from=check`
		]);
	});

	it('adds a headed group per Practice-named page, each Change link its own', () => {
		const sections = answerSections(
			{ ...blankAnswers(), fieldValues: { allergies: 'Peanuts' } },
			[
				{ heading: 'Health', fields: [field({ id: 'allergies', label: 'Allergies' })] },
				{ heading: 'Birth', fields: [field({ id: 'plan', label: 'Birth plan' })] }
			],
			basePath
		);

		expect(sections).toHaveLength(3);
		expect(sections[1]).toEqual({
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
		expect(sections[2].answers[0].changeHref).toBe(`${basePath}/sections/1?from=check`);
	});
});
