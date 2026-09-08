/**
 * The usage this dashboard pairs with the bill, and the Cloud Monitoring
 * requests that fetch it.
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
 * The monitored resource Cloud SQL reports under, and the one instance in it.
 *
 * `cloudsql_database` carries no `instance_id` label: it identifies an
 * instance with `database_id`, which is `project:instance`. So the instance
 * this dashboard watches — `doula-cloud-pg` — is named in that qualified
 * form. Read off the live resource on 2026-09-08, not assumed.
 */
export const CLOUD_SQL_RESOURCE_TYPE = 'cloudsql_database';
export const CLOUD_SQL_DATABASE_ID = `${MONITORING_PROJECT_ID}:doula-cloud-pg`;

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
	readonly perSeriesAligner: 'ALIGN_SUM' | 'ALIGN_MEAN' | 'ALIGN_MAX';
	readonly crossSeriesReducer: 'REDUCE_SUM' | 'REDUCE_MEAN' | 'REDUCE_MAX';
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
 * The peak a level reached over the period, rather than its average.
 *
 * The one alignment a metric may ask for instead of its kind's default, and
 * only a GAUGE may ask: taking the maximum of a DELTA counter's samples
 * reports the busiest slice of the period rather than the period.
 *
 * It is not the GAUGE default because most level metrics bill on the area
 * under them — a byte-second charge is the mean size times the period, and a
 * peak would overstate it. A provisioned quota is the exception: it is a step
 * function, so the mean understates what was actually provisioned if it grew
 * mid-period, while the maximum is the size being paid for.
 */
export const GAUGE_PEAK: Alignment = {
	perSeriesAligner: 'ALIGN_MAX',
	crossSeriesReducer: 'REDUCE_MAX'
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

/**
 * The Cloud SQL figure the panel shows, keyed the way the DTO carries it.
 * Named for its unit, because Monitoring reports the quota in bytes while a
 * bill is read in GiB.
 */
export type CloudSqlMetricId = 'diskQuotaBytes';

/**
 * One metric, and how it is read.
 *
 * A union rather than one shape with an optional field, so that only a GAUGE
 * can carry the {@link GAUGE_PEAK} override: a DELTA counter has no legal
 * alignment other than the sum its kind gives it.
 */
export type UsageMetric<Id extends string = string> =
	| { readonly id: Id; readonly type: string; readonly kind: 'DELTA' }
	| {
			readonly id: Id;
			readonly type: string;
			readonly kind: 'GAUGE';
			readonly alignment?: typeof GAUGE_PEAK;
	  };

/**
 * One service's metrics and the resource they are read from.
 */
export interface UsageScope<Id extends string = string> {
	readonly resourceFilter: string;
	readonly metrics: readonly UsageMetric<Id>[];
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
export const CLOUD_RUN_SCOPE: UsageScope<CloudRunMetricId> = {
	resourceFilter: `resource.type="${CLOUD_RUN_RESOURCE_TYPE}" AND resource.labels.service_name="${CLOUD_RUN_SERVICE_NAME}"`,
	metrics: [
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
	]
};

/**
 * The one Cloud SQL metric that maps to the bill.
 *
 * Provisioned disk is what Cloud SQL storage is charged on, and this is it —
 * a GAUGE of INT64 bytes, read from the live `metricDescriptors` endpoint of
 * the `doula-cloud` project on 2026-09-08. It takes {@link GAUGE_PEAK}
 * because a quota only steps upward.
 *
 * CPU and memory utilization are deliberately absent. Cloud SQL compute bills
 * flat per instance-tier-hour, so a utilization percentage is a sizing signal
 * and not usage that produced a charge; showing one in a cost panel would say
 * the bill moves with it, which is false.
 */
export const CLOUD_SQL_SCOPE: UsageScope<CloudSqlMetricId> = {
	resourceFilter: `resource.type="${CLOUD_SQL_RESOURCE_TYPE}" AND resource.labels.database_id="${CLOUD_SQL_DATABASE_ID}"`,
	metrics: [
		{
			id: 'diskQuotaBytes',
			type: 'cloudsql.googleapis.com/database/disk/quota',
			kind: 'GAUGE',
			alignment: GAUGE_PEAK
		}
	]
};

/**
 * One pull of usage, as the panels render it.
 *
 * A metric is absent rather than zero when Monitoring returned no series for
 * it, so a panel can say "not reported" instead of claiming no usage.
 */
export interface UsageSnapshot {
	/*
	 * Start of the billing period the figures cover, ISO-8601.
	 */
	readonly since: string;
	/*
	 * The instant they were read, ISO-8601. Unlike cost, usage is current.
	 */
	readonly through: string;
	readonly cloudRun: { readonly [K in CloudRunMetricId]?: number };
	readonly cloudSql: { readonly [K in CloudSqlMetricId]?: number };
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

function alignmentOf(metric: UsageMetric): Alignment {
	return metric.kind === 'GAUGE' && metric.alignment !== undefined
		? metric.alignment
		: ALIGNMENT_BY_KIND[metric.kind];
}

/**
 * The request that collapses one metric to a single scalar for the billing
 * period.
 *
 * The alignment period is the whole period, which is what makes Monitoring
 * return one point rather than a series; the cross-series reducer folds every
 * series of the resource into that one point.
 */
export function buildUsageRequest(
	scope: UsageScope,
	metric: UsageMetric,
	now: Date
): UsageRequest {
	const since = startOfBillingPeriod(now);
	const elapsed = toEpochSeconds(now) - toEpochSeconds(since);

	return {
		name: `projects/${MONITORING_PROJECT_ID}`,
		filter: `metric.type="${metric.type}" AND ${scope.resourceFilter}`,
		interval: {
			startTime: { seconds: toEpochSeconds(since) },
			endTime: { seconds: toEpochSeconds(now) }
		},
		aggregation: {
			alignmentPeriod: { seconds: Math.max(elapsed, MINIMUM_ALIGNMENT_PERIOD_SECONDS) },
			...alignmentOf(metric)
		}
	};
}
