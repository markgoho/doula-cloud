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
		totalToPayFact,
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
		returnedToYouSentence
	} from '#lib/clientRegister.js';
	import { formatMoney } from '#lib/money.js';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
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
		failureMessage: 'Failed to load more invoices'
	});

	onMount(async () => {
		try {
			const first = await loadClientInvoicesPage(apiFetchWithSession, page.params.engagementId!, '');
			totalToPayCents = first.totalToPayCents;
			invoices.reset(first);
			isLoaded = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load invoices';
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

<!-- `even` (#2038): the three sections are one set of columns read one
     under the next, so each column starts at the same place in each. -->
{#snippet table(rows: ClientInvoice[], emptyMessage: string)}
	<DataTable
		columnWidths="even"
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

<!-- The hub's own "Total to pay" row (#2038): one treatment in both places. -->
{#snippet owedSection()}
	<SummaryList rows={[totalToPayFact(totalToPayCents, currency)]} />
	{@render table(owed, owedEmptyMessage)}
{/snippet}

{#snippet paidSection()}
	{@render table(paid, NO_PAYMENTS_MESSAGE)}
{/snippet}

{#snippet releasedSection()}
	{@render table(released, '')}
{/snippet}

<RecordDetail
	title={INVOICES_HEADING}
	serviceName={page.data.practiceName}
	backHref={resolve('/portal/(authenticated)/engagements/[engagementId]', {
		engagementId: page.params.engagementId!
	})}
	sections={isLoaded
		? [
				{ heading: OWED_HEADING, content: owedSection },
				{ heading: PAID_HEADING, content: paidSection },
				...(released.length > 0 ? [{ heading: NO_LONGER_OWED_HEADING, content: releasedSection }] : [])
			]
		: []}
	loading={isLoaded || error ? undefined : 'Loading your invoices'}
	loadError={error || undefined}
/>
