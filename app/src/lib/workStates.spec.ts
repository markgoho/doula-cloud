import { describe, expect, it } from 'vitest';
import { WORK_STATES, WORK_STATE_NAMES, workStateCode } from './workStates.js';

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
});
