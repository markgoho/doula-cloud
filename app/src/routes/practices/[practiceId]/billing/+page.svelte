<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import {
		formatSignedQuantity,
		loadLedgerPage,
		originLabel,
		purchaseCredits,
		type LedgerEntry
	} from '#lib/billing.js';
	import { formatMoney } from '#lib/money.js';
	import { readApprovalReturn } from '#lib/engagementRequest.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
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
		loadPage: (cursor) => loadLedgerPage(apiFetchWithSession, page.params.practiceId!, cursor),
		failureMessage: 'Failed to load more ledger entries'
	});

	// This screen reads no role of its own (#1162). Who may be here at all
	// is ADR-0008's Credit row -- Owner and Admin, Doula ✗ -- and both the
	// balance read and the purchase declare that same seat,
	// staffauth.OwnerAndAdmin in billing/mount.go. The balance arrives
	// through +page.ts's own load, so a session outside that seat meets its
	// refuseRead and practices/+error.svelte and never mounts this
	// component; every session that does mount it may buy. The button is
	// therefore unconditional, rather than drawn disabled for a caller who
	// cannot reach the page (#257's fix, which #272 and #910 between them
	// left with no session to fire for).
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

	// Defaults to 5, not 1: no minimum purchase, but the common case lands
	// nearer Stripe's 3.2% effective rate than the 4.4% a single Credit
	// pays (#286, decided on #429).
	let quantity = $state(5);
	let purchaseError = $state('');
	let isPurchasing = $state(false);

	// Undefined, not zero, when the BFF couldn't read the Price off Stripe
	// (#285) -- the price region below says so in words instead of
	// pretending a Credit is free.
	const priceItems = $derived(
		data.price
			? [
					{ label: 'Price per Credit', value: formatMoney(data.price.unitAmountCents, data.price.currency) },
					{
						label: 'Subtotal',
						value: formatMoney(data.price.unitAmountCents * quantity, data.price.currency)
					}
				]
			: []
	);

	onMount(() => {
		approvalReturn = readApprovalReturn();
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

{#snippet intro()}
	<!--
		#256: this screen used to say only "Credit balance: {n}", nothing
		naming who a Credit is bought from or what one costs -- so an Owner
		could not tell it apart from Getting paid (Stripe Connect, a
		different counterparty entirely) without opening both. The three
		sentences below state that in CONTEXT.md's own words: Credit ("a
		unit of Doula Cloud's own billing... one costs $20.00") and
		Connected account ("so Clients can pay that Practice directly").
		"Stripe account" itself is on that entry's own _Avoid_ list, so the
		cross-reference says "connects Stripe", matching the button and
		status copy on that screen.
	-->
	<Text text="Practices buy Credits from Doula Cloud." />
	<Text
		text="One Credit covers one Engagement — a single Client relationship centered on one baby, from intake through the end of care."
	/>
	<Text text="One Credit costs $20.00." />
	<Text text="Getting paid is a separate screen, where this Practice connects Stripe so its Clients can pay it directly." />
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
	{:else if checkoutStatus === 'canceled'}
		<Notice message="Credit purchase canceled." variant="status" />
	{/if}

	<!-- stacked-form:ignore: #1108 -- the Quantity box is `required` with a `min`, and that is the only thing standing between an empty or zero quantity and a Stripe checkout. `StackedForm` sets `novalidate` (ADR-0021), so adopting it here would take that refusal away and put nothing in its place; #1228 is where this form gets a refusal of its own and then adopts the molecule. The stack below is `StackedForm`'s own arrangement, written inline meanwhile. -->
	<form onsubmit={handlePurchase}>
		<stack-l space="var(--space-5)">
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

			{#if data.price}
				<DescriptionList items={priceItems} />
				<Text
					text="New York sales tax is added at checkout where it applies."
					step="body-sm"
					tone="variant"
				/>
			{:else}
				<!--
					#256's intro above now states the ordinary $20.00 price as a
					fixed fact, so this sentence is scoped to what is actually
					missing -- today's checkout total from Stripe -- rather than
					repeating "price" and reading as a contradiction of it.
				-->
				<Text text="This purchase's exact price could not be confirmed with Stripe right now." step="body-sm" tone="variant" />
			{/if}

			<Button label="Buy credits" type="submit" loading={isPurchasing} />
			{#if purchaseError}
				<Notice message={purchaseError} variant="error" />
			{/if}
		</stack-l>
	</form>
{/snippet}

<ListPage title="Credits" {intro} {content} />
