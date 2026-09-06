<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiErrorMessage, apiFetchWithSession } from '#lib/api.js';
	import { isOwner } from '#lib/roles.js';
	import { PaginatedList, type CursorPage } from '#lib/paginatedList.svelte.js';
	import {
		billingPath,
		formatSignedQuantity,
		originLabel,
		purchaseCredits,
		type LedgerEntry
	} from '#lib/billing.js';
	import { readApprovalReturn } from '#lib/engagementRequest.js';
	import type { PracticeSession } from '../+layout.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PageProps as PageProperties } from './$types';

	const quantityId = 'buy-credits-quantity';

	// Balance and the ledger's first page come from +page.ts's load now,
	// not an onMount fetch (#471) -- a role refusal has to reach
	// practices/+error.svelte rather than sit in a local `error` string
	// this page owned before. Later pages (#446) are appended locally.
	let { data }: PageProperties = $props();

	// untrack because capturing data.ledger's first page once is the point
	// (#446): the list grows itself from here, so re-deriving from `data`
	// would drop every page appended since.
	const ledger = new PaginatedList<LedgerEntry>({
		first: untrack(() => data.ledger),
		loadPage: loadLedgerPage,
		failureMessage: 'Failed to load more ledger entries'
	});

	// Resolved once by practices/[practiceId]/+layout.ts (#835), not a
	// second /session fetch here -- the buy-credits button's enabled state
	// mirrors the "owner"-role gating the root Practice page already uses,
	// server-side enforcement (RequireOwner) is what actually matters.
	const session = $derived((page.data as { session: PracticeSession }).session);
	let isPracticeOwner = $derived(isOwner(session));
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

	onMount(() => {
		approvalReturn = readApprovalReturn();
	});

	// Throws on a refusal rather than returning it: PaginatedList catches,
	// and the balance endpoint answers the whole payload, of which only
	// the ledger pages.
	async function loadLedgerPage(cursor: string): Promise<CursorPage<LedgerEntry>> {
		const response = await apiFetchWithSession(
			`${billingPath(page.params.practiceId!)}?cursor=${encodeURIComponent(cursor)}`
		);
		if (!response.ok) throw new Error(await apiErrorMessage(response));
		const loaded: { ledger: CursorPage<LedgerEntry> } = await response.json();
		return loaded.ledger;
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

{#snippet intro()}
	<Text text={`Credit balance: ${data.balance}`} />
{/snippet}

{#snippet content()}
	<DataTable
		{columns}
		rows={ledger.items}
		hasMore={ledger.hasMore}
		onLoadMore={() => ledger.loadMore()}
		isLoadingMore={ledger.isLoadingMore}
		loadMoreError={ledger.loadMoreError}
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
		<Button label="Buy credits" type="submit" disabled={!isPracticeOwner} loading={isPurchasing} />
		{#if purchaseError}
			<Notice message={purchaseError} variant="error" />
		{/if}
	</form>
{/snippet}

<ListPage title="Billing" {intro} {content} />
