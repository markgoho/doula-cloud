import { BigQuery } from '@google-cloud/bigquery';
import {
	BILLING_EXPORT_LOCATION,
	BILLING_PROJECT_ID,
	buildCostQuery,
	type CostQueryRow
} from './costQuery.ts';

/**
 * Runs the billing-export query and hands back its rows.
 */
export type CostSource = () => Promise<readonly CostQueryRow[]>;

/**
 * A cost source backed by the real BigQuery billing export.
 *
 * The client is given a project to bill the query to and nothing else: no
 * credentials block, because it discovers Application Default Credentials on
 * its own, which is the only auth this tool has. There is no service account
 * and no key file — see README.md.
 *
 * `now` is a parameter so a spec can pin the billing period it asks for.
 */
export function createBigQueryCostSource(now: () => Date = () => new Date()): CostSource {
	const bigQuery = new BigQuery({ projectId: BILLING_PROJECT_ID });

	return async () => {
		const [rows] = await bigQuery.query({
			query: buildCostQuery(now()),
			location: BILLING_EXPORT_LOCATION
		});

		return rows as CostQueryRow[];
	};
}
