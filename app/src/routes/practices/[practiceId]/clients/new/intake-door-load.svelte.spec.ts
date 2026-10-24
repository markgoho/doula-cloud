import { isRedirect } from '@sveltejs/kit';
import { beforeEach, describe, expect, it } from 'vitest';
import { blankAnswers, intakeDraft } from '#lib/intakeDraft.svelte.js';
import { handOffToIntake } from '#lib/intakeHandoff.js';
import { load } from './+page.js';
import { practiceId } from './intakeFixture.js';

/**
Runs the door, and returns where it sent the reader.
*/
function enter(search = ''): string {
	const url = new URL(`https://example.test/practices/${practiceId}/clients/new${search}`);
	try {
		load({ params: { practiceId }, url } as unknown as Parameters<typeof load>[0]);
	} catch (error) {
		if (isRedirect(error)) return error.location;
		throw error;
	}
	throw new Error('the intake door did not redirect');
}

beforeEach(() => {
	sessionStorage.clear();
	intakeDraft.clear();
});

describe('the intake door, clients/new/+page.ts', () => {
	// #1829: the search's fields reach intake in module memory, so the
	// URL the door sends the reader to carries none of them.
	it('seeds the draft with what the search handed over, and puts none of it in the URL', () => {
		handOffToIntake(practiceId, {
			name: 'Ana',
			dateOfBirth: '1990-01-02',
			email: 'ana@example.com',
			phone: '555-0100'
		});

		const location = enter();

		expect(location).toBe(`/practices/${practiceId}/clients/new/name`);
		expect(intakeDraft.answers).toMatchObject({
			givenName: 'Ana',
			dateOfBirth: '1990-01-02',
			email: 'ana@example.com',
			phone: '555-0100'
		});
	});

	// The second Client of the day is not the first one again.
	it('opens a blank draft when nothing was handed over', () => {
		intakeDraft.scope = practiceId;
		intakeDraft.update({ givenName: 'Sarah' });

		enter();

		expect(intakeDraft.answers).toEqual(blankAnswers());
	});

	// `from` names a screen, not a Client, so it still rides the URL.
	it('keeps the screen that opened intake in the URL', () => {
		expect(enter('?from=overview')).toBe(`/practices/${practiceId}/clients/new/name?from=overview`);
	});
});
