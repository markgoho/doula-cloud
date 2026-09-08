/**
 * The Cloud Run usage this dashboard pairs with the Cloud Run bill, and the
 * Cloud Monitoring requests that fetch it.
 *
 * Every metric here is collapsed to a single scalar for the billing period,
 * so a stat sits beside the cost it produced rather than beside a chart.
 */

/**
 * The GCP project Cloud Monitoring is queried under. Application Default
 * Credentials carry no project of their own, so the request names it.
 */
export const MONITORING_PROJECT_ID = 'doula-cloud';

/**
 * The monitored resource Cloud Run reports under, and the one service in it
 * this dashboard cares about. `service_name` is the label `cloud_run_revision`
 * carries — not `service`.
 */
export const CLOUD_RUN_RESOURCE_TYPE = 'cloud_run_revision';
export const CLOUD_RUN_SERVICE_NAME = 'doula-api';

/**
 * Cloud Monitoring rejects an alignment period under a minute. The first
 * seconds of a new billing period would otherwise ask for one.
 */
export const MINIMUM_ALIGNMENT_PERIOD_SECONDS = 60;

/**
 * How a metric accumulates, which is what decides how it may be aligned. A
 * DELTA counter is summed over the period; a GAUGE is a level at an instant
 * and has to be averaged instead, because summing samples of a level counts
 * the same thing many times.
 */
export type MetricKind = 'DELTA' | 'GAUGE';

interface Alignment {
	readonly perSeriesAligner: 'ALIGN_SUM' | 'ALIGN_MEAN';
	readonly crossSeriesReducer: 'REDUCE_SUM' | 'REDUCE_MEAN';
}

/**
 * The only aligner each metric kind may be read with. Alignment is derived
 * from the kind rather than written out per metric, so a metric added later
 * cannot pick up an alignment its kind does not support.
 */
export const ALIGNMENT_BY_KIND: Record<MetricKind, Alignment> = {
	DELTA: { perSeriesAligner: 'ALIGN_SUM', crossSeriesReducer: 'REDUCE_SUM' },
	GAUGE: { perSeriesAligner: 'ALIGN_MEAN', crossSeriesReducer: 'REDUCE_MEAN' }
};

/**
 * The four Cloud Run figures the panel shows, keyed the way the DTO carries
 * them.
 */
export type CloudRunMetricId =
	| 'billableInstanceTime'
	| 'cpuAllocationTime'
	| 'memoryAllocationTime'
	| 'requestCount';

export interface UsageMetric {
	readonly id: CloudRunMetricId;
	readonly type: string;
	readonly kind: MetricKind;
}

/**
 * The Cloud Run metrics that map to the bill.
 *
 * All four are DELTA counters, so all four are summed. That is not an
 * assumption: each `metricKind` below was read from the live
 * `metricDescriptors` endpoint of the `doula-cloud` project on 2026-09-08.
 * `billable_instance_time` is the closest single proxy for what Cloud Run
 * charges; the CPU and memory allocation times are the inputs behind it, and
 * the request count is what drove them.
 */
export const CLOUD_RUN_METRICS: readonly UsageMetric[] = [
	{
		id: 'billableInstanceTime',
		type: 'run.googleapis.com/container/billable_instance_time',
		kind: 'DELTA'
	},
	{
		id: 'cpuAllocationTime',
		type: 'run.googleapis.com/container/cpu/allocation_time',
		kind: 'DELTA'
	},
	{
		id: 'memoryAllocationTime',
		type: 'run.googleapis.com/container/memory/allocation_time',
		kind: 'DELTA'
	},
	{ id: 'requestCount', type: 'run.googleapis.com/request_count', kind: 'DELTA' }
];

/**
 * One pull of Cloud Run usage, as the panel renders it.
 *
 * A metric is absent rather than zero when Monitoring returned no series for
 * it, so the panel can say "not reported" instead of claiming no traffic.
 */
export interface CloudRunUsage {
	/*
	 * Start of the billing period the figures cover, ISO-8601.
	 */
	readonly since: string;
	/*
	 * The instant they were read, ISO-8601. Unlike cost, usage is current.
	 */
	readonly through: string;
	readonly metrics: { readonly [K in CloudRunMetricId]?: number };
}

/**
 * A `listTimeSeries` request, in the shape the Monitoring client takes.
 */
export interface UsageRequest {
	readonly name: string;
	readonly filter: string;
	readonly interval: {
		readonly startTime: { readonly seconds: number };
		readonly endTime: { readonly seconds: number };
	};
	readonly aggregation: Alignment & {
		readonly alignmentPeriod: { readonly seconds: number };
	};
}

/**
 * First instant of the billing period `now` falls in, in UTC.
 *
 * The period is the calendar month, matching the `invoice.month` the cost
 * query groups by, so a usage figure and a cost figure cover the same window.
 */
export function startOfBillingPeriod(now: Date): Date {
	return new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1));
}

function toEpochSeconds(at: Date): number {
	return Math.floor(at.getTime() / 1000);
}

/**
 * The request that collapses one metric to a single scalar for the billing
 * period.
 *
 * The alignment period is the whole period, which is what makes Monitoring
 * return one point rather than a series; the cross-series reducer folds every
 * revision of the service into that one point.
 */
export function buildUsageRequest(metric: UsageMetric, now: Date): UsageRequest {
	const since = startOfBillingPeriod(now);
	const elapsed = toEpochSeconds(now) - toEpochSeconds(since);

	return {
		name: `projects/${MONITORING_PROJECT_ID}`,
		filter: `metric.type="${metric.type}" AND resource.type="${CLOUD_RUN_RESOURCE_TYPE}" AND resource.labels.service_name="${CLOUD_RUN_SERVICE_NAME}"`,
		interval: {
			startTime: { seconds: toEpochSeconds(since) },
			endTime: { seconds: toEpochSeconds(now) }
		},
		aggregation: {
			alignmentPeriod: { seconds: Math.max(elapsed, MINIMUM_ALIGNMENT_PERIOD_SECONDS) },
			...ALIGNMENT_BY_KIND[metric.kind]
		}
	};
}
