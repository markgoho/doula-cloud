<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		billingPath,
		formatSignedQuantity,
		originLabel,
		purchaseCredits,
		type LedgerEntry
	} from '#lib/billing.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import type { PageProps as PageProperties } from './$types';

	const quantityId = 'buy-credits-quantity';

	// Balance and the ledger's first page come from +page.ts's load now,
	// not an onMount fetch (#471) -- a role refusal has to reach
	// practices/+error.svelte rather than sit in a local `error` string
	// this page owned before. Later pages (#446) are appended locally.
	let { data }: PageProperties = $props();

	// Deliberately captures data.ledger's initial value only (#446):
	// handleLoadMoreLedger grows this list itself from here on, so
	// re-deriving from `data` on every change would drop appended pages.
	let ledgerEntries = $state(data.ledger.items);
	let ledgerCursor = $state(data.ledger.nextCursor ?? '');
	let isMoreLedgerAvailable = $state(data.ledger.hasMore);

	let roles = $state<string[]>([]);
	let isOwner = $derived(roles.includes('owner'));
	let checkoutStatus = $derived(page.url.searchParams.get('checkout'));

	const columns = [
		{ label: 'Date', accessor: (entry: LedgerEntry) => new Date(entry.createdAt).toLocaleString() },
		{ label: 'Origin', accessor: (entry: LedgerEntry) => originLabel(entry.origin) },
		{
			label: 'Quantity',
			accessor: (entry: LedgerEntry) => formatSignedQuantity(entry.quantity),
			numeric: true
		}
	];

	let quantity = $state(1);
	let purchaseError = $state('');
	let isPurchasing = $state(false);

	onMount(async () => {
		// The buy-credits button's enabled state mirrors the "owner"-role
		// gating the root Practice page already uses -- server-side
		// enforcement (RequireOwner) is what actually matters, this is UX
		// only.
		const sessionResponse = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/session`);
		if (sessionResponse.ok) {
			const body: { roles: string[] } = await sessionResponse.json();
			roles = body.roles;
		}
	});

	async function handleLoadMoreLedger() {
		const response = await apiFetchWithSession(
			`${billingPath(page.params.practiceId!)}?cursor=${encodeURIComponent(ledgerCursor)}`
		);
		if (!response.ok) return;
		const loaded: { ledger: { items: LedgerEntry[]; nextCursor?: string; hasMore: boolean } } =
			await response.json();
		ledgerEntries = [...ledgerEntries, ...loaded.ledger.items];
		ledgerCursor = loaded.ledger.nextCursor ?? '';
		isMoreLedgerAvailable = loaded.ledger.hasMore;
	}

	async function handlePurchase(event: SubmitEvent) {
		event.preventDefault();
		purchaseError = '';
		isPurchasing = true;
		try {
			const checkoutUrl = await purchaseCredits(apiFetchWithSession, page.params.practiceId!, quantity);
			location.assign(checkoutUrl);
		} catch (error_) {
			purchaseError = error_ instanceof Error ? error_.message : 'Failed to start credit purchase';
		} finally {
			isPurchasing = false;
		}
	}
</script>

<PageTitle page="Billing" />
<h1>Billing</h1>

<p>Credit balance: {data.balance}</p>

<DataTable
	{columns}
	rows={ledgerEntries}
	hasMore={isMoreLedgerAvailable}
	onLoadMore={handleLoadMoreLedger}
	emptyMessage="No ledger history yet."
/>

{#if checkoutStatus === 'success'}
	<p role="status">Credit purchase complete. The balance updates once Stripe confirms payment.</p>
{:else if checkoutStatus === 'cancelled'}
	<p role="status">Credit purchase cancelled.</p>
{/if}

<form onsubmit={handlePurchase}>
	<!--
		Through LabeledField and TextInput rather than a raw <label> around a
		raw <input>: reaching around the atoms put the word "Quantity" on the
		same line as its box, which is the defect #425 found and #475 walked
		the pages to catch the rest of.
	-->
	<LabeledField id={quantityId} label="Quantity">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				type="number"
				inputmode="numeric"
				min={1}
				required
				value={String(quantity)}
				onInput={(entered) => (quantity = Number(entered))}
			/>
		{/snippet}
	</LabeledField>
	<Button label="Buy credits" type="submit" disabled={!isOwner} loading={isPurchasing} />
	{#if purchaseError}
		<p role="alert">{purchaseError}</p>
	{/if}
</form>
