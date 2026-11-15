import { describe, expect, it } from 'vitest';
import { doulaWords } from './doulaNames.js';

describe('doulaWords', () => {
	it('names one Doula whole', () => {
		expect(doulaWords(['Joss Adeyemi'], ['a'], 'z')).toEqual({ full: 'Joss Adeyemi', lead: 'Joss Adeyemi', more: 0 });
	});

	it('calls the reader "You", and puts her first', () => {
		expect(doulaWords(['Joss Adeyemi', 'Lena Vasquez'], ['a', 'b'], 'b')).toEqual({
			full: 'You, Joss Adeyemi',
			lead: 'You',
			more: 1
		});
	});

	it('draws both names where two fit', () => {
		expect(doulaWords(['Jo Ames', 'Li Cho'], ['a', 'b'], 'z')).toEqual({ full: 'Jo Ames, Li Cho', lead: 'Jo Ames, Li Cho', more: 0 });
	});

	it('draws the first name and a count where they do not fit', () => {
		expect(doulaWords(['Joss Adeyemi', 'Lena Vasquez', 'Priya Raman'], ['a', 'b', 'c'])).toEqual({
			full: 'Joss Adeyemi, Lena Vasquez, Priya Raman',
			lead: 'Joss Adeyemi',
			more: 2
		});
	});

	it('names no one for no Doula', () => {
		expect(doulaWords([])).toEqual({ full: '', lead: '', more: 0 });
	});

	it('leaves the reader out when no id says she is one', () => {
		expect(doulaWords(['Joss Adeyemi'], [], 'b').full).toBe('Joss Adeyemi');
	});
});
