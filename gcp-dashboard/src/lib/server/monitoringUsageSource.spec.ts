import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createMonitoringUsageSource } from './monitoringUsageSource.ts';
import {
	buildUsageRequest,
	CLOUD_RUN_SCOPE,
	CLOUD_SQL_SCOPE,
	CLOUD_STORAGE_SCOPE,
	FIREBASE_HOSTING_SCOPE,
	FIRESTORE_SCOPE,
	type UsageScope
} from './usageQuery.ts';

// The order the source reads them in, which is the order the panels are
// written in and the order the requests are expected to arrive.
const EVERY_SCOPE: readonly UsageScope[] = [
	CLOUD_RUN_SCOPE,
	CLOUD_SQL_SCOPE,
	CLOUD_STORAGE_SCOPE,
	FIRESTORE_SCOPE,
	FIREBASE_HOSTING_SCOPE
];

const EMPTY_SNAPSHOT = {
	cloudRun: {},
	cloudSql: {},
	cloudStorage: {},
	firestore: {},
	firebaseHosting: {}
};

const constructed = vi.fn();
const listTimeSeries = vi.fn();

// Stubbing the client is what keeps this spec off real GCP: nothing here
// authenticates, and no query leaves the process.
vi.mock('@google-cloud/monitoring', () => ({
	MetricServiceClient: class {
		listTimeSeries = listTimeSeries;

		constructor(...arguments_: unknown[]) {
			constructed(...arguments_);
		}
	}
}));

const readAt = new Date('2026-09-08T04:30:24Z');
const now = () => readAt;

function noSeries() {
	return [[]];
}

function double(value: number) {
	return [[{ points: [{ value: { doubleValue: value } }] }]];
}

function int64(value: string) {
	return [[{ points: [{ value: { int64Value: value } }] }]];
}

describe('createMonitoringUsageSource', () => {
	beforeEach(() => {
		constructed.mockClear();
		listTimeSeries.mockReset();
		listTimeSeries.mockResolvedValue(noSeries());
	});

	it('builds the client with nothing at all, so it discovers ADC on its own', () => {
		createMonitoringUsageSource();

		expect(constructed).toHaveBeenCalledWith();
	});

	it("asks Monitoring for every panel's metrics over the billing period", async () => {
		await createMonitoringUsageSource(now)();

		expect(listTimeSeries.mock.calls.map(([request]) => request)).toEqual(
			EVERY_SCOPE.flatMap((scope) =>
				scope.metrics.map((metric) => buildUsageRequest(scope, metric, readAt))
			)
		);
	});

	it('reads the DOUBLE the allocation-time metrics report', async () => {
		listTimeSeries.mockResolvedValue(double(620_750.6));

		const usage = await createMonitoringUsageSource(now)();

		expect(usage.cloudRun.billableInstanceTime).toBe(620_750.6);
	});

	it('reads the INT64 request count Monitoring hands back as a string', async () => {
		listTimeSeries.mockResolvedValue(int64('4785'));

		const usage = await createMonitoringUsageSource(now)();

		expect(usage.cloudRun.requestCount).toBe(4785);
	});

	it('reads the disk quota, which is an INT64 count of bytes', async () => {
		listTimeSeries.mockResolvedValue(int64('10464022528'));

		const usage = await createMonitoringUsageSource(now)();

		expect(usage.cloudSql.diskQuotaBytes).toBe(10_464_022_528);
	});

	it('leaves a metric out rather than calling an unreported one zero', async () => {
		const usage = await createMonitoringUsageSource(now)();

		expect(usage).toMatchObject(EMPTY_SNAPSHOT);
	});

	it('leaves it out when the series came back without a point too', async () => {
		listTimeSeries.mockResolvedValue([[{ points: [] }]]);

		const usage = await createMonitoringUsageSource(now)();

		expect(usage).toMatchObject(EMPTY_SNAPSHOT);
	});

	it('leaves it out when the point carries no value at all', async () => {
		listTimeSeries.mockResolvedValue([[{ points: [{}] }]]);

		const usage = await createMonitoringUsageSource(now)();

		expect(usage).toMatchObject(EMPTY_SNAPSHOT);
	});

	it('reads the DOUBLE Cloud Storage reports its stored bytes as', async () => {
		listTimeSeries.mockResolvedValue(double(567_149.5));

		const usage = await createMonitoringUsageSource(now)();

		expect(usage.cloudStorage.storedBytes).toBe(567_149.5);
	});

	it('reads the INT64 egress counter Monitoring hands back as a string', async () => {
		listTimeSeries.mockResolvedValue(int64('918273'));

		const usage = await createMonitoringUsageSource(now)();

		expect(usage.cloudStorage.sentBytes).toBe(918_273);
	});

	it('reads all three Firestore document counters', async () => {
		listTimeSeries.mockResolvedValue(int64('42'));

		const usage = await createMonitoringUsageSource(now)();

		expect(usage.firestore).toEqual({
			documentReads: 42,
			documentWrites: 42,
			documentDeletes: 42
		});
	});

	it('reads the bytes Firebase Hosting has served this billing period', async () => {
		listTimeSeries.mockResolvedValue(int64('9467734'));

		const usage = await createMonitoringUsageSource(now)();

		expect(usage.firebaseHosting.sentBytes).toBe(9_467_734);
	});

	it('says which window the figures cover, so they do not read as the cost period', async () => {
		const usage = await createMonitoringUsageSource(now)();

		expect(usage).toMatchObject({
			since: '2026-09-01T00:00:00.000Z',
			through: '2026-09-08T04:30:24.000Z'
		});
	});

	it('reads the wall clock when it was given no other clock', async () => {
		const before = Date.now();

		const usage = await createMonitoringUsageSource()();

		expect(Date.parse(usage.through)).toBeGreaterThanOrEqual(before);
	});
});
