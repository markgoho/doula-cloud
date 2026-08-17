<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import { loadBalance, type LedgerEntry } from '#lib/billing.js';

	let balance = $state<number | undefined>();
	let ledger = $state<LedgerEntry[]>([]);
	let error = $state('');

	onMount(async () => {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			error = 'You must be logged in to view billing';
			return;
		}
		try {
			const idToken = await user.getIdToken();
			const fetch = (path: string, init?: RequestInit) => apiFetch(path, idToken, init);
			const result = await loadBalance(fetch, page.params.practiceId!);
			balance = result.balance;
			ledger = result.ledger;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load billing balance';
		}
	});
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
{/if}
