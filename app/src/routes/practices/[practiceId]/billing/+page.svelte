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
	import { readApprovalReturn } from '#lib/engagementRequest.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
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
	let isLoadingMoreLedger = $state(false);
	let loadMoreLedgerError = $state('');

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

	/*
	 * An approver sent here by an empty balance mid-decision (#502) is
	 * given the way back to the Request she was deciding. Stripe's return
	 * URLs point at this page and nothing else, so the approval screen
	 * leaves its own address in sessionStorage on the way out and this
	 * page reads it -- read once on mount rather than derived, because
	 * nothing on this page changes it.
	 */
	let approvalReturn = $state('');

	let quantity = $state(1);
	let purchaseError = $state('');
	let isPurchasing = $state(false);

	onMount(async () => {
		approvalReturn = readApprovalReturn();
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
		loadMoreLedgerError = '';
		isLoadingMoreLedger = true;
		try {
			const response = await apiFetchWithSession(
				`${billingPath(page.params.practiceId!)}?cursor=${encodeURIComponent(ledgerCursor)}`
			);
			if (!response.ok) {
				loadMoreLedgerError = await response.text();
				return;
			}
			const loaded: { ledger: { items: LedgerEntry[]; nextCursor?: string; hasMore: boolean } } =
				await response.json();
			ledgerEntries = [...ledgerEntries, ...loaded.ledger.items];
			ledgerCursor = loaded.ledger.nextCursor ?? '';
			isMoreLedgerAvailable = loaded.ledger.hasMore;
		} catch (error_) {
			loadMoreLedgerError =
				error_ instanceof Error ? error_.message : 'Failed to load more ledger entries';
		} finally {
			isLoadingMoreLedger = false;
		}
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
<Heading level={1} text="Billing" />

<Text text={`Credit balance: ${data.balance}`} />

<DataTable
	{columns}
	rows={ledgerEntries}
	hasMore={isMoreLedgerAvailable}
	onLoadMore={handleLoadMoreLedger}
	isLoadingMore={isLoadingMoreLedger}
	loadMoreError={loadMoreLedgerError}
	emptyMessage="No ledger history yet."
/>

{#if approvalReturn}
	<Link href={approvalReturn} label="Back to the engagement request you were deciding" />
{/if}

{#if checkoutStatus === 'success'}
	<Notice
		message="Credit purchase complete. The balance updates once Stripe confirms payment."
		variant="status"
	/>
{:else if checkoutStatus === 'cancelled'}
	<Notice message="Credit purchase cancelled." variant="status" />
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
		<Notice message={purchaseError} variant="error" />
	{/if}
</form>
