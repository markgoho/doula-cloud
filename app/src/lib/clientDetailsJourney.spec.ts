import { describe, expect, it } from 'vitest';
import type { ClientDetail, ResolvedField } from './clientDetail.js';
import {
	answersOnFile,
	canAddDetails,
	detailsLinkLabel,
	detailsToSave,
	engagementOrigin,
	fieldSectionIndexes,
	hasEveryDetail,
	stepOrigin
} from './clientDetailsJourney.js';
import { blankAnswers, type IntakeAnswers } from './intakeDraft.svelte.js';

function detail(partial: Partial<ClientDetail> = {}): ClientDetail {
	return {
		id: 'client-1',
		givenName: 'Anne-Marie',
		familyName: 'Ochieng',
		preferredName: '',
		email: '',
		phone: '',
		addressLine1: '',
		addressLine2: '',
		addressLocality: '',
		addressRegion: '',
		addressPostalCode: '',
		dateOfBirth: '',
		fieldValues: {},
		resolvedFields: [],
		engagements: [],
		history: [],
		...partial
	};
}

function resolved(partial: Partial<ResolvedField> & { fieldId: string }): ResolvedField {
	return { label: partial.fieldId, type: 'short_text', ...partial };
}

const complete: Partial<ClientDetail> = {
	email: 'anne@example.com',
	phone: '585 555 0142',
	addressLine1: '1 Main Street',
	addressLocality: 'Rochester',
	addressRegion: 'NY',
	addressPostalCode: '14607',
	dateOfBirth: '1988-02-09'
};

function typed(partial: Partial<IntakeAnswers>): IntakeAnswers {
	return { ...blankAnswers(), ...partial };
}

describe('answersOnFile', () => {
	it('starts each question from the value on file', () => {
		const answers = answersOnFile(
			detail({ ...complete, fieldValues: { referral: 'A sister' } })
		);

		expect(answers).toMatchObject({ email: 'anne@example.com', dateOfBirth: '1988-02-09' });
		expect(answers.fieldValues).toEqual({ referral: 'A sister' });
	});

	it('reads no Practice-defined values as none', () => {
		expect(answersOnFile(detail({ fieldValues: undefined })).fieldValues).toEqual({});
	});
});

describe('detailsToSave', () => {
	it('saves one fact typed over a blank', () => {
		const record = detail();
		const saved = detailsToSave(record, { ...answersOnFile(record), phone: '585 555 0199' }, []);

		expect(saved.phone).toBe('585 555 0199');
		expect(saved.givenName).toBe('Anne-Marie');
	});

	// #2048: the journey is the one way to change her record, so it asks
	// the name too.
	it('saves a changed name', () => {
		const record = detail();
		const saved = detailsToSave(record, { ...answersOnFile(record), givenName: 'Annie' }, []);

		expect(saved.givenName).toBe('Annie');
	});

	it('saves a Practice-defined value', () => {
		const saved = detailsToSave(
			detail({ fieldValues: { photos: true } }),
			typed({ fieldValues: { referral: 'A sister' } }),
			['referral', 'photos']
		);

		expect(saved.fieldValues).toEqual({ referral: 'A sister', photos: true });
	});

	// #2048: with the Edit form retired, a question cleared on its page is
	// how a fact is taken away.
	it('clears the value on file where the answer is blank', () => {
		const saved = detailsToSave(
			detail({ ...complete, fieldValues: { referral: 'A sister', attendees: ['Partner'] } }),
			typed({ email: '', fieldValues: { referral: '', attendees: [] } }),
			['referral', 'attendees']
		);

		expect(saved.email).toBe('');
		expect(saved.fieldValues).toEqual({ referral: '', attendees: [] });
	});

	it('keeps a Practice value the draft holds no answer to', () => {
		const saved = detailsToSave(detail({ fieldValues: { referral: 'A sister' } }), typed({}), ['referral']);

		expect(saved.fieldValues).toEqual({ referral: 'A sister' });
	});

	// A ticked box that is unticked is an answer, not a blank.
	it('saves an unticked box over a ticked one', () => {
		const saved = detailsToSave(
			detail({ fieldValues: { photos: true } }),
			typed({ fieldValues: { photos: false } }),
			['photos']
		);

		expect(saved.fieldValues).toEqual({ photos: false });
	});

	// The journey never asks a field the Practice archived, so a stale
	// draft cannot change one.
	it('takes a field the journey does not ask from the record on file', () => {
		const saved = detailsToSave(
			detail({ fieldValues: { retired: 'Kept' } }),
			typed({ fieldValues: { retired: 'Changed' } }),
			['referral']
		);

		expect(saved.fieldValues).toEqual({ retired: 'Kept' });
	});
});

describe('stepOrigin', () => {
	it.each([['name'], ['date-of-birth'], ['address'], ['sections/2']])('reads the question %s', (slug) => {
		expect(stepOrigin(new URLSearchParams({ step: slug }))).toBe(slug);
	});

	it.each([
		['no question', ''],
		['an address', 'step=..%2F..%2Flogin'],
		['a step the journey has not', 'step=check'],
		['a section that is not a number', 'step=sections%2Fone']
	])('reads %s as none', (_case, query) => {
		expect(stepOrigin(new URLSearchParams(query))).toBeUndefined();
	});
});

describe('fieldSectionIndexes', () => {
	// The fields before the first header are a section of their own, a
	// header over nothing opens none, and a field the Practice archived
	// is asked by no section.
	it('counts the sections the journey asks, the way intake does', () => {
		const indexes = fieldSectionIndexes([
			resolved({ fieldId: 'referral' }),
			resolved({ fieldId: 'old', note: 'No longer collected' }),
			resolved({ fieldId: 'care', type: 'section_header' }),
			resolved({ fieldId: 'hopes' }),
			resolved({ fieldId: 'empty', type: 'section_header' }),
			resolved({ fieldId: 'birth', type: 'section_header' }),
			resolved({ fieldId: 'place' })
		]);

		expect([...indexes]).toEqual([
			['referral', 0],
			['hopes', 1],
			['place', 2]
		]);
	});

	it('starts at the first section when the first field is a header', () => {
		const indexes = fieldSectionIndexes([
			resolved({ fieldId: 'care', type: 'section_header' }),
			resolved({ fieldId: 'hopes' })
		]);

		expect(indexes.get('hopes')).toBe(0);
	});
});

describe('hasEveryDetail', () => {
	it('is false for a record with only a name', () => {
		expect(hasEveryDetail(detail())).toBe(false);
	});

	it('is true once every question has an answer on file', () => {
		const resolvedFields = [
			resolved({ fieldId: 'h', type: 'section_header' }),
			resolved({ fieldId: 'photos', type: 'checkbox', value: false }),
			resolved({ fieldId: 'old', note: 'No longer collected' })
		];

		expect(hasEveryDetail(detail({ ...complete, resolvedFields }))).toBe(true);
	});

	// The second address line is the one part of an address a person may
	// not have.
	it('does not need a second address line', () => {
		expect(hasEveryDetail(detail({ ...complete, addressLine2: '' }))).toBe(true);
	});

	it.each([
		['the date of birth', { dateOfBirth: '' }],
		['the city', { addressLocality: '' }]
	])('is false without %s', (_fact, missing) => {
		expect(hasEveryDetail(detail({ ...complete, ...missing }))).toBe(false);
	});

	it.each([
		['no value', undefined],
		['a blank answer', ''],
		['nothing chosen', []]
	])('is false where a Practice question has %s', (_state, value) => {
		const resolvedFields = [resolved({ fieldId: 'referral', value })];

		expect(hasEveryDetail(detail({ ...complete, resolvedFields }))).toBe(false);
	});
});

describe('detailsLinkLabel', () => {
	it('says Add while a fact is missing', () => {
		expect(detailsLinkLabel(detail())).toBe("Add Anne-Marie's details");
	});

	// The first name, her preferred one where she has one (#2115): the
	// page's heading already names her in full.
	it('says Change once each fact is given', () => {
		expect(detailsLinkLabel(detail({ ...complete, preferredName: 'Annie' }))).toBe(
			"Change Annie's details"
		);
	});
});

describe('engagementOrigin', () => {
	it('reads the Engagement that opened the journey', () => {
		expect(engagementOrigin(new URLSearchParams('engagement=engagement-1'))).toBe('engagement-1');
	});

	it.each([
		['no Engagement', ''],
		['an address', 'engagement=..%2F..%2Flogin'],
		['an empty value', 'engagement=']
	])('reads %s as none', (_case, query) => {
		expect(engagementOrigin(new URLSearchParams(query))).toBeUndefined();
	});
});

describe('canAddDetails', () => {
	it('is true for an ordinary record', () => {
		expect(canAddDetails(detail())).toBe(true);
	});

	it.each([
		['an erased record', { erasedAt: '2026-09-01T00:00:00Z' }],
		['a merged record', { mergedInto: 'client-2' }]
	])('is false for %s', (_kind, partial) => {
		expect(canAddDetails(detail(partial))).toBe(false);
	});
});
