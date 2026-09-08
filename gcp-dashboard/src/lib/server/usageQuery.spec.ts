import { describe, expect, it } from 'vitest';
import {
	ALIGNMENT_BY_KIND,
	buildUsageRequest,
	CLOUD_RUN_SCOPE,
	CLOUD_SQL_SCOPE,
	GAUGE_PEAK,
	MINIMUM_ALIGNMENT_PERIOD_SECONDS,
	MONITORING_PROJECT_ID,
	startOfBillingPeriod,
	type UsageMetric
} from './usageQuery.ts';

const now = new Date('2026-09-08T04:30:24Z');
const [billableInstanceTime] = CLOUD_RUN_SCOPE.metrics;
const [diskQuota] = CLOUD_SQL_SCOPE.metrics;

describe('CLOUD_RUN_SCOPE', () => {
	it('covers the four Cloud Run figures the bill is read from', () => {
		expect(CLOUD_RUN_SCOPE.metrics.map((metric) => metric.type)).toEqual([
			'run.googleapis.com/container/billable_instance_time',
			'run.googleapis.com/container/cpu/allocation_time',
			'run.googleapis.com/container/memory/allocation_time',
			'run.googleapis.com/request_count'
		]);
	});

	it('records every one of them as the DELTA counter Monitoring says it is', () => {
		expect(CLOUD_RUN_SCOPE.metrics.every((metric) => metric.kind === 'DELTA')).toBe(true);
	});
});

describe('CLOUD_SQL_SCOPE', () => {
	it('reads provisioned disk and nothing else, because that is the billing input', () => {
		expect(CLOUD_SQL_SCOPE.metrics.map((metric) => metric.type)).toEqual([
			'cloudsql.googleapis.com/database/disk/quota'
		]);
	});

	it('shows no CPU or memory utilization, which bills flat and is not usage', () => {
		const utilization = CLOUD_SQL_SCOPE.metrics.filter((metric) =>
			metric.type.includes('utilization')
		);

		expect(utilization).toEqual([]);
	});

	it('records the quota as the GAUGE Monitoring says it is', () => {
		expect(diskQuota.kind).toBe('GAUGE');
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
		expect(buildUsageRequest(CLOUD_RUN_SCOPE, billableInstanceTime, now).name).toBe(
			`projects/${MONITORING_PROJECT_ID}`
		);
	});

	it('scopes the filter to one Cloud Run service, by the label the resource carries', () => {
		expect(buildUsageRequest(CLOUD_RUN_SCOPE, billableInstanceTime, now).filter).toBe(
			'metric.type="run.googleapis.com/container/billable_instance_time" AND resource.type="cloud_run_revision" AND resource.labels.service_name="doula-api"'
		);
	});

	it('scopes the filter to one Cloud SQL instance, which the resource names project:instance', () => {
		expect(buildUsageRequest(CLOUD_SQL_SCOPE, diskQuota, now).filter).toBe(
			'metric.type="cloudsql.googleapis.com/database/disk/quota" AND resource.type="cloudsql_database" AND resource.labels.database_id="doula-cloud:doula-cloud-pg"'
		);
	});

	it('covers the billing period so far', () => {
		expect(buildUsageRequest(CLOUD_RUN_SCOPE, billableInstanceTime, now).interval).toEqual({
			startTime: { seconds: Date.parse('2026-09-01T00:00:00Z') / 1000 },
			endTime: { seconds: Date.parse('2026-09-08T04:30:24Z') / 1000 }
		});
	});

	it('aligns over the whole period, so one metric collapses to one scalar', () => {
		expect(buildUsageRequest(CLOUD_RUN_SCOPE, billableInstanceTime, now).aggregation).toEqual({
			alignmentPeriod: { seconds: 621_024 },
			perSeriesAligner: 'ALIGN_SUM',
			crossSeriesReducer: 'REDUCE_SUM'
		});
	});

	it('never asks for an alignment period Monitoring would reject as too short', () => {
		const firstSecondOfTheMonth = new Date('2026-09-01T00:00:01Z');

		expect(
			buildUsageRequest(CLOUD_RUN_SCOPE, billableInstanceTime, firstSecondOfTheMonth).aggregation
				.alignmentPeriod
		).toEqual({ seconds: MINIMUM_ALIGNMENT_PERIOD_SECONDS });
	});

	it('aligns a GAUGE by its own kind rather than by the sum a counter gets', () => {
		const gauge: UsageMetric = {
			id: 'instanceCount',
			type: 'run.googleapis.com/container/instance_count',
			kind: 'GAUGE'
		};

		expect(buildUsageRequest(CLOUD_RUN_SCOPE, gauge, now).aggregation).toMatchObject({
			perSeriesAligner: 'ALIGN_MEAN',
			crossSeriesReducer: 'REDUCE_MEAN'
		});
	});

	it('takes the peak instead when a GAUGE asks for it, as a stepped quota does', () => {
		expect(buildUsageRequest(CLOUD_SQL_SCOPE, diskQuota, now).aggregation).toMatchObject(
			GAUGE_PEAK
		);
	});
});
