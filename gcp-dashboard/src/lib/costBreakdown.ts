import type { CostQueryRow } from './server/costQuery.ts';

/**
 * Services that bill, but publish no Cloud Monitoring metric that could be
 * paired with the bill. Their cost rows render like any other; where a usage
 * panel would go, they carry {@link USAGE_DETAIL_UNAVAILABLE_LABEL} instead,
 * so the absence reads as a known limit rather than a missing panel.
 *
 * The strings are the `service.description` values the billing export
 * actually emits — Firebase Authentication bills as "Identity Platform".
 */
export const NO_USAGE_DETAIL_SERVICES: readonly string[] = [
	'Artifact Registry',
	'Secret Manager',
	'Identity Platform'
];

/**
 * Shown in place of a usage panel for {@link NO_USAGE_DETAIL_SERVICES}.
 */
export const USAGE_DETAIL_UNAVAILABLE_LABEL = 'usage detail not available — billing export only';

/**
 * The billing export is written once a day, so the newest cost it can report
 * is roughly a day behind the usage that produced it. Stated everywhere a
 * total is, so no number here reads as spend-so-far-today.
 */
export const EXPORT_FRESHNESS_CAVEAT = 'the billing export lags usage by about 24 hours';

/**
 * How the billing export names Cloud Run, which is what pairs the Cloud Run
 * usage panel with the cost that produced it.
 */
export const CLOUD_RUN_SERVICE_DESCRIPTION = 'Cloud Run';

/**
 * One SKU's share of a service's cost.
 */
export interface SkuCost {
	readonly sku: string;
	readonly cost: number;
}

/**
 * One billed service, with the SKUs that make up its cost.
 */
export interface ServiceCost {
	readonly service: string;
	readonly cost: number;
	/**
	 * Fraction of the period total, `0` to `1`. `0` when the total is zero.
	 */
	readonly share: number;
	readonly skus: readonly SkuCost[];
	readonly usageDetailAvailable: boolean;
}

/**
 * Everything the dashboard renders from one pull of the billing export.
 */
export interface CostBreakdown {
	readonly total: number;
	readonly services: readonly ServiceCost[];
	/**
	 * Latest usage instant any row accounts for, ISO-8601. Absent when the
	 * export reported none.
	 */
	readonly costsThrough?: string;
}

function readTimestamp(value: CostQueryRow['latestUsage']): string | undefined {
	// Loose: BigQuery hands back SQL NULL as null, and an absent column as
	// undefined. Both mean the same thing here.
	if (value == undefined) return undefined;
	return typeof value === 'string' ? value : value.value;
}

/**
 * Folds the export's service/SKU rows into the shape the page renders:
 * services ordered by cost, each with its SKUs, plus the period total and
 * how current the data is.
 */
export function summarizeCost(rows: readonly CostQueryRow[]): CostBreakdown {
	const byService = new Map<string, { cost: number; skus: SkuCost[] }>();
	let total = 0;
	let costsThrough: string | undefined;

	for (const row of rows) {
		total += row.cost;

		const entry = byService.get(row.service) ?? { cost: 0, skus: [] };
		entry.cost += row.cost;
		entry.skus.push({ sku: row.sku, cost: row.cost });
		byService.set(row.service, entry);

		const latest = readTimestamp(row.latestUsage);
		if (latest !== undefined && (costsThrough === undefined || latest > costsThrough)) {
			costsThrough = latest;
		}
	}

	const services = [...byService]
		.map(([service, entry]) => ({
			service,
			cost: entry.cost,
			// A period with no spend at all would otherwise divide by zero and
			// render every bar as NaN% wide.
			share: total === 0 ? 0 : entry.cost / total,
			skus: entry.skus.toSorted((a, b) => b.cost - a.cost),
			usageDetailAvailable: !NO_USAGE_DETAIL_SERVICES.includes(service)
		}))
		.toSorted((a, b) => b.cost - a.cost);

	return { total, services, costsThrough };
}

/**
 * What one service cost this period, or `undefined` when nothing has been
 * synced yet or the export billed nothing for it. A usage panel reads this to
 * show the cost its figures produced.
 */
export function findServiceCost(
	breakdown: CostBreakdown | undefined,
	service: string
): number | undefined {
	return breakdown?.services.find((entry) => entry.service === service)?.cost;
}
