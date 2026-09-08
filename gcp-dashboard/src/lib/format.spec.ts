import { describe, expect, it } from 'vitest';
import { NOT_REPORTED, formatClock, formatDay, formatShare, formatUsd } from './format.ts';

describe('formatUsd', () => {
	it('rounds a raw BigQuery cost to cents', () => {
		expect(formatUsd(6.6726)).toBe('$6.67');
	});
});

describe('formatShare', () => {
	it('renders a fraction as a percentage with one decimal', () => {
		expect(formatShare(0.3852)).toBe('38.5%');
	});
});

describe('formatDay', () => {
	it('renders an instant as a short day', () => {
		expect(formatDay('2026-09-07T18:00:00Z')).toBe('Sep 7');
	});

	it('says so when the export reported no usage instant', () => {
		expect(formatDay(undefined)).toBe(NOT_REPORTED);
	});
});

describe('formatClock', () => {
	it('renders the hour and minute of a sync', () => {
		expect(formatClock(Date.parse('2026-09-07T18:04:00Z'))).toMatch(/\d{2}:04/);
	});
});
