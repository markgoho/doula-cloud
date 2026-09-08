import { describe, expect, it } from 'vitest';
import {
	ALIGNMENT_BY_KIND,
	buildUsageRequest,
	CLOUD_RUN_METRICS,
	MINIMUM_ALIGNMENT_PERIOD_SECONDS,
	MONITORING_PROJECT_ID,
	startOfBillingPeriod,
	type UsageMetric
} from './usageQuery.ts';

const now = new Date('2026-09-08T04:30:24Z');
const [billableInstanceTime] = CLOUD_RUN_METRICS;

describe('CLOUD_RUN_METRICS', () => {
	it('covers the four Cloud Run figures the bill is read from', () => {
		expect(CLOUD_RUN_METRICS.map((metric) => metric.type)).toEqual([
			'run.googleapis.com/container/billable_instance_time',
			'run.googleapis.com/container/cpu/allocation_time',
			'run.googleapis.com/container/memory/allocation_time',
			'run.googleapis.com/request_count'
		]);
	});

	it('records every one of them as the DELTA counter Monitoring says it is', () => {
		expect(CLOUD_RUN_METRICS.every((metric) => metric.kind === 'DELTA')).toBe(true);
	});
});

describe('ALIGNMENT_BY_KIND', () => {
	it('sums a DELTA counter over the period', () => {
		expect(ALIGNMENT_BY_KIND.DELTA.perSeriesAligner).toBe('ALIGN_SUM');
	});

	it('averages a GAUGE instead, because summing samples of a level double-counts it', () => {
		expect(ALIGNMENT_BY_KIND.GAUGE.perSeriesAligner).toBe('ALIGN_MEAN');
	});
});

describe('startOfBillingPeriod', () => {
	it('is the first instant of the calendar month in UTC, matching invoice.month', () => {
		expect(startOfBillingPeriod(now).toISOString()).toBe('2026-09-01T00:00:00.000Z');
	});
});

describe('buildUsageRequest', () => {
	it('asks the project Application Default Credentials do not name on their own', () => {
		expect(buildUsageRequest(billableInstanceTime, now).name).toBe(
			`projects/${MONITORING_PROJECT_ID}`
		);
	});

	it('scopes the filter to one Cloud Run service, by the label the resource carries', () => {
		expect(buildUsageRequest(billableInstanceTime, now).filter).toBe(
			'metric.type="run.googleapis.com/container/billable_instance_time" AND resource.type="cloud_run_revision" AND resource.labels.service_name="doula-api"'
		);
	});

	it('covers the billing period so far', () => {
		expect(buildUsageRequest(billableInstanceTime, now).interval).toEqual({
			startTime: { seconds: Date.parse('2026-09-01T00:00:00Z') / 1000 },
			endTime: { seconds: Date.parse('2026-09-08T04:30:24Z') / 1000 }
		});
	});

	it('aligns over the whole period, so one metric collapses to one scalar', () => {
		expect(buildUsageRequest(billableInstanceTime, now).aggregation).toEqual({
			alignmentPeriod: { seconds: 621_024 },
			perSeriesAligner: 'ALIGN_SUM',
			crossSeriesReducer: 'REDUCE_SUM'
		});
	});

	it('never asks for an alignment period Monitoring would reject as too short', () => {
		const firstSecondOfTheMonth = new Date('2026-09-01T00:00:01Z');

		expect(
			buildUsageRequest(billableInstanceTime, firstSecondOfTheMonth).aggregation.alignmentPeriod
		).toEqual({ seconds: MINIMUM_ALIGNMENT_PERIOD_SECONDS });
	});

	it('aligns a GAUGE by its own kind rather than by the sum a counter gets', () => {
		const gauge: UsageMetric = {
			id: 'requestCount',
			type: 'run.googleapis.com/container/instance_count',
			kind: 'GAUGE'
		};

		expect(buildUsageRequest(gauge, now).aggregation.perSeriesAligner).toBe('ALIGN_MEAN');
	});
});
