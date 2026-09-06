import { describe, expect, it } from 'vitest';
import type { ClientMatch } from './client.js';
import { blankAnswers } from './intakeDraft.svelte.js';
import { mergedEditFields, proposedChanges } from './intakeMerge.js';

function match(partial: Partial<ClientMatch> = {}): ClientMatch {
	return {
		id: 'c1',
		givenName: 'Sarah',
		familyName: 'Okafor',
		preferredName: '',
		email: 'sarah@example.com',
		phone: '',
		addressLine1: '12 Elm Street',
		addressLine2: '',
		addressLocality: 'Rochester',
		addressRegion: 'NY',
		addressPostalCode: '14607',
		dateOfBirth: '1988-02-09',
		engagements: [],
		...partial
	};
}

describe('proposedChanges', () => {
	it('has nothing to propose when what was typed is already on file', () => {
		const answers = { ...blankAnswers(), givenName: 'Sarah', email: 'sarah@example.com' };

		expect(proposedChanges(answers, match())).toEqual([]);
	});

	it('never proposes overwriting what is on file with a blank', () => {
		expect(proposedChanges(blankAnswers(), match())).toEqual([]);
	});

	it('proposes a genuinely different value, naming what is there now', () => {
		const answers = { ...blankAnswers(), phone: '555-0100', familyName: 'Okafor-Reid' };

		expect(proposedChanges(answers, match())).toEqual([
			{ label: 'Family name', onFile: 'Okafor', typed: 'Okafor-Reid' },
			{ label: 'Phone number', onFile: 'Not answered', typed: '555-0100' }
		]);
	});
});

describe('mergedEditFields', () => {
	it('keeps what is on file where intake typed nothing', () => {
		const merged = mergedEditFields(blankAnswers(), match());

		expect(merged.addressLocality).toBe('Rochester');
		expect(merged.dateOfBirth).toBe('1988-02-09');
	});

	it('takes what intake typed where it typed something', () => {
		const answers = { ...blankAnswers(), addressPostalCode: '14620' };

		expect(mergedEditFields(answers, match()).addressPostalCode).toBe('14620');
	});

	// #495's hazard: a full-object PUT that did not round-trip
	// fieldValues would silently wipe a Practice's own data.
	it('round-trips the Practice-defined values already on file', () => {
		const merged = mergedEditFields(
			blankAnswers(),
			match({ fieldValues: { allergies: 'Peanuts' } })
		);

		expect(merged.fieldValues).toEqual({ allergies: 'Peanuts' });
	});

	it('layers what intake collected over what is on file', () => {
		const answers = { ...blankAnswers(), fieldValues: { allergies: 'None' } };

		const merged = mergedEditFields(
			answers,
			match({ fieldValues: { allergies: 'Peanuts', doula: 'Anne' } })
		);

		expect(merged.fieldValues).toEqual({ allergies: 'None', doula: 'Anne' });
	});

	it('reads an absent field-value map as an empty one', () => {
		expect(mergedEditFields(blankAnswers(), match({ fieldValues: undefined })).fieldValues).toEqual(
			{}
		);
	});
});
