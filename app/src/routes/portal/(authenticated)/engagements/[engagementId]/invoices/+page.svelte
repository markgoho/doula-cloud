<script lang="ts">
	/*
	 * The Client's money, one screen (#983, built on #1564): her Invoices,
	 * as "What you still owe" over the ones she has not paid and "What you
	 * have paid" over the ones she has. One list from the server serves
	 * both (#1011) -- the sections are filters over it, so there is no
	 * second array to fall out of step.
	 *
	 * Every section renders, empty or not (#982): a missing section and a
	 * section that failed to load look alike to her, and this is the screen
	 * she visits to be reassured. A third section, "What you no longer owe", holds
	 * the void and written-off Invoices and appears only when there is one:
	 * #981 gave those Invoices a label but no heading, and a heading over
	 * nothing would say something about a Client who has none.
	 *
	 * Each row links to the Invoice's own page, where the paying happens: a
	 * Payment Element belongs to exactly one Invoice (#983).
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import {
		havePaid,
		invoiceStatusText,
		loadClientInvoicesPage,
		noLongerOwed,
		stillOwed,
		totalReturnedCents,
		type ClientInvoice
	} from '#lib/clientInvoice.js';
	import {
		ALL_INVOICES_PAID_MESSAGE,
		INVOICES_HEADING,
		NO_INVOICES_MESSAGE,
		NO_LONGER_OWED_HEADING,
		NO_PAYMENTS_MESSAGE,
		OWED_HEADING,
		PAID_HEADING,
		TOTAL_TO_PAY_LABEL,
		returnedToYouSentence
	} from '#lib/clientRegister.js';
	import { formatMoney } from '#lib/money.js';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';

	let isLoaded = $state(false);
	let error = $state('');
	// The whole-Engagement figure from the server, repeated on every page.
	let totalToPayCents = $state(0);

	const invoices = new PaginatedList<ClientInvoice>({
		first: { items: [], hasMore: false },
		loadPage: async (cursor) => {
			const next = await loadClientInvoicesPage(apiFetchWithSession, page.params.engagementId!, cursor);
			totalToPayCents = next.totalToPayCents;
			return next;
		},
		failureMessage: 'Failed to load more Invoices'
	});

	onMount(async () => {
		try {
			const first = await loadClientInvoicesPage(apiFetchWithSession, page.params.engagementId!, '');
			totalToPayCents = first.totalToPayCents;
			invoices.reset(first);
			isLoaded = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Invoices';
		}
	});

	const currency = $derived(invoices.items[0]?.currency ?? 'usd');
	const owed = $derived(stillOwed(invoices.items));
	const paid = $derived(havePaid(invoices.items));
	const released = $derived(noLongerOwed(invoices.items));

	/*
	 * The owed section's empty sentence is the one #982 wrote for the state
	 * she is in. With Invoices that are all void it says nothing beyond the
	 * total above it: none of the three ruled sentences is true there.
	 */
	const owedEmptyMessage = $derived.by(() => {
		if (invoices.items.length === 0) return NO_INVOICES_MESSAGE;
		if (paid.length === 0) return '';
		const returned = totalReturnedCents(invoices.items);
		return returned > 0
			? `${ALL_INVOICES_PAID_MESSAGE} ${returnedToYouSentence(formatMoney(returned, currency))}`
			: ALL_INVOICES_PAID_MESSAGE;
	});

	const columns = [
		{ label: 'Invoice number', accessor: (row: ClientInvoice) => row.reference, variant: 'body' as const },
		{
			label: 'Amount',
			accessor: (row: ClientInvoice) => formatMoney(row.amountCents, row.currency),
			numeric: true
		},
		{ label: 'Status', accessor: (row: ClientInvoice) => invoiceStatusText(row) }
	];

	function invoiceHref(row: ClientInvoice) {
		return resolve('/portal/(authenticated)/engagements/[engagementId]/invoices/[invoiceId]', {
			engagementId: page.params.engagementId!,
			invoiceId: row.id
		});
	}
</script>

{#snippet table(rows: ClientInvoice[], emptyMessage: string)}
	<DataTable
		{columns}
		{rows}
		rowHref={invoiceHref}
		hasMore={invoices.hasMore}
		onLoadMore={() => invoices.loadMore()}
		isLoadingMore={invoices.isLoadingMore}
		loadMoreError={invoices.loadMoreError}
		{emptyMessage}
	/>
{/snippet}

{#snippet owedSection()}
	<p class="total">
		<span class="total-label">{TOTAL_TO_PAY_LABEL}</span>
		<span class="total-figure">{formatMoney(totalToPayCents, currency)}</span>
	</p>
	{@render table(owed, owedEmptyMessage)}
{/snippet}

{#snippet paidSection()}
	{@render table(paid, NO_PAYMENTS_MESSAGE)}
{/snippet}

{#snippet releasedSection()}
	{@render table(released, '')}
{/snippet}

<container-l>
	<center-l max="none" gutters="var(--page-gutter)">
		<BackLink
			href={resolve('/portal/(authenticated)/engagements/[engagementId]', {
				engagementId: page.params.engagementId!
			})}
		/>
	</center-l>
</container-l>

<RecordDetail
	title={INVOICES_HEADING}
	serviceName={page.data.practiceName}
	sections={isLoaded
		? [
				{ heading: OWED_HEADING, content: owedSection },
				{ heading: PAID_HEADING, content: paidSection },
				...(released.length > 0 ? [{ heading: NO_LONGER_OWED_HEADING, content: releasedSection }] : [])
			]
		: []}
	loading={isLoaded || error ? undefined : 'Loading your Invoices'}
	loadError={error || undefined}
/>

<style>
	@layer components {
		/* Wraps rather than assuming a width: the figure drops under its
		   label when the container cannot hold both (ADR-0024). */
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
			overflow-wrap: anywhere;
		}
	}
</style>
