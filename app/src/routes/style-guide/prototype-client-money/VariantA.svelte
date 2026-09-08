<script lang="ts">
	/*
	 * PROTOTYPE variant A -- "A section on the hub, two lists".
	 *
	 * Placement: the money surface is a section of the portal Engagement
	 * detail page, a sibling of "Everything that has happened". No new
	 * nav item, no new route. The Client signs in and the answer to
	 * "what do I still owe" is already on the page she lands on.
	 *
	 * Shape: two lists, exactly the two surfaces #981 named. Every open
	 * Invoice is a row and carries its own pay action, so the deposit-
	 * plus-balance case (#741) is legible without a running total having
	 * to explain itself.
	 */
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import { formatInstant } from '#lib/dates.js';
	import { formatMoney } from '#lib/money.js';
	import {
		emptyInvoices,
		emptyOwed,
		emptyPayments,
		invoiceStatusLabel,
		invoiceStatusVariant,
		paymentMethodLabel,
		totalToPayCents,
		unpaid,
		type MoneyState
	} from './fixture.js';

	let { money }: { money: MoneyState } = $props();

	const owed = $derived(unpaid(money));
	const settled = $derived(money.invoices.filter((invoice) => invoice.status !== 'open'));
	const total = $derived(totalToPayCents(money));
	const currency = $derived(money.invoices[0]?.currency ?? 'usd');
</script>

<!-- Both sections always render. #982: a hidden section and a broken one
     look alike to her. -->
<section class="surface">
	<Heading level={2} variant="section" text="What you still owe" />

	{#if money.invoices.length === 0}
		<Text text={emptyInvoices} tone="muted" />
	{:else if owed.length === 0}
		<Text text={emptyOwed} tone="muted" />
	{:else}
		<p class="total">
			<span class="total-label">Total to pay</span>
			<span class="total-figure">{formatMoney(total, currency)}</span>
		</p>

		<ul class="rows">
			{#each owed as invoice (invoice.id)}
				<li class="row">
					<div class="row-facts">
						<span class="amount">{formatMoney(invoice.amountCents, invoice.currency)}</span>
						<Badge
							label={invoiceStatusLabel(invoice.status)}
							variant={invoiceStatusVariant(invoice.status)}
						/>
						<span class="meta">Invoice {invoice.reference}</span>
						<span class="meta">Sent {formatInstant(invoice.createdAt)}</span>
					</div>

					{#if invoice.billingMode === 'stripe'}
						<Button label="Pay this Invoice" variant="primary" />
					{:else}
						<!-- #946 NOT YET DECIDED: whether a by-hand Invoice
						     reaches her at all. #980 settled that if it does,
						     there is no payable Stripe object to click, by
						     construction. This is what stands in the button's
						     place. -->
						<Text
							text="Your Practice collects this one directly. Quote Invoice {invoice.reference} when you pay."
							step="body-sm"
							tone="variant"
						/>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>

<section class="surface">
	<Heading level={2} variant="section" text="What you have paid" />

	{#if settled.length === 0}
		<Text text={emptyPayments} tone="muted" />
	{:else}
		<ul class="rows">
			{#each settled as invoice (invoice.id)}
				{@const record = money.payments.find((payment) => payment.invoiceId === invoice.id)}
				<li class="row">
					<div class="row-facts">
						<span class="amount">{formatMoney(invoice.amountCents, invoice.currency)}</span>
						<Badge
							label={invoiceStatusLabel(invoice.status)}
							variant={invoiceStatusVariant(invoice.status)}
						/>
						<span class="meta">Invoice {invoice.reference}</span>
						{#if invoice.paidAt}
							<span class="meta">Paid {formatInstant(invoice.paidAt)}</span>
						{/if}
						<!-- #981: she reads the method, never the recorder's note. -->
						{#if record}
							<span class="meta">{paymentMethodLabel(record.method)}</span>
						{/if}
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</section>

<style>
	.surface {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		container-type: inline-size;
	}

	.total {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: var(--space-2) var(--space-4);
		margin: 0;
	}

	.total-label {
		font-size: var(--text-label-size);
		font-weight: var(--text-label-weight);
		color: var(--color-on-surface-variant);
	}

	.total-figure {
		font-size: var(--text-heading-lg-size);
		font-weight: var(--text-heading-lg-weight);
		font-variant-numeric: tabular-nums;
		color: var(--color-on-surface);
	}

	.rows {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.row {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		padding-block: var(--space-3);
		border-block-start: var(--border-thin) solid var(--color-outline-variant);
	}

	/* The row goes two-column only when its own box can hold both,
	   never at a viewport width (ADR-0024). */
	@container (min-width: 30rem) {
		.row {
			flex-direction: row;
			align-items: center;
			justify-content: space-between;
		}
	}

	.row-facts {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2) var(--space-3);
		min-width: 0;
	}

	.amount {
		font-size: var(--text-heading-size);
		font-weight: var(--text-heading-weight);
		font-variant-numeric: tabular-nums;
	}

	.meta {
		font-size: var(--text-meta-size);
		color: var(--color-on-surface-muted);
	}
</style>
