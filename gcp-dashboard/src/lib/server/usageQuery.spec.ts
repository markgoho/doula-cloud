import { describe, expect, it } from 'vitest';
import {
	ALIGNMENT_BY_KIND,
	buildUsageRequest,
	CLOUD_RUN_SCOPE,
	CLOUD_SQL_SCOPE,
	CLOUD_STORAGE_SCOPE,
	FIREBASE_HOSTING_SCOPE,
	FIRESTORE_SCOPE,
	GAUGE_PEAK,
	GAUGE_TOTAL,
	MINIMUM_ALIGNMENT_PERIOD_SECONDS,
	MONITORING_PROJECT_ID,
	type UsageMetric
} from './usageQuery.ts';

const now = new Date('2026-09-08T04:30:24Z');
const [billableInstanceTime] = CLOUD_RUN_SCOPE.metrics;
const [diskQuota] = CLOUD_SQL_SCOPE.metrics;
const [storedBytes, sentBytes] = CLOUD_STORAGE_SCOPE.metrics;
const [hostingSentBytes] = FIREBASE_HOSTING_SCOPE.metrics;

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

describe('CLOUD_STORAGE_SCOPE', () => {
	it('reads what is stored and what left, which is what storage is charged on', () => {
		expect(CLOUD_STORAGE_SCOPE.metrics.map((metric) => metric.type)).toEqual([
			'storage.googleapis.com/storage/total_bytes',
			'storage.googleapis.com/network/sent_bytes_count'
		]);
	});

	it('shows no received bytes, because ingress is not billed', () => {
		const ingress = CLOUD_STORAGE_SCOPE.metrics.filter((metric) =>
			metric.type.includes('received')
		);

		expect(ingress).toEqual([]);
	});

	it('names no bucket, because every bucket in the project produces the bill', () => {
		expect(CLOUD_STORAGE_SCOPE.resourceFilter).toBe('resource.type="gcs_bucket"');
	});

	it('adds the buckets up rather than reporting the average one', () => {
		expect(storedBytes).toMatchObject({ kind: 'GAUGE', alignment: GAUGE_TOTAL });
	});

	it('records egress as the DELTA counter Monitoring says it is', () => {
		expect(sentBytes.kind).toBe('DELTA');
	});
});

describe('FIRESTORE_SCOPE', () => {
	it('reads the three document counters Firestore charges per operation', () => {
		expect(FIRESTORE_SCOPE.metrics.map((metric) => metric.type)).toEqual([
			'firestore.googleapis.com/document/read_count',
			'firestore.googleapis.com/document/write_count',
			'firestore.googleapis.com/document/delete_count'
		]);
	});

	it('records every one of them as the DELTA counter Monitoring says it is', () => {
		expect(FIRESTORE_SCOPE.metrics.every((metric) => metric.kind === 'DELTA')).toBe(true);
	});

	it('names no database, because project_id is the only label the resource carries', () => {
		expect(FIRESTORE_SCOPE.resourceFilter).toBe('resource.type="firestore_instance"');
	});
});

describe('FIREBASE_HOSTING_SCOPE', () => {
	it('reads bytes served and nothing else, because that is what Hosting charges', () => {
		expect(FIREBASE_HOSTING_SCOPE.metrics.map((metric) => metric.type)).toEqual([
			'firebasehosting.googleapis.com/network/sent_bytes_count'
		]);
	});

	it('shows no storage figure, which Hosting reports but does not bill on', () => {
		const storage = FIREBASE_HOSTING_SCOPE.metrics.filter((metric) =>
			metric.type.includes('storage')
		);

		expect(storage).toEqual([]);
	});

	it('reads a DELTA counter and overrides nothing, so no reset can land in the period', () => {
		expect(hostingSentBytes).toEqual({
			id: 'sentBytes',
			type: 'firebasehosting.googleapis.com/network/sent_bytes_count',
			kind: 'DELTA'
		});
	});

	it('names no domain, because every domain serves bytes the Hosting bill charges for', () => {
		expect(FIREBASE_HOSTING_SCOPE.resourceFilter).toBe('resource.type="firebase_domain"');
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

	it('covers the billing period so far, from midnight Pacific rather than midnight UTC', () => {
		expect(buildUsageRequest(CLOUD_RUN_SCOPE, billableInstanceTime, now).interval).toEqual({
			startTime: { seconds: Date.parse('2026-09-01T07:00:00Z') / 1000 },
			endTime: { seconds: Date.parse('2026-09-08T04:30:24Z') / 1000 }
		});
	});

	it('aligns over the whole period, so one metric collapses to one scalar', () => {
		expect(buildUsageRequest(CLOUD_RUN_SCOPE, billableInstanceTime, now).aggregation).toEqual({
			alignmentPeriod: { seconds: 595_824 },
			perSeriesAligner: 'ALIGN_SUM',
			crossSeriesReducer: 'REDUCE_SUM'
		});
	});

	it('reads the window as August for an instant seven hours into the UTC month, since Pacific is still August then', () => {
		const stillAugustInPacific = new Date('2026-09-01T03:00:00Z');

		expect(
			buildUsageRequest(CLOUD_RUN_SCOPE, billableInstanceTime, stillAugustInPacific).interval
				.startTime
		).toEqual({ seconds: Date.parse('2026-08-01T07:00:00Z') / 1000 });
	});

	it('never asks for an alignment period Monitoring would reject as too short', () => {
		const firstSecondOfTheBillingPeriod = new Date('2026-09-01T07:00:01Z');

		expect(
			buildUsageRequest(CLOUD_RUN_SCOPE, billableInstanceTime, firstSecondOfTheBillingPeriod)
				.aggregation.alignmentPeriod
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

	it('averages each bucket and adds them, so stored bytes are the estate not the average', () => {
		expect(buildUsageRequest(CLOUD_STORAGE_SCOPE, storedBytes, now).aggregation).toMatchObject(
			GAUGE_TOTAL
		);
	});

	it('sums Firebase Hosting egress per domain and across them, with no reset edge', () => {
		expect(
			buildUsageRequest(FIREBASE_HOSTING_SCOPE, hostingSentBytes, now).aggregation
		).toMatchObject(ALIGNMENT_BY_KIND.DELTA);
	});


	it('scopes the filter to every bucket in the project', () => {
		expect(buildUsageRequest(CLOUD_STORAGE_SCOPE, storedBytes, now).filter).toBe(
			'metric.type="storage.googleapis.com/storage/total_bytes" AND resource.type="gcs_bucket"'
		);
	});

	it('scopes the filter to the project Firestore instance', () => {
		const [documentReads] = FIRESTORE_SCOPE.metrics;

		expect(buildUsageRequest(FIRESTORE_SCOPE, documentReads, now).filter).toBe(
			'metric.type="firestore.googleapis.com/document/read_count" AND resource.type="firestore_instance"'
		);
	});
});
