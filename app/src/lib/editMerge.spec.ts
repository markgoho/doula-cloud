import { describe, expect, it } from 'vitest';
import type { ClientEditFields, CollisionMatch } from './client.js';
import { proposedMergeChanges } from './editMerge.js';

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
		wouldSurvive: true,
		engagements: [],
		...partial
	};
}

describe('proposedMergeChanges', () => {
	it('has nothing to propose when the two sides already agree', () => {
		expect(proposedMergeChanges(fields(), match())).toEqual([]);
	});

	describe('when the match survives (wouldSurvive: true)', () => {
		it('proposes what was typed as a change to the match, naming what is on file', () => {
			const typed = fields({ phone: '555-0100', familyName: 'Okafor-Reid' });

			expect(proposedMergeChanges(typed, match())).toEqual([
				{ label: 'Family name', onFile: 'Okafor', typed: 'Okafor-Reid' },
				{ label: 'Phone number', onFile: 'Not answered', typed: '555-0100' }
			]);
		});

		it('never proposes overwriting the match with a blank typed value', () => {
			const typed = fields({ familyName: '' });

			expect(proposedMergeChanges(typed, match())).toEqual([]);
		});
	});

	describe('when the record being edited survives (wouldSurvive: false)', () => {
		it("proposes the match's own values as what would overwrite what was typed", () => {
			const survivor = fields({ phone: '', familyName: 'Okafor' });
			const absorbed = match({
				wouldSurvive: false,
				phone: '555-0199',
				familyName: 'Okafor-Reid'
			});

			expect(proposedMergeChanges(survivor, absorbed)).toEqual([
				{ label: 'Family name', onFile: 'Okafor', typed: 'Okafor-Reid' },
				{ label: 'Phone number', onFile: 'Not answered', typed: '555-0199' }
			]);
		});

		it("never proposes overwriting what was typed with the match's own blank", () => {
			const survivor = fields({ familyName: 'Okafor' });
			const absorbed = match({ wouldSurvive: false, familyName: '' });

			expect(proposedMergeChanges(survivor, absorbed)).toEqual([]);
		});

		it('has nothing to propose when both sides already agree', () => {
			const absorbed = match({ wouldSurvive: false });

			expect(proposedMergeChanges(fields(), absorbed)).toEqual([]);
		});
	});
});
