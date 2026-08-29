import { describe, expect, it } from 'vitest';
import {
	WORK_STATES,
	WORK_STATE_NAMES,
	workStateCode,
	workStateName,
	workStateReportedOn
} from './workStates.js';

describe('workStates', () => {
	it('carries the 50 states and the District of Columbia', () => {
		expect(WORK_STATES).toHaveLength(51);
	});

	it('names every state in the same order as its codes', () => {
		expect(WORK_STATE_NAMES).toHaveLength(WORK_STATES.length);
		expect(WORK_STATE_NAMES[0]).toBe('Alabama');
	});

	// The API stores the USPS code (migration 00043's CHECK constraint), so
	// the dropdown's full name has to convert before it is sent.
	it('converts a full state name to its USPS code', () => {
		expect(workStateCode('New York')).toBe('NY');
	});

	// An empty select is the untouched form: the server refuses it rather
	// than the browser silently sending something.
	it('returns an empty code for anything that is not a state', () => {
		expect(workStateCode('Ontario')).toBe('');
	});

	// The screens that read a recorded state back out -- the roster, the
	// account screen, the Invitation acceptance -- have a code and need a
	// name, which is the opposite direction from the dropdown's.
	it('converts a USPS code to its full state name', () => {
		expect(workStateName('NY')).toBe('New York');
	});

	// Shown as it came rather than blanked: a value this list does not
	// know is how anyone finds out the API's list and ours have drifted.
	it('shows an unknown code as itself rather than nothing', () => {
		expect(workStateName('ZZ')).toBe('ZZ');
	});

	// The date is the only staleness signal the design has, so every
	// screen that shows it has to show it the same way.
	it('formats the day a work state was reported', () => {
		expect(workStateReportedOn('2026-08-28T14:02:11Z')).toBe(
			new Date('2026-08-28T14:02:11Z').toLocaleDateString(undefined, {
				year: 'numeric',
				month: 'short',
				day: 'numeric'
			})
		);
	});
});
