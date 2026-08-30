<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import { purchaseCredits, type LedgerEntry } from '#lib/billing.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import type { PageProps as PageProperties } from './$types';

	// Balance and ledger come from +page.ts's load now, not an onMount
	// fetch (#471) -- a role refusal has to reach practices/+error.svelte
	// rather than sit in a local `error` string this page owned before.
	let { data }: PageProperties = $props();

	let roles = $state<string[]>([]);
	let isOwner = $derived(roles.includes('owner'));
	let checkoutStatus = $derived(page.url.searchParams.get('checkout'));

	const columns = [
		{ label: 'Date', accessor: (entry: LedgerEntry) => new Date(entry.createdAt).toLocaleString() },
		{ label: 'Origin', accessor: (entry: LedgerEntry) => entry.origin },
		{
			label: 'Quantity',
			accessor: (entry: LedgerEntry) => `${entry.quantity > 0 ? '+' : ''}${entry.quantity}`
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

<h1>Billing</h1>

<p>Credit balance: {data.balance}</p>

<DataTable {columns} rows={data.ledger} emptyMessage="No ledger history yet." />

{#if checkoutStatus === 'success'}
	<p role="status">Credit purchase complete. The balance updates once Stripe confirms payment.</p>
{:else if checkoutStatus === 'cancelled'}
	<p role="status">Credit purchase cancelled.</p>
{/if}

<form onsubmit={handlePurchase}>
	<label>
		Quantity
		<input type="number" min="1" bind:value={quantity} required />
	</label>
	<Button label="Buy credits" type="submit" disabled={!isOwner} loading={isPurchasing} />
	{#if purchaseError}
		<p role="alert">{purchaseError}</p>
	{/if}
</form>
