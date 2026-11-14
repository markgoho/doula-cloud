import { describe, expect, it } from 'vitest';
import { typedIn } from './typedIn.js';

function fieldHolding(value: string) {
	const field = document.createElement('div');
	const input = document.createElement('input');
	input.value = value;
	field.append(input);
	return field;
}

describe('typedIn', () => {
	it('reads the input as it is drawn, trimmed, and an emptied one is empty', () => {
		expect(typedIn(fieldHolding('  Camille '), 'Old')).toBe('Camille');
		expect(typedIn(fieldHolding(''), 'Old')).toBe('');
	});

	it('keeps the held value where there is no input to read', () => {
		expect(typedIn(undefined, ' Old ')).toBe('Old');
		expect(typedIn(document.createElement('div'), 'Old')).toBe('Old');
	});
});
