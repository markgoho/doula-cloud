import { describe, expect, it } from 'vitest';
import {
	NOT_REPORTED,
	formatBytes,
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

describe('formatBytes', () => {
	it('leaves a small count in bytes rather than rounding it away', () => {
		expect(formatBytes(512)).toEqual({ value: '512', unit: 'B' });
	});

	it('steps up to the next unit at the boundary', () => {
		expect(formatBytes(1024)).toEqual({ value: '1', unit: 'KiB' });
	});

	it('renders what the buckets hold in a unit that reads as more than zero', () => {
		expect(formatBytes(567_149)).toEqual({ value: '553.9', unit: 'KiB' });
	});

	it('renders bytes served in mebibytes', () => {
		expect(formatBytes(9_467_734)).toEqual({ value: '9', unit: 'MiB' });
	});

	it('renders a disk-sized count in gibibytes', () => {
		expect(formatBytes(10_464_022_528)).toEqual({ value: '9.7', unit: 'GiB' });
	});

	it('stops at the largest unit it knows rather than inventing one', () => {
		expect(formatBytes(1024 ** 6)).toEqual({ value: '1,048,576', unit: 'TiB' });
	});

	it('says so, with no unit, when Cloud Monitoring reported no such metric', () => {
		expect(formatBytes(undefined)).toEqual({ value: NOT_REPORTED, unit: '' });
	});
});
