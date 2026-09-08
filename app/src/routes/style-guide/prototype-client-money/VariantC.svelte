<script lang="ts">
	/*
	 * PROTOTYPE variant C -- "A statement: one list, in time order".
	 *
	 * Placement: a section on the hub, like A. What differs is the
	 * information hierarchy -- there are no owed/paid sections at all.
	 * One list, newest first, of everything that has happened to money on
	 * this Engagement: an Invoice sent, a Payment received. "Total to
	 * pay" is pinned above it, as the one figure she came for.
	 *
	 * The argument for it: a Client's question is usually "where are we
	 * up to", and two lists make her assemble the story from both halves.
	 * The argument against, which the prototype exists to expose: the
	 * portal hub ALREADY has a chronological list of everything that has
	 * happened (#486's activity ledger), and this is a second one that
	 * covers part of the same ground.
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
		invoiceStatusLabel,
		invoiceStatusVariant,
		paymentMethodLabel,
		totalToPayCents,
		unpaid,
		type MoneyState
	} from './fixture.js';

	let { money }: { money: MoneyState } = $props();

	interface Entry {
		id: string;
		at: string;
		title: string;
		amountCents: number;
		currency: string;
		statusLabel?: string;
		statusVariant?: 'success' | 'info' | 'neutral';
		detail: string;
		payable: boolean;
		reference: string;
		byHand: boolean;
	}

	const entries = $derived.by(() => {
		const rows: Entry[] = [];
		for (const invoice of money.invoices) {
			rows.push({
				id: `${invoice.id}-sent`,
				at: invoice.createdAt,
				title: 'Invoice sent',
				amountCents: invoice.amountCents,
				currency: invoice.currency,
				statusLabel: invoiceStatusLabel(invoice.status),
				statusVariant: invoiceStatusVariant(invoice.status),
				detail: `Invoice ${invoice.reference}`,
				payable: invoice.status === 'open' && invoice.billingMode === 'stripe',
				reference: invoice.reference,
				byHand: invoice.status === 'open' && invoice.billingMode === 'by_hand'
			});
			if (invoice.paidAt) {
				const record = money.payments.find((payment) => payment.invoiceId === invoice.id);
				rows.push({
					id: `${invoice.id}-paid`,
					at: invoice.paidAt,
					title: 'Payment received',
					amountCents: invoice.amountCents,
					currency: invoice.currency,
					// #981: the method, never the recorder's note.
					detail: record
						? `Invoice ${invoice.reference} — ${paymentMethodLabel(record.method)}`
						: `Invoice ${invoice.reference}`,
					payable: false,
					reference: invoice.reference,
					byHand: false
				});
			}
		}
		return rows.toSorted((a, b) => b.at.localeCompare(a.at));
	});

	const owed = $derived(unpaid(money));
	const total = $derived(totalToPayCents(money));
	const currency = $derived(money.invoices[0]?.currency ?? 'usd');
</script>

<section class="surface">
	<Heading level={2} variant="section" text="What you still owe" />

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

		<ol class="statement">
			{#each entries as entry (entry.id)}
				<li class="entry">
					<span class="when">{formatInstant(entry.at)}</span>
					<span class="what">
						<span class="title">{entry.title}</span>
						<span class="meta">{entry.detail}</span>
						{#if entry.byHand}
							<!-- #946 NOT YET DECIDED: whether a by-hand Invoice
							     reaches her at all. #980: no payable Stripe
							     object exists on that rail, by construction. -->
							<span class="meta"
								>Your Practice collects this one directly. Quote Invoice {entry.reference} when you
								pay.</span
							>
						{/if}
					</span>
					<span class="figures">
						<span class="amount">{formatMoney(entry.amountCents, entry.currency)}</span>
						{#if entry.statusLabel && entry.statusVariant}
							<Badge label={entry.statusLabel} variant={entry.statusVariant} />
						{/if}
						{#if entry.payable}
							<Button label="Pay" variant="primary" size="sm" />
						{/if}
					</span>
				</li>
			{/each}
		</ol>
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
	}

	.statement {
		display: flex;
		flex-direction: column;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.entry {
		display: grid;
		gap: var(--space-1) var(--space-4);
		padding-block: var(--space-3);
		border-block-start: var(--border-thin) solid var(--color-outline-variant);
	}

	/* Three columns only where the entry's own box can hold them. */
	@container (min-width: 34rem) {
		.entry {
			grid-template-columns: max-content minmax(0, 1fr) max-content;
			align-items: center;
		}
	}

	.when {
		font-size: var(--text-meta-size);
		color: var(--color-on-surface-muted);
		font-variant-numeric: tabular-nums;
	}

	.what {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.title {
		font-weight: var(--font-weight-medium);
	}

	.meta {
		font-size: var(--text-meta-size);
		color: var(--color-on-surface-muted);
	}

	.figures {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2) var(--space-3);
	}

	.amount {
		font-size: var(--text-heading-size);
		font-weight: var(--text-heading-weight);
		font-variant-numeric: tabular-nums;
	}
</style>
