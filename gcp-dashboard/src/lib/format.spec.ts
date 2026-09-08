import { describe, expect, it } from 'vitest';
import {
	NOT_REPORTED,
	formatClock,
	formatCompact,
	formatDay,
	formatGibibytes,
	formatHours,
	formatShare,
	formatUsd
} from './format.ts';

describe('formatUsd', () => {
	it('rounds a raw BigQuery cost to cents', () => {
		expect(formatUsd(6.6726)).toBe('$6.67');
	});

	it('says so when the export billed nothing for the service', () => {
		expect(formatUsd(undefined)).toBe(NOT_REPORTED);
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

describe('formatHours', () => {
	it('renders instance-seconds as the hours a bill is read in', () => {
		expect(formatHours(620_750.6)).toBe('172.4');
	});

	it('says so when Cloud Monitoring did not report the metric', () => {
		expect(formatHours(undefined)).toBe(NOT_REPORTED);
	});
});

describe('formatCompact', () => {
	it('renders a large count at a glance', () => {
		expect(formatCompact(184_300)).toBe('184.3K');
	});

	it('says so when Cloud Monitoring reported no such metric', () => {
		expect(formatCompact(undefined)).toBe(NOT_REPORTED);
	});
});

describe('formatGibibytes', () => {
	it('renders a disk quota in bytes as the GiB a bill is read in', () => {
		expect(formatGibibytes(10_464_022_528)).toBe('9.7');
	});

	it('says so when Cloud Monitoring reported no quota', () => {
		expect(formatGibibytes(undefined)).toBe(NOT_REPORTED);
	});
});
