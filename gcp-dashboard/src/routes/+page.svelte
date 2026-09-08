<script lang="ts">
	import {
		CLOUD_RUN_SERVICE_DESCRIPTION,
		EXPORT_FRESHNESS_CAVEAT,
		findServiceCost,
		USAGE_DETAIL_UNAVAILABLE_LABEL
	} from '#lib/costBreakdown.js';
	import { loadDashboard } from '#lib/dashboard.js';
	import { DashboardSync } from '#lib/dashboardSync.svelte.js';
	import {
		formatClock,
		formatCompact,
		formatDay,
		formatHours,
		formatShare,
		formatUsd
	} from '#lib/format.js';

	const sync = new DashboardSync(() => loadDashboard(fetch));

	const breakdown = $derived(sync.breakdown);
	const usage = $derived(sync.usage);
	const cloudRunCost = $derived(findServiceCost(breakdown, CLOUD_RUN_SERVICE_DESCRIPTION));
	const isLoading = $derived(sync.state === 'loading');
</script>

<svelte:head><title>GCP spend — doula-cloud</title></svelte:head>

<div class="dashboard">
	<div class="layout">
		<aside class="card summary">
			<p class="eyebrow">doula-cloud</p>
			<h1>GCP spend</h1>

			{#if breakdown}
				<p class="total">{formatUsd(breakdown.total)}</p>
				<p class="caveat">
					This billing period, through {formatDay(breakdown.costsThrough)} —
					{EXPORT_FRESHNESS_CAVEAT}, so today is not in it yet.
				</p>
				<p class="caveat">
					{breakdown.services.length} service{breakdown.services.length === 1 ? '' : 's'} billed.
				</p>
			{:else}
				<p class="caveat">
					No figures yet. Sync to read the billing export — {EXPORT_FRESHNESS_CAVEAT}, so a
					total never includes today.
				</p>
			{/if}

			<hr />

			<button type="button" onclick={() => sync.sync()} disabled={isLoading}>
				{isLoading ? 'Syncing…' : (breakdown ? 'Sync again' : 'Sync now')}
			</button>

			<p class="status" aria-live="polite">
				{#if isLoading}
					<span class="spinner" aria-hidden="true"></span>Reading the billing export…
				{:else if sync.syncedAt !== undefined}
					Synced {formatClock(sync.syncedAt)}
				{:else if sync.state === 'idle'}
					Not synced yet
				{/if}
			</p>

			{#if sync.state === 'error'}
				<p class="error" role="alert">
					<strong>Sync failed.</strong>
					{sync.errorMessage}
				</p>
			{/if}
		</aside>

		<main class="main">
			<section class="card">
				<h2>Cost by service and SKU</h2>

				{#if breakdown}
					{#each breakdown.services as service (service.service)}
						<details class="service">
							<summary>
								<span class="name">{service.service}</span>
								<span class="cost">{formatUsd(service.cost)}</span>
								<span class="share">{formatShare(service.share)}</span>
								<span class="track" aria-hidden="true">
									<span class="fill" style:inline-size="{service.share * 100}%"></span>
								</span>
							</summary>

							<ul class="skus">
								{#each service.skus as sku (sku.sku)}
									<li><span>{sku.sku}</span><span class="cost">{formatUsd(sku.cost)}</span></li>
								{/each}
							</ul>

							{#if !service.usageDetailAvailable}
								<p class="unavailable">{USAGE_DETAIL_UNAVAILABLE_LABEL}</p>
							{/if}
						</details>
					{:else}
						<p class="caveat">The billing export returned nothing for this period.</p>
					{/each}

					<p class="caveat">
						This list is whatever the billing export returns — a newly-enabled service appears
						here on its own.
					</p>
				{:else}
					<p class="caveat">Sync to see where the money went.</p>
				{/if}
			</section>

			<section class="card usage">
				<h2>Cloud Run usage</h2>

				{#if usage}
					<p class="panel-cost">
						{formatUsd(cloudRunCost)}<span class="unit">billed this period</span>
					</p>

					<!-- Each stat is a term and its value, so it is a description list. The
					     term is written first, which is both what HTML requires inside a
					     `<dl>` and the order a screen reader should hear it in; the panel
					     draws the figure above its label with `column-reverse`. -->
					<dl class="stat-grid">
						<div class="stat">
							<dt class="stat-label">billable instance time</dt>
							<dd class="stat-num">
								{formatHours(usage.metrics.billableInstanceTime)}<span class="unit">hrs</span>
							</dd>
						</div>
						<div class="stat">
							<dt class="stat-label">CPU allocated</dt>
							<dd class="stat-num">
								{formatCompact(usage.metrics.cpuAllocationTime)}<span class="unit">vCPU&#8209;s</span
								>
							</dd>
						</div>
						<div class="stat">
							<dt class="stat-label">memory allocated</dt>
							<dd class="stat-num">
								{formatCompact(usage.metrics.memoryAllocationTime)}<span class="unit"
									>GiB&#8209;s</span
								>
							</dd>
						</div>
						<div class="stat">
							<dt class="stat-label">requests</dt>
							<dd class="stat-num">{formatCompact(usage.metrics.requestCount)}</dd>
						</div>
					</dl>

					<p class="caveat">
						This billing period, through {formatClock(Date.parse(usage.through))} today: Cloud
						Monitoring reports usage live. The cost beside it stops earlier, because {EXPORT_FRESHNESS_CAVEAT}.
					</p>
				{:else}
					<p class="caveat">Sync to see the usage that produced the Cloud Run bill.</p>
				{/if}
			</section>
		</main>
	</div>
</div>

<style>
	.dashboard {
		container-type: inline-size;
		margin-inline: auto;
		max-inline-size: 70rem;
		padding: 1rem;
	}

	.layout {
		display: grid;
		gap: 1.25rem;
	}

	/* Two columns only once the dashboard's own box is wide enough for them.
	   A sticky sidebar is worth having beside the breakdown and is a nuisance
	   stacked above it, so both arrive together. */
	@container (min-width: 45rem) {
		.layout {
			align-items: start;
			grid-template-columns: minmax(14rem, 18rem) minmax(0, 1fr);
		}

		.summary {
			position: sticky;
			inset-block-start: 1rem;
		}
	}

	/* The panels stack, whatever room the column has. There is one usage panel
	   so far, so nothing here assumes a row of them. */
	.main {
		align-content: start;
		display: grid;
		gap: 1.25rem;
	}

	.card {
		background: Canvas;
		border: 1px solid color-mix(in srgb, CanvasText 20%, transparent);
		border-radius: 0.5rem;
		padding: 1rem;
	}

	h1 {
		font-size: 1.125rem;
		margin: 0;
	}

	h2 {
		font-size: 1rem;
		margin: 0 0 0.75rem;
	}

	.eyebrow {
		font-size: 0.75rem;
		letter-spacing: 0.08em;
		margin: 0 0 0.25rem;
		opacity: 0.7;
		text-transform: uppercase;
	}

	.total {
		font-size: 2rem;
		font-variant-numeric: tabular-nums;
		font-weight: 600;
		margin: 0.5rem 0 0.25rem;
	}

	.caveat {
		font-size: 0.8125rem;
		margin: 0.25rem 0 0;
		opacity: 0.75;
		text-wrap: pretty;
	}

	hr {
		border: none;
		border-block-start: 1px solid color-mix(in srgb, CanvasText 20%, transparent);
		margin-block: 1rem;
	}

	button {
		background: CanvasText;
		border: 1px solid CanvasText;
		border-radius: 0.25rem;
		color: Canvas;
		cursor: pointer;
		font: inherit;
		inline-size: 100%;
		padding: 0.5rem 0.75rem;
	}

	button:disabled {
		cursor: default;
		opacity: 0.6;
	}

	.status {
		font-size: 0.8125rem;
		margin: 0.5rem 0 0;
		min-block-size: 1.25rem;
		opacity: 0.75;
	}

	.spinner {
		animation: spin 0.7s linear infinite;
		block-size: 0.75rem;
		border: 2px solid color-mix(in srgb, CanvasText 30%, transparent);
		border-block-start-color: CanvasText;
		border-radius: 50%;
		display: inline-block;
		inline-size: 0.75rem;
		margin-inline-end: 0.375rem;
		vertical-align: -0.0625rem;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.spinner {
			animation-duration: 2.2s;
		}
	}

	.error {
		border: 1px solid color-mix(in srgb, currentcolor 50%, transparent);
		border-radius: 0.25rem;
		color: color-mix(in srgb, red 65%, CanvasText);
		font-size: 0.8125rem;
		margin: 0.75rem 0 0;
		padding: 0.5rem 0.75rem;
		text-wrap: pretty;
	}

	.service {
		border-block-end: 1px solid color-mix(in srgb, CanvasText 12%, transparent);
		padding-block: 0.5rem;
	}

	/* The label may be long and the box may be 320px wide, so the bar gets a
	   row of its own under the text rather than a fixed-width column beside
	   it. */
	summary {
		cursor: pointer;
		display: grid;
		font-size: 0.875rem;
		gap: 0.25rem 0.75rem;
		grid-template-columns: minmax(0, 1fr) auto auto;
		align-items: baseline;
	}

	.name {
		min-inline-size: 0;
		overflow-wrap: anywhere;
	}

	/* `display: grid` on the summary drops the disclosure marker, so the
	   affordance is drawn back in. `<details>` still tells assistive tech
	   whether it is open, so this is decoration only. */
	.name::before {
		content: '\25B8\A0';
	}

	.service[open] .name::before {
		content: '\25BE\A0';
	}

	.cost,
	.share {
		font-variant-numeric: tabular-nums;
		text-align: end;
	}

	.share {
		font-size: 0.75rem;
		opacity: 0.7;
	}

	.track {
		background: color-mix(in srgb, CanvasText 12%, transparent);
		block-size: 0.5rem;
		border-radius: 0.25rem;
		grid-column: 1 / -1;
		overflow: hidden;
	}

	.fill {
		background: CanvasText;
		block-size: 100%;
		display: block;
	}

	.skus {
		font-size: 0.8125rem;
		list-style: none;
		margin: 0.5rem 0 0;
		padding: 0;
	}

	.skus li {
		display: grid;
		gap: 0.75rem;
		grid-template-columns: minmax(0, 1fr) auto;
		opacity: 0.85;
		padding-block: 0.125rem;
	}

	.skus span:first-child {
		overflow-wrap: anywhere;
	}

	.usage {
		container-type: inline-size;
	}

	.panel-cost {
		font-size: 1.375rem;
		font-variant-numeric: tabular-nums;
		font-weight: 600;
		margin: 0 0 0.75rem;
	}

	/* Two by two, as the panel is designed — and one column once the panel
	   itself is too narrow for two readable cells, which is the panel's own
	   width talking, not the viewport's. */
	.stat-grid {
		display: grid;
		gap: 0.75rem 1rem;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		margin: 0;
	}

	/* Measured, not guessed: two cells still read at a 320px viewport, where
	   this panel's own box is about 15rem wide. One column is for a panel
	   narrower than that — a sidebar, or a column of its own. */
	@container (max-width: 11rem) {
		.stat-grid {
			grid-template-columns: minmax(0, 1fr);
		}
	}

	.stat-grid + .caveat {
		margin-block-start: 0.75rem;
	}

	/* The figure reads above its label, while the markup keeps the term
	   before its description. */
	.stat {
		display: flex;
		flex-direction: column-reverse;
	}

	.stat-num {
		font-size: 1.125rem;
		font-variant-numeric: tabular-nums;
		font-weight: 600;
		margin: 0;
		overflow-wrap: anywhere;
	}

	.unit {
		font-size: 0.6875rem;
		font-weight: 400;
		margin-inline-start: 0.1875rem;
		opacity: 0.7;
	}

	.stat-label {
		font-size: 0.6875rem;
		margin-block-start: 0.125rem;
		opacity: 0.7;
		text-wrap: pretty;
	}

	.unavailable {
		font-size: 0.75rem;
		font-style: italic;
		margin: 0.5rem 0 0;
		opacity: 0.75;
		text-wrap: pretty;
	}
</style>
