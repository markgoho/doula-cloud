/**
 * The billing export this dashboard reads, and the one query it runs.
 *
 * Nothing here names a GCP service. The `GROUP BY` decides what the
 * dashboard shows, so a newly-enabled service appears with no code change.
 */

import { billingPeriodMonth } from './billingPeriod.ts';

/**
 * The GCP project that owns the billing-export dataset, and the project
 * BigQuery bills the query itself to. Application Default Credentials carry
 * no project of their own, so the client has to be told this one.
 */
export const BILLING_PROJECT_ID = 'doula-cloud';

/**
 * The GCP project whose spend this dashboard reports on: `doula-cloud`. The
 * export records the number, not the id, so the filter uses the number.
 */
export const BILLING_PROJECT_NUMBER = '850855848778';

/**
 * BigQuery region of the `doula-cloud:billing_export` dataset. A query
 * against it fails with "dataset not found" unless the job is submitted to
 * the same region, so this travels with every request.
 */
export const BILLING_EXPORT_LOCATION = 'us-central1';

/**
 * The Standard usage cost export table, matched by wildcard. The real table
 * carries the billing account id in its name
 * (`gcp_billing_export_v1_01873B_A8A1B5_E62BC2`); the wildcard keeps that id
 * out of the repository and out of the way of a billing-account change.
 */
export const BILLING_EXPORT_TABLE = 'doula-cloud.billing_export.gcp_billing_export_v1_*';

/**
 * One `service.description` + `sku.description` pair, as BigQuery returns it.
 */
export interface CostQueryRow {
	readonly service: string;
	readonly sku: string;
	readonly cost: number;
	/**
	 * Latest usage instant this row accounts for. A TIMESTAMP arrives wrapped
	 * in `{ value }`; SQL NULL arrives as `null`, which the type has to admit.
	 */
	readonly latestUsage: string | { readonly value: string } | null | undefined;
}

/**
 * Cost per service/SKU for the billing period `now` falls in.
 *
 * `invoice.month` (a `YYYYMM` string) is the billing period itself, rather
 * than a date range over `usage_start_time`: it is what the invoice will be
 * cut from, and it keeps a late-arriving line item in the period it belongs
 * to. `project.number` is a STRING column in the Standard export schema, so
 * the comparison value is quoted.
 *
 * `invoice.month` is read off `now` through {@link billingPeriodMonth} rather
 * than asked of BigQuery's own `CURRENT_DATE()`, which defaults to UTC and
 * would name the wrong month for the first several hours of every UTC month
 * — GCP assigns `invoice.month` in `America/Los_Angeles`, not UTC. This is
 * the same conversion `startOfBillingPeriod` (in `usageQuery.ts`) opens the
 * usage window with, so the two figures cannot drift apart. See
 * [#1174](https://github.com/markgoho/doula-cloud/issues/1174).
 */
export function buildCostQuery(now: Date): string {
	return `SELECT
  service.description AS service,
  sku.description AS sku,
  SUM(cost) AS cost,
  MAX(usage_end_time) AS latestUsage
FROM \`${BILLING_EXPORT_TABLE}\`
WHERE project.number = '${BILLING_PROJECT_NUMBER}'
  AND invoice.month = '${billingPeriodMonth(now)}'
GROUP BY service, sku
ORDER BY cost DESC`;
}
