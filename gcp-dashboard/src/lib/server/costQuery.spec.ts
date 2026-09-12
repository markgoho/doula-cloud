import { describe, expect, it } from 'vitest';
import { BILLING_EXPORT_TABLE, BILLING_PROJECT_NUMBER, buildCostQuery } from './costQuery.ts';

const now = new Date('2026-09-08T04:30:24Z');

describe('buildCostQuery', () => {
	const query = buildCostQuery(now);

	it('reads the doula-cloud billing export table', () => {
		expect(query).toContain(`\`${BILLING_EXPORT_TABLE}\``);
	});

	it('filters to the doula-cloud project number, quoted for a STRING column', () => {
		expect(query).toContain(`project.number = '${BILLING_PROJECT_NUMBER}'`);
	});

	it('scopes the read to the billing period, named in the zone GCP invoices in rather than asked of BigQuery in UTC', () => {
		expect(query).toContain("invoice.month = '202609'");
		expect(query).not.toContain('CURRENT_DATE');
	});

	it('names August for an instant seven hours into the UTC month, since Pacific is still August then', () => {
		const stillAugustInPacific = new Date('2026-09-01T03:00:00Z');

		expect(buildCostQuery(stillAugustInPacific)).toContain("invoice.month = '202608'");
	});

	it('groups by service and SKU, so no service is named in the query', () => {
		expect(query).toContain('GROUP BY service, sku');
	});

	it('carries the latest usage instant, for the freshness caveat', () => {
		expect(query).toContain('MAX(usage_end_time) AS latestUsage');
	});
});
