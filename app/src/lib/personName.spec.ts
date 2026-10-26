import { describe, expect, it } from 'vitest';
import { firstName, fullName } from './personName.js';

describe('firstName', () => {
	it('reads the preferred name when there is one', () => {
		expect(firstName({ givenName: 'Rebecca', preferredName: 'Bex', familyName: 'Okafor' })).toBe('Bex');
	});

	it('falls back to the given name when there is no preferred name', () => {
		expect(firstName({ givenName: 'Amara', preferredName: '', familyName: 'Okafor' })).toBe('Amara');
	});
});

describe('fullName', () => {
	it('puts the family name after a preferred name', () => {
		expect(fullName({ givenName: 'Rebecca', preferredName: 'Bex', familyName: 'Okafor' })).toBe('Bex Okafor');
	});

	it('puts the family name after the given name when there is no preferred name', () => {
		expect(fullName({ givenName: 'Amara', preferredName: undefined, familyName: 'Okafor' })).toBe('Amara Okafor');
	});

	it('is the first name alone when there is no family name', () => {
		expect(fullName({ givenName: 'Amara' })).toBe('Amara');
	});
});
