<script lang="ts">
	/*
	 * The Practice-wide Invoice list (#265, gap RA-G7). Before it, an
	 * Invoice was reachable only inside one Engagement's Contract, so
	 * "who owes us money" meant opening every Engagement in turn.
	 *
	 * This page composes existing components only and writes no CSS of its
	 * own: the totals are a `DescriptionList`, the book is a `DataTable`,
	 * and both already adapt to the space they are given. That is also
	 * what keeps it inside CLAUDE.md's no-new-components block.
	 */
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		formatAmount,
		invoiceStatusLabel,
		loadPracticeInvoices,
		type PracticeInvoice
	} from '#lib/invoice.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	/*
	 * Takes the load's first page and grows from there (#446). The list
	 * owns its own cursor, in-flight guard and error text, so re-deriving
	 * from `data` -- which would drop every page appended since -- is not
	 * something this file can do by accident any more.
	 */
	const invoices = new PaginatedList({
		// untrack because capturing the load's page once is the whole point:
		// re-deriving from `data` would drop every page appended since.
		first: untrack(() => data),
		loadPage: (cursor) => loadPracticeInvoices(apiFetchWithSession, page.params.practiceId!, cursor),
		failureMessage: 'Failed to load more invoices'
	});

	/*
	 * The totals stay the load's, never the loaded pages': the BFF returns
	 * the whole book's figures on every page, so paging must not make
	 * "outstanding" look like it is only what has been scrolled to.
	 */
	const summary = $derived([
		{ label: 'Outstanding', value: formatAmount(data.outstandingCents) },
		{
			label: 'Unpaid invoices',
			value: String(data.outstandingCount)
		},
		{ label: 'Paid', value: formatAmount(data.paidCents) }
	]);

	const columns = [
		{ label: 'Client', accessor: (invoice: PracticeInvoice) => invoice.clientName },
		{
			label: 'Amount',
			accessor: (invoice: PracticeInvoice) => formatAmount(invoice.amountCents),
			numeric: true
		},
		{ label: 'Status', accessor: (invoice: PracticeInvoice) => invoiceStatusLabel(invoice.status) },
		{
			label: 'Billed',
			accessor: (invoice: PracticeInvoice) => new Date(invoice.createdAt).toLocaleDateString()
		},
		{
			// An em dash rather than a blank, so an unpaid row reads as
			// "nothing here yet" instead of a cell that failed to render.
			label: 'Paid',
			accessor: (invoice: PracticeInvoice) =>
				invoice.paidAt ? new Date(invoice.paidAt).toLocaleDateString() : '—'
		}
	];

	function engagementHref(invoice: PracticeInvoice): string {
		return resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId: invoice.engagementId
		});
	}

</script>

<PageTitle page="Invoices" />
<Heading level={1} text="Invoices" />
<Text
	text="Every invoice this practice has billed, newest first. Open one to reach the engagement it belongs to."
	tone="muted"
/>

<DescriptionList items={summary} />

<DataTable
	{columns}
	rows={invoices.items}
	rowHref={engagementHref}
	hasMore={invoices.hasMore}
	onLoadMore={() => invoices.loadMore()}
	isLoadingMore={invoices.isLoadingMore}
	emptyMessage="No invoices yet. One appears here as soon as a contract is billed."
/>

{#if invoices.loadMoreError}
	<Notice message={invoices.loadMoreError} variant="error" />
{/if}
