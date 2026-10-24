import { describe, expect, it } from 'vitest';
import { handOffToIntake, takeIntakeHandoff } from './intakeHandoff.js';

const fields = { name: 'Ada', dateOfBirth: '1815-12-10', email: 'ada@example.com', phone: '555-0100' };

describe('the search-to-intake hand-off (#1829)', () => {
	it('gives the door what the search typed, once', () => {
		handOffToIntake('practice-1', fields);

		expect(takeIntakeHandoff('practice-1')).toEqual(fields);
		expect(takeIntakeHandoff('practice-1')).toBeUndefined();
	});

	it('gives nothing to a door at another Practice, and is gone afterwards', () => {
		handOffToIntake('practice-1', fields);

		expect(takeIntakeHandoff('practice-2')).toBeUndefined();
		expect(takeIntakeHandoff('practice-1')).toBeUndefined();
	});
});
