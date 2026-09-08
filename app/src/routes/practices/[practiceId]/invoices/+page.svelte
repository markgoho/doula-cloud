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
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		clientsCannotPayMessage,
		dueLabel,
		formatAmount,
		invoiceStatusLabel,
		loadPracticeInvoices,
		type PracticeInvoice
	} from '#lib/invoice.js';
	import Link from '#lib/components/atoms/Link.svelte';
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
		// Every later page asks for the same narrowing the first one did
		// (#768) -- otherwise "load more" on an overdue list would append
		// the whole book underneath it.
		loadPage: (cursor) =>
			loadPracticeInvoices(apiFetchWithSession, page.params.practiceId!, cursor, data.isNarrowedToOverdue),
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
		{ label: 'Paid', value: formatAmount(data.paidCents) },
		// #768: the ageing figure sits beside the outstanding one it is a
		// slice of, so "who owes me, and for how long" is answered by the
		// same block rather than by opening a second screen.
		{ label: 'Overdue', value: formatAmount(data.overdueCents) },
		{ label: 'Overdue invoices', value: String(data.overdueCount) }
	]);

	/*
	 * #768: the narrowing is two links, not a control this page holds in
	 * memory -- the URL is the state, so it survives a reload and a back
	 * button, and the page needs no client-side JavaScript to offer it.
	 * The Practice's own overdue count is on the link, because a filter
	 * that shows nothing and a filter nobody needs look identical until
	 * the number is said.
	 */
	const invoicesHref = $derived(
		resolve('/practices/[practiceId]/invoices', { practiceId: page.params.practiceId! })
	);

	const columns = [
		{ label: 'Client', accessor: (invoice: PracticeInvoice) => invoice.clientName },
		{
			// #271: a by-hand Invoice's own per-Practice sequence, or
			// Stripe's own `number` -- a check "for invoice ___" is matched
			// against this, whichever rail it came from.
			label: 'Reference',
			accessor: (invoice: PracticeInvoice) => invoice.reference
		},
		{
			label: 'Amount',
			accessor: (invoice: PracticeInvoice) => formatAmount(invoice.amountCents),
			numeric: true
		},
		{ label: 'Status', accessor: (invoice: PracticeInvoice) => invoiceStatusLabel(invoice.status) },
		{
			// #271: local and Stripe Invoices share one figure, so the rail
			// is shown per row rather than as a second book.
			label: 'Billed via',
			accessor: (invoice: PracticeInvoice) => (invoice.billingMode === 'by_hand' ? 'By hand' : 'Stripe')
		},
		{
			label: 'Billed',
			accessor: (invoice: PracticeInvoice) => new Date(invoice.createdAt).toLocaleDateString()
		},
		{
			// #768. "Payment due", not "Due date": a due date in this domain
			// is the pregnancy's (ADR-0015), and the two must not share a
			// word. The lateness is derived here, from the row's own date
			// against now, so it stays right in a tab left open overnight.
			label: 'Payment due',
			accessor: (invoice: PracticeInvoice) => dueLabel(invoice, new Date())
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

<Link href={invoicesHref} label="All invoices" variant="chip" current={!data.isNarrowedToOverdue} />
<Link
	href="{invoicesHref}?overdue=true"
	label="Overdue ({data.overdueCount})"
	variant="chip"
	current={data.isNarrowedToOverdue}
/>

<!--
	#270: a standing fact about the Practice, from the same aggregate
	field the totals above already carry -- an empty book looks identical
	whether nobody has billed anything yet or Clients cannot pay this
	Practice at all, so this is the only thing on the page that tells the
	two apart.
-->
{#if !data.clientsCanPay}
	<Notice variant="info" message={clientsCannotPayMessage} />
{/if}

<DataTable
	{columns}
	rows={invoices.items}
	rowHref={engagementHref}
	hasMore={invoices.hasMore}
	onLoadMore={() => invoices.loadMore()}
	isLoadingMore={invoices.isLoadingMore}
	emptyMessage={data.isNarrowedToOverdue
		? 'Nothing is overdue. Every unpaid invoice is still within its payment terms.'
		: 'No invoices yet. One appears here as soon as a contract is billed.'}
/>

{#if invoices.loadMoreError}
	<Notice message={invoices.loadMoreError} variant="error" />
{/if}
