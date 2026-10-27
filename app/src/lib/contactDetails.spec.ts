import { describe, expect, it } from 'vitest';
import { formatAddress, formatPhoneNumber } from './contactDetails';

describe('formatAddress (#2147)', () => {
	it('puts no comma between the state and the ZIP code', () => {
		expect(
			formatAddress({
				addressLine1: '12 Park Avenue',
				addressLocality: 'Rochester',
				addressRegion: 'NY',
				addressPostalCode: '14607'
			})
		).toBe('12 Park Avenue, Rochester, NY 14607');
	});

	it('keeps a second line between the first and the city', () => {
		expect(
			formatAddress({
				addressLine1: '1180 East Henrietta Road',
				addressLine2: 'Apartment 4B',
				addressLocality: 'Rochester',
				addressRegion: 'NY',
				addressPostalCode: '14623-1234'
			})
		).toBe('1180 East Henrietta Road, Apartment 4B, Rochester, NY 14623-1234');
	});

	it('trims each part and leaves no separator for an empty one', () => {
		expect(formatAddress({ addressLine1: ' 12 Park Avenue ', addressLine2: '  ', addressLocality: 'Rochester' })).toBe(
			'12 Park Avenue, Rochester'
		);
		expect(formatAddress({ addressLocality: 'Rochester', addressPostalCode: '14607' })).toBe('Rochester 14607');
		expect(formatAddress({ addressRegion: 'NY', addressPostalCode: '14607' })).toBe('NY 14607');
		expect(formatAddress({ addressLine1: '12 Park Avenue', addressPostalCode: '14607' })).toBe('12 Park Avenue, 14607');
		expect(formatAddress({ addressPostalCode: '14607' })).toBe('14607');
	});

	it('gives nothing for an address with nothing in it', () => {
		expect(formatAddress({})).toBe('');
		expect(formatAddress({ addressLine1: '', addressLocality: ' ' })).toBe('');
	});
});

describe('formatPhoneNumber (#2147)', () => {
	it('reads a US number the same way however it was typed', () => {
		for (const typed of [
			'585 555 0142',
			'5855550142',
			'585-555-0142',
			'585.555.0142',
			'(585) 555-0142',
			'(585)555-0142',
			'585/555-0142',
			'1 585 555 0142',
			'1-585-555-0142',
			'+1 585 555 0142',
			'+1 (585) 555-0142',
			'+15855550142',
			'  585 555 0142  '
		]) {
			expect(formatPhoneNumber(typed), typed).toBe('(585) 555-0142');
		}
	});

	it('shows any other number exactly as typed, trimmed', () => {
		for (const typed of [
			'+44 7911 123456',
			'07911 123456',
			'+91 98765 43210',
			'+585 555 0142',
			'585 555 0142 x12',
			'585-555-0142 ext. 12',
			'555-0142',
			'0585 555 0142',
			'(185) 555-0142',
			'(585) 155-0142',
			'2 585 555 0142',
			'585 555 01422',
			'585 + 555 0142',
			'call the office'
		]) {
			expect(formatPhoneNumber(typed), typed).toBe(typed);
		}
		expect(formatPhoneNumber('  +44 7911 123456 ')).toBe('+44 7911 123456');
	});

	it('gives nothing for nothing', () => {
		expect(formatPhoneNumber('')).toBe('');
		expect(formatPhoneNumber(' '.repeat(3))).toBe('');
	});
});
