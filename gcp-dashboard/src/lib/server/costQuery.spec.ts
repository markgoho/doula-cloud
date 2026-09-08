import { describe, expect, it } from 'vitest';
import { BILLING_EXPORT_TABLE, BILLING_PROJECT_NUMBER, buildCostQuery } from './costQuery.ts';

describe('buildCostQuery', () => {
	const query = buildCostQuery();

	it('reads the doula-cloud billing export table', () => {
		expect(query).toContain(`\`${BILLING_EXPORT_TABLE}\``);
	});

	it('filters to the doula-cloud project number, quoted for a STRING column', () => {
		expect(query).toContain(`project.number = '${BILLING_PROJECT_NUMBER}'`);
	});

	it('scopes the read to the current billing period', () => {
		expect(query).toContain("invoice.month = FORMAT_DATE('%Y%m', CURRENT_DATE())");
	});

	it('groups by service and SKU, so no service is named in the query', () => {
		expect(query).toContain('GROUP BY service, sku');
	});

	it('carries the latest usage instant, for the freshness caveat', () => {
		expect(query).toContain('MAX(usage_end_time) AS latestUsage');
	});
});
