import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { blankAnswers, IntakeDraft, intakeDraft } from './intakeDraft.svelte.js';

/* What a browser configured to refuse site data does on every one of the
   three calls: it throws, rather than returning null. */
function denied(): never {
	throw new Error('denied');
}

beforeEach(() => {
	sessionStorage.clear();
});

afterEach(() => {
	vi.restoreAllMocks();
});

describe('IntakeDraft', () => {
	it('starts blank', () => {
		const draft = new IntakeDraft();

		expect(draft.answers).toEqual(blankAnswers());
		expect(draft.hasGivenName).toBe(false);
	});

	// ADR-0017: the search that fronts intake carries whatever was typed
	// into it, so a genuinely new Client costs nothing to search first.
	it('seeds an empty draft from what the search carried', () => {
		const draft = new IntakeDraft();

		draft.start('p1', { givenName: 'Sarah', phone: '555-0100' });

		expect(draft.answers.givenName).toBe('Sarah');
		expect(draft.answers.phone).toBe('555-0100');
		expect(draft.hasGivenName).toBe(true);
	});

	it('never overwrites what has been typed with what was searched', () => {
		const draft = new IntakeDraft();
		draft.start('p1', { givenName: 'Sarah' });
		draft.update({ givenName: 'Sara' });

		draft.start('p1', { givenName: 'Sarah' });

		expect(draft.answers.givenName).toBe('Sara');
	});

	// The layout seeds from the query string on every render, and a reader
	// who clears the given name to retype it flips `hasGivenName` back to
	// false -- so a seed carrying empty strings would wipe the phone,
	// email and date of birth already typed on the pages after it.
	it('never seeds a key the search did not carry', () => {
		const draft = new IntakeDraft();
		draft.start('p1', { givenName: 'Sarah', phone: '555-0100' });
		draft.update({ givenName: '' });

		draft.start('p1', {});

		expect(draft.answers.phone).toBe('555-0100');
	});

	it('drops the draft when the Practice changes', () => {
		const draft = new IntakeDraft();
		draft.start('p1', { givenName: 'Sarah' });
		draft.visit('name');

		draft.start('p2');

		expect(draft.answers).toEqual(blankAnswers());
		expect(draft.visitedSteps).toEqual([]);
	});

	it('records a Practice-defined answer under its field id', () => {
		const draft = new IntakeDraft();
		draft.start('p1');

		draft.setFieldValue('allergies', 'None');
		draft.setFieldValue('consents', ['photos', 'email']);

		expect(draft.answers.fieldValues).toEqual({ allergies: 'None', consents: ['photos', 'email'] });
	});

	it('counts a step once however many times it is walked', () => {
		const draft = new IntakeDraft();

		draft.visit('name');
		draft.visit('name');
		draft.visit('email');

		expect(draft.visitedSteps).toEqual(['name', 'email']);
	});

	it('leaves nothing behind once it is cleared', () => {
		const draft = new IntakeDraft();
		draft.start('p1', { givenName: 'Sarah' });
		draft.visit('name');
		draft.matches = [];

		draft.clear();

		expect(draft.answers).toEqual(blankAnswers());
		expect(draft.visitedSteps).toEqual([]);
		expect(sessionStorage.getItem('doula-cloud:intake:p1')).toBeNull();
	});

	it('exports one draft the whole flow shares', () => {
		expect(intakeDraft).toBeInstanceOf(IntakeDraft);
	});
});

describe('IntakeDraft and the browser it runs in', () => {
	it('reads a draft back after a page load', () => {
		const first = new IntakeDraft();
		first.start('p1');
		first.update({ givenName: 'Sarah', addressLocality: 'Rochester' });

		const second = new IntakeDraft();
		second.start('p1');

		expect(second.answers.givenName).toBe('Sarah');
		expect(second.answers.addressLocality).toBe('Rochester');
	});

	it('fills in anything an older stored draft did not carry', () => {
		sessionStorage.setItem('doula-cloud:intake:p1', JSON.stringify({ givenName: 'Sarah' }));

		const draft = new IntakeDraft();
		draft.start('p1');

		expect(draft.answers).toEqual({ ...blankAnswers(), givenName: 'Sarah' });
	});

	// A browser with site data refused throws on access rather than
	// returning null, on every one of the three calls.
	it('works when the browser refuses to store anything', () => {
		vi.spyOn(Storage.prototype, 'getItem').mockImplementation(denied);
		vi.spyOn(Storage.prototype, 'setItem').mockImplementation(denied);
		vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(denied);

		const draft = new IntakeDraft();
		draft.start('p1', { givenName: 'Sarah' });
		draft.update({ email: 'sarah@example.com' });
		draft.clear();

		expect(draft.answers).toEqual(blankAnswers());
	});

	it('ignores a stored draft that is not readable', () => {
		sessionStorage.setItem('doula-cloud:intake:p1', 'not json');

		const draft = new IntakeDraft();
		draft.start('p1');

		expect(draft.answers).toEqual(blankAnswers());
	});
});
