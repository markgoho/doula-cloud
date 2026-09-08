<script lang="ts">
	/*
	 * PROTOTYPE variant B -- "Its own route, an index and a detail page".
	 *
	 * Placement: a fifth sibling under the Engagement, beside contract,
	 * messages, birth-plan and notifications, reached from a one-line
	 * teaser on the hub. The hub stays about the care; money is a
	 * destination.
	 *
	 * Shape: the list is NAVIGATION, not the place anything is paid --
	 * one row per Invoice, and the pay affordance lives on the Invoice's
	 * own page. This is the shape that survives the fact #980 surfaced:
	 * each Invoice is its own Stripe object with its own client secret,
	 * so a Payment Element belongs to one Invoice and cannot pay two.
	 * A page per Invoice gives that Element somewhere to live.
	 *
	 * Clicking a row walks to the Invoice's own page; "Back to Invoices"
	 * returns.
	 */
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import { formatInstant } from '#lib/dates.js';
	import { formatMoney } from '#lib/money.js';
	import {
		emptyInvoices,
		emptyOwed,
		invoiceStatusLabel,
		invoiceStatusVariant,
		paymentMethodLabel,
		totalToPayCents,
		unpaid,
		type MoneyState
	} from './fixture.js';

	let { money }: { money: MoneyState } = $props();

	let openedId = $state<string | undefined>();
	const opened = $derived(money.invoices.find((invoice) => invoice.id === openedId));
	const owed = $derived(unpaid(money));
	const total = $derived(totalToPayCents(money));
	const currency = $derived(money.invoices[0]?.currency ?? 'usd');

	function detailItems(invoiceId: string) {
		const invoice = money.invoices.find((each) => each.id === invoiceId)!;
		const record = money.payments.find((payment) => payment.invoiceId === invoiceId);
		const items = [
			{ label: 'Amount', value: formatMoney(invoice.amountCents, invoice.currency) },
			{ label: 'Invoice number', value: invoice.reference },
			{ label: 'Status', value: invoiceStatusLabel(invoice.status) },
			{ label: 'Sent', value: formatInstant(invoice.createdAt) }
		];
		if (invoice.paidAt) items.push({ label: 'Paid', value: formatInstant(invoice.paidAt) });
		if (record) items.push({ label: 'How you paid', value: paymentMethodLabel(record.method) });
		return items;
	}
</script>

<div class="surface">
	<!-- The hub keeps one line, and it is a link, not a figure she has to
	     act on where she stands. -->
	<aside class="teaser">
		<Text
			text={owed.length === 0
				? 'Invoices — nothing to pay right now'
				: `Invoices — ${formatMoney(total, currency)} to pay`}
			step="body-sm"
			tone="variant"
		/>
	</aside>

	{#if opened}
		<article class="page">
			<Button label="Back to Invoices" variant="bare" onClick={() => (openedId = undefined)} />
			<Heading level={1} variant="page" text="Invoice {opened.reference}" />
			<DescriptionList items={detailItems(opened.id)} />

			{#if opened.status === 'open'}
				{#if opened.billingMode === 'stripe'}
					<!-- #980: a finalized send_invoice Invoice exposes a
					     confirmation_secret, and the Payment Element mounts
					     against it with stripeAccount. This box is where it
					     goes. -->
					<div class="element-slot">
						<Text text="Stripe Payment Element mounts here" step="body-sm" tone="muted" />
					</div>
					<Button label="Pay {formatMoney(opened.amountCents, opened.currency)}" variant="primary" />
				{:else}
					<!-- #946 NOT YET DECIDED: whether she reaches this page at
					     all on the by-hand rail. #980 settled that there is no
					     payable Stripe object here to click. -->
					<Text
						text="Your Practice collects this Invoice directly. Quote Invoice {opened.reference} when you pay."
						tone="variant"
					/>
				{/if}
			{/if}
		</article>
	{:else}
		<article class="page">
			<Heading level={1} variant="page" text="Invoices" />

			{#if money.invoices.length === 0}
				<Text text={emptyInvoices} tone="muted" />
			{:else}
				{#if owed.length === 0}
					<Text text={emptyOwed} tone="muted" />
				{:else}
					<p class="total">
						<span class="total-label">Total to pay</span>
						<span class="total-figure">{formatMoney(total, currency)}</span>
					</p>
				{/if}

				<ul class="rows">
					{#each money.invoices as invoice (invoice.id)}
						<li>
							<button class="row" type="button" onclick={() => (openedId = invoice.id)}>
								<span class="row-lead">
									<span class="amount">{formatMoney(invoice.amountCents, invoice.currency)}</span>
									<span class="meta">Invoice {invoice.reference}</span>
								</span>
								<Badge
									label={invoiceStatusLabel(invoice.status)}
									variant={invoiceStatusVariant(invoice.status)}
								/>
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</article>
	{/if}
</div>

<style>
	.surface {
		display: flex;
		flex-direction: column;
		gap: var(--space-6);
		container-type: inline-size;
	}

	.teaser {
		padding: var(--space-3);
		border: var(--border-thin) dashed var(--color-outline-variant);
		border-radius: var(--radius-md);
	}

	.page {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		align-items: start;
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
	}

	.rows {
		display: flex;
		flex-direction: column;
		gap: 0;
		align-self: stretch;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.row {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2) var(--space-4);
		align-items: center;
		justify-content: space-between;
		inline-size: 100%;
		padding-block: var(--space-3);
		border: 0;
		border-block-start: var(--border-thin) solid var(--color-outline-variant);
		background: none;
		font: inherit;
		text-align: start;
		cursor: pointer;
	}

	.row-lead {
		display: flex;
		flex-direction: column;
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

	.element-slot {
		align-self: stretch;
		padding: var(--space-6) var(--space-4);
		border: var(--border-thin) dashed var(--color-outline-variant);
		border-radius: var(--radius-md);
		text-align: center;
	}
</style>
