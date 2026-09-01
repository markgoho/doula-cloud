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
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		formatAmount,
		invoiceStatusLabel,
		loadPracticeInvoices,
		type PracticeInvoice
	} from '#lib/invoice.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	/*
	 * Captures the first page only, the way the Billing ledger does
	 * (#446): `handleLoadMore` grows this list from here, so re-deriving
	 * it from `data` would drop every page appended since.
	 */
	let invoices = $state(data.items);
	let cursor = $state(data.nextCursor ?? '');
	let isMoreAvailable = $state(data.hasMore);
	let loadError = $state('');

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

	async function handleLoadMore() {
		loadError = '';
		try {
			const next = await loadPracticeInvoices(apiFetchWithSession, page.params.practiceId!, cursor);
			invoices = [...invoices, ...next.items];
			cursor = next.nextCursor ?? '';
			isMoreAvailable = next.hasMore;
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load more invoices';
		}
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
	rows={invoices}
	rowHref={engagementHref}
	hasMore={isMoreAvailable}
	onLoadMore={handleLoadMore}
	emptyMessage="No invoices yet. One appears here as soon as a contract is billed."
/>

{#if loadError}
	<!-- A raw <p role="alert">, the way the Billing page reports its own
	     purchase failure: Text is a closed set of type steps and tones and
	     carries no live-region role. -->
	<p role="alert">{loadError}</p>
{/if}
