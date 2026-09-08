import { describe, expect, it } from 'vitest';
import {
	CLOUD_RUN_SERVICE_DESCRIPTION,
	findServiceCost,
	NO_USAGE_DETAIL_SERVICES,
	summarizeCost
} from './costBreakdown.ts';
import type { CostQueryRow } from './server/costQuery.ts';

function row(partial: Partial<CostQueryRow> & { service: string; cost: number }): CostQueryRow {
	return { sku: 'a sku', latestUsage: undefined, ...partial };
}

describe('summarizeCost', () => {
	it('totals every row it is given', () => {
		const breakdown = summarizeCost([
			row({ service: 'Cloud Run', cost: 6.5 }),
			row({ service: 'Cloud SQL', cost: 3.5 })
		]);

		expect(breakdown.total).toBe(10);
	});

	it('folds a service’s SKUs together, biggest SKU first', () => {
		const breakdown = summarizeCost([
			row({ service: 'Cloud Run', sku: 'CPU', cost: 1 }),
			row({ service: 'Cloud Run', sku: 'Memory', cost: 3 })
		]);

		expect(breakdown.services).toHaveLength(1);
		expect(breakdown.services[0].cost).toBe(4);
		expect(breakdown.services[0].skus).toEqual([
			{ sku: 'Memory', cost: 3 },
			{ sku: 'CPU', cost: 1 }
		]);
	});

	it('orders services by cost and gives each its share of the total', () => {
		const breakdown = summarizeCost([
			row({ service: 'Secret Manager', cost: 1 }),
			row({ service: 'Cloud Run', cost: 3 })
		]);

		expect(breakdown.services.map((service) => service.service)).toEqual([
			'Cloud Run',
			'Secret Manager'
		]);
		expect(breakdown.services[0].share).toBe(0.75);
	});

	it('reports a zero share rather than NaN when nothing was billed', () => {
		const breakdown = summarizeCost([row({ service: 'BigQuery', cost: 0 })]);

		expect(breakdown.services[0].share).toBe(0);
	});

	it('has no services and no freshness date for an empty export', () => {
		expect(summarizeCost([])).toEqual({ total: 0, services: [], costsThrough: undefined });
	});

	it('reports the latest usage instant across every row', () => {
		const breakdown = summarizeCost([
			row({ service: 'Cloud Run', cost: 1, latestUsage: { value: '2026-09-06T00:00:00Z' } }),
			row({ service: 'Cloud SQL', cost: 1, latestUsage: '2026-09-07T00:00:00Z' }),
			// eslint-disable-next-line unicorn/no-null -- BigQuery reports SQL NULL as null
			row({ service: 'BigQuery', cost: 0, latestUsage: null })
		]);

		expect(breakdown.costsThrough).toBe('2026-09-07T00:00:00Z');
	});

	it.each(NO_USAGE_DETAIL_SERVICES)('marks %s as having no usage detail', (service) => {
		const breakdown = summarizeCost([row({ service, cost: 1 })]);

		expect(breakdown.services[0].usageDetailAvailable).toBe(false);
	});

	it('marks a service with a Monitoring metric as having usage detail', () => {
		const breakdown = summarizeCost([row({ service: 'Cloud Run', cost: 1 })]);

		expect(breakdown.services[0].usageDetailAvailable).toBe(true);
	});
});

describe('findServiceCost', () => {
	it('finds what the service the usage panel reports on actually cost', () => {
		const breakdown = summarizeCost([
			row({ service: 'Cloud Run', cost: 18.4 }),
			row({ service: 'Cloud SQL', cost: 14.1 })
		]);

		expect(findServiceCost(breakdown, CLOUD_RUN_SERVICE_DESCRIPTION)).toBe(18.4);
	});

	it('reports nothing for a service this period did not bill for', () => {
		const breakdown = summarizeCost([row({ service: 'Cloud SQL', cost: 14.1 })]);

		expect(findServiceCost(breakdown, CLOUD_RUN_SERVICE_DESCRIPTION)).toBeUndefined();
	});

	it('reports nothing before the first sync, when there is no breakdown at all', () => {
		expect(findServiceCost(undefined, CLOUD_RUN_SERVICE_DESCRIPTION)).toBeUndefined();
	});
});
