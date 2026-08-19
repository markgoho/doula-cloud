<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadBalance, purchaseCredits, type LedgerEntry } from '#lib/billing.js';

	let balance = $state<number | undefined>();
	let ledger = $state<LedgerEntry[]>([]);
	let error = $state('');
	let roles = $state<string[]>([]);
	let isOwner = $derived(roles.includes('owner'));
	let checkoutStatus = $derived(page.url.searchParams.get('checkout'));

	let quantity = $state(1);
	let purchaseError = $state('');
	let isPurchasing = $state(false);

	onMount(async () => {
		try {
			const result = await loadBalance(apiFetchWithSession, page.params.practiceId!);
			balance = result.balance;
			ledger = result.ledger;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load billing balance';
			return;
		}

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

{#if error}
	<p role="alert">{error}</p>
{:else if balance !== undefined}
	<p>Credit balance: {balance}</p>

	{#if ledger.length === 0}
		<p>No ledger history yet.</p>
	{:else}
		<table>
			<thead>
				<tr>
					<th>Date</th>
					<th>Origin</th>
					<th>Quantity</th>
				</tr>
			</thead>
			<tbody>
				{#each ledger as entry (entry.createdAt + entry.origin + entry.quantity)}
					<tr>
						<td>{new Date(entry.createdAt).toLocaleString()}</td>
						<td>{entry.origin}</td>
						<td>{entry.quantity > 0 ? '+' : ''}{entry.quantity}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}

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
		<button type="submit" disabled={isPurchasing || !isOwner}>Buy credits</button>
		{#if purchaseError}
			<p role="alert">{purchaseError}</p>
		{/if}
	</form>
{/if}
