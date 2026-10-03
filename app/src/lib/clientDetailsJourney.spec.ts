import { describe, expect, it } from 'vitest';
import type { ClientDetail, ResolvedField } from './clientDetail.js';
import {
	answersOnFile,
	canAddDetails,
	detailsLinkLabel,
	detailsToSave,
	engagementOrigin,
	hasEveryDetail
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
		const saved = detailsToSave(detail(), typed({ phone: '585 555 0199' }), []);

		expect(saved.phone).toBe('585 555 0199');
		expect(saved.givenName).toBe('Anne-Marie');
	});

	it('saves a Practice-defined value', () => {
		const saved = detailsToSave(
			detail({ fieldValues: { photos: true } }),
			typed({ fieldValues: { referral: 'A sister' } }),
			['referral', 'photos']
		);

		expect(saved.fieldValues).toEqual({ referral: 'A sister', photos: true });
	});

	// #1610: a blank never replaces a value on file. The Edit form is
	// the path for taking a fact away.
	it('keeps the value on file where the answer is blank', () => {
		const saved = detailsToSave(
			detail({ ...complete, fieldValues: { referral: 'A sister', attendees: ['Partner'] } }),
			typed({ email: '  ', fieldValues: { referral: '', attendees: [] } }),
			['referral', 'attendees']
		);

		expect(saved.email).toBe('anne@example.com');
		expect(saved.fieldValues).toEqual({ referral: 'A sister', attendees: ['Partner'] });
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

	// The journey never asks the name or a field the Practice archived,
	// so a stale draft cannot change either.
	it('takes every fact the journey does not ask from the record on file', () => {
		const saved = detailsToSave(
			detail({ fieldValues: { retired: 'Kept' } }),
			typed({ givenName: 'Someone else', fieldValues: { retired: 'Changed' } }),
			['referral']
		);

		expect(saved.givenName).toBe('Anne-Marie');
		expect(saved.fieldValues).toEqual({ retired: 'Kept' });
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
		expect(detailsLinkLabel(detail())).toBe("Add Anne-Marie Ochieng's details");
	});

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
