import { describe, expect, it } from 'vitest';
import type { ClientEditFields, CollisionMatch } from './client.js';
import { proposedChanges, proposedMergeChanges } from './proposedChanges.js';

function fields(partial: Partial<ClientEditFields> = {}): ClientEditFields {
	return {
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
		fieldValues: {},
		...partial
	};
}

function match(partial: Partial<CollisionMatch> = {}): CollisionMatch {
	return { ...fields(), id: 'c1', wouldSurvive: true, engagements: [], ...partial };
}

describe('proposedChanges', () => {
	it('has nothing to propose when the two sides already agree', () => {
		expect(proposedChanges(fields(), fields())).toEqual([]);
	});

	it('proposes a genuinely different value, naming what the survivor holds now', () => {
		const absorbed = fields({ phone: '555-0100', familyName: 'Okafor-Reid' });

		expect(proposedChanges(fields(), absorbed)).toEqual([
			{ label: 'Family name', before: 'Okafor', after: 'Okafor-Reid' },
			{ label: 'Phone number', before: 'Not answered', after: '555-0100' }
		]);
	});

	it("never proposes overwriting the survivor's value with a blank", () => {
		expect(proposedChanges(fields(), fields({ familyName: '', email: ' ' }))).toEqual([]);
	});

	it('does not read surrounding whitespace as a difference', () => {
		expect(proposedChanges(fields(), fields({ familyName: ' Okafor ' }))).toEqual([]);
	});
});

describe('proposedMergeChanges', () => {
	it('reads the match as the survivor when it would survive', () => {
		const typed = fields({ familyName: 'Okafor-Reid' });

		expect(proposedMergeChanges(typed, match({ wouldSurvive: true }))).toEqual([
			{ label: 'Family name', before: 'Okafor', after: 'Okafor-Reid' }
		]);
	});

	it('reads the record being edited as the survivor when the match would not', () => {
		const absorbed = match({ wouldSurvive: false, familyName: 'Okafor-Reid' });

		expect(proposedMergeChanges(fields(), absorbed)).toEqual([
			{ label: 'Family name', before: 'Okafor', after: 'Okafor-Reid' }
		]);
	});
});
