import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createMonitoringUsageSource } from './monitoringUsageSource.ts';
import { buildUsageRequest, CLOUD_RUN_SCOPE, CLOUD_SQL_SCOPE } from './usageQuery.ts';

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

	it('asks Monitoring for every Cloud Run and Cloud SQL metric over the billing period', async () => {
		await createMonitoringUsageSource(now)();

		expect(listTimeSeries.mock.calls.map(([request]) => request)).toEqual([
			...CLOUD_RUN_SCOPE.metrics.map((metric) =>
				buildUsageRequest(CLOUD_RUN_SCOPE, metric, readAt)
			),
			...CLOUD_SQL_SCOPE.metrics.map((metric) => buildUsageRequest(CLOUD_SQL_SCOPE, metric, readAt))
		]);
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

		expect(usage).toMatchObject({ cloudRun: {}, cloudSql: {} });
	});

	it('leaves it out when the series came back without a point too', async () => {
		listTimeSeries.mockResolvedValue([[{ points: [] }]]);

		const usage = await createMonitoringUsageSource(now)();

		expect(usage).toMatchObject({ cloudRun: {}, cloudSql: {} });
	});

	it('leaves it out when the point carries no value at all', async () => {
		listTimeSeries.mockResolvedValue([[{ points: [{}] }]]);

		const usage = await createMonitoringUsageSource(now)();

		expect(usage).toMatchObject({ cloudRun: {}, cloudSql: {} });
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
