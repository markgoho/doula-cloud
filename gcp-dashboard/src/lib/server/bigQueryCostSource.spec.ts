import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createBigQueryCostSource } from './bigQueryCostSource.ts';
import { BILLING_EXPORT_LOCATION, BILLING_PROJECT_ID, buildCostQuery } from './costQuery.ts';

const constructed = vi.fn();
const query = vi.fn();

// Stubbing the client is what keeps this spec off real GCP: nothing here
// authenticates, and no query leaves the process.
vi.mock('@google-cloud/bigquery', () => ({
	BigQuery: class {
		query = query;

		constructor(...arguments_: unknown[]) {
			constructed(...arguments_);
		}
	}
}));

describe('createBigQueryCostSource', () => {
	beforeEach(() => {
		constructed.mockClear();
		query.mockReset();
	});

	it('builds the client with a project and no credentials, so it discovers ADC on its own', () => {
		createBigQueryCostSource();

		expect(constructed).toHaveBeenCalledWith({ projectId: BILLING_PROJECT_ID });
	});

	it('runs the cost query in the region the billing-export dataset lives in', async () => {
		query.mockResolvedValue([[]]);

		await createBigQueryCostSource()();

		expect(query).toHaveBeenCalledWith({
			query: buildCostQuery(),
			location: BILLING_EXPORT_LOCATION
		});
	});

	it('hands back the rows BigQuery returned', async () => {
		const rows = [{ service: 'Cloud Run', sku: 'CPU', cost: 1, latestUsage: undefined }];
		query.mockResolvedValue([rows]);

		await expect(createBigQueryCostSource()()).resolves.toEqual(rows);
	});
});
