import { MetricServiceClient } from '@google-cloud/monitoring';
import {
	buildUsageRequest,
	CLOUD_RUN_SCOPE,
	CLOUD_SQL_SCOPE,
	CLOUD_STORAGE_SCOPE,
	FIREBASE_HOSTING_SCOPE,
	FIRESTORE_SCOPE,
	startOfBillingPeriod,
	type UsageScope,
	type UsageSnapshot
} from './usageQuery.ts';

/**
 * Runs the usage pull and hands back one scalar per metric.
 */
export type UsageSource = () => Promise<UsageSnapshot>;

/**
 * As much of a returned time series as reading one scalar needs.
 */
interface ScalarSeries {
	readonly points?: readonly { readonly value?: PointValue | null }[] | null;
}

interface PointValue {
	readonly doubleValue?: number | null;
	/*
	 * An INT64 metric — `request_count` and the Cloud SQL disk quota are both
	 * ones — does not arrive as a number. The client hands it back as a
	 * string, or as a `Long` if it was configured to, so it is read through
	 * its own text either way.
	 */
	readonly int64Value?: number | string | { toString(): string } | null;
}

function readScalar(series: readonly ScalarSeries[]): number | undefined {
	const [point] = series[0]?.points ?? [];
	const value: PointValue = point?.value ?? {};

	if (value.doubleValue != undefined) return value.doubleValue;
	if (value.int64Value != undefined) return Number(String(value.int64Value));

	// Monitoring returns no series at all for a metric with nothing to
	// report. That is not zero usage, so it stays absent.
	return undefined;
}

/**
 * Every metric in one scope, keyed as the DTO carries it. A metric Monitoring
 * had nothing to report for is left out rather than set to zero.
 */
async function readScope<Id extends string>(
	monitoring: MetricServiceClient,
	scope: UsageScope<Id>,
	readAt: Date
): Promise<{ [K in Id]?: number }> {
	const readings = await Promise.all(
		scope.metrics.map(async (metric) => {
			const [series] = await monitoring.listTimeSeries(buildUsageRequest(scope, metric, readAt));
			return [metric.id, readScalar(series)] as const;
		})
	);

	const metrics: { [K in Id]?: number } = {};
	for (const [id, scalar] of readings) {
		if (scalar !== undefined) metrics[id] = scalar;
	}

	return metrics;
}

/**
 * A usage source backed by the real Cloud Monitoring API.
 *
 * The client is built with nothing at all: no credentials block, because it
 * discovers Application Default Credentials on its own, which is the only
 * auth this tool has. There is no service account and no key file — see
 * README.md.
 *
 * `now` is a parameter so a spec can pin the billing period it asks for.
 */
export function createMonitoringUsageSource(now: () => Date = () => new Date()): UsageSource {
	const monitoring = new MetricServiceClient();

	return async () => {
		const readAt = now();

		// One sync, every service: the panels are read together so they cover
		// the same window as each other and as the cost beside them.
		const [cloudRun, cloudSql, cloudStorage, firestore, firebaseHosting] = await Promise.all([
			readScope(monitoring, CLOUD_RUN_SCOPE, readAt),
			readScope(monitoring, CLOUD_SQL_SCOPE, readAt),
			readScope(monitoring, CLOUD_STORAGE_SCOPE, readAt),
			readScope(monitoring, FIRESTORE_SCOPE, readAt),
			readScope(monitoring, FIREBASE_HOSTING_SCOPE, readAt)
		]);

		return {
			since: startOfBillingPeriod(readAt).toISOString(),
			through: readAt.toISOString(),
			cloudRun,
			cloudSql,
			cloudStorage,
			firestore,
			firebaseHosting
		};
	};
}
