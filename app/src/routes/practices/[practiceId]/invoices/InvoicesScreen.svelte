<script lang="ts">
	/*
	 * The Practice-wide Invoice list (#265, gap RA-G7). Before it, an
	 * Invoice was reachable only inside one Engagement's Contract, so
	 * "who owes us money" meant opening every Engagement in turn.
	 *
	 * This page composes existing components only and writes no CSS of its
	 * own: the frame is `ListPage`'s (#1576), the totals are a
	 * `SummaryList`, the book is a `DataTable`, and each already adapts
	 * to the space it is given. That is also what keeps it inside
	 * CLAUDE.md's no-new-components block.
	 */
	import { untrack } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import {
		cardPaymentsWait,
		dueLabel,
		formatAmount,
		invoiceRowStatusLabel,
		loadPracticeInvoices,
		type PracticeInvoice
	} from '#lib/invoice.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import { refreshSessionIfClientsCanPayChanged } from '#lib/clientsCanPaySession.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PracticeInvoiceListData } from '#lib/invoice.js';
	import type { PracticeSession } from '../+layout.js';
	import { isOwner, isOwnerOrAdmin } from '#lib/roles.js';

	/*
	 * `data` is the read once it lands (#2255). Until then the screen draws
	 * what the URL already says -- the totals' labels and the two links --
	 * with placeholders where the figures and the rows will land.
	 */
	let { data }: { data?: PracticeInvoiceListData } = $props();

	const isNarrowedToOverdue = $derived(page.url.searchParams.get('overdue') === 'true');
	const session = $derived((page.data as { session?: PracticeSession }).session);
	// #2200: Stripe not being connected stops a card payment, never a
	// Practice that bills by hand, so that one is told nothing about it.
	const isWaitingOnStripe = $derived(session?.clientsCanPay === false && session.billingMode !== 'by_hand');
	// Owner/Admin read this screen (+page.ts), so each gets a way to
	// Getting paid; cardPaymentsWait words it for which one she is.
	const cardPaymentsWaitWords = $derived(
		session ? cardPaymentsWait({ isOwner: isOwner(session), isOwnerOrAdmin: isOwnerOrAdmin(session) }) : undefined
	);
	const paymentsSettingsHref = $derived(
		resolve('/practices/[practiceId]/settings/payments', { practiceId: page.params.practiceId! })
	);

	/*
	 * Grows from the landed read's first page (#446). The list owns its own
	 * cursor, in-flight guard and error text, so re-deriving from `data` --
	 * which would drop every page appended since -- is not something this
	 * file can do by accident.
	 *
	 * #2254: a narrowing re-runs `load` without remounting this screen. The
	 * derived holds back a rerender that hands back the same read.
	 */
	const read = $derived(data);
	const invoices = new PaginatedList({
		first: untrack(() => read) ?? { items: [], hasMore: false },
		// Every later page asks for the same narrowing its rows were read
		// under (#768) -- otherwise "load more" on an overdue list would
		// append the whole book underneath it.
		loadPage: (cursor) =>
			loadPracticeInvoices(
				apiFetchWithSession,
				page.params.practiceId!,
				cursor,
				read?.isNarrowedToOverdue ?? isNarrowedToOverdue
			),
		failureMessage: 'Failed to load more invoices'
	});
	$effect(() => {
		const landed = read;
		if (!landed) return;
		untrack(() => {
			invoices.reset(landed);
			refreshSessionIfClientsCanPayChanged(session ?? {}, landed.clientsCanPay);
		});
	});

	/*
	 * The totals stay the load's, never the loaded pages': the BFF returns
	 * the whole book's figures on every page, so paging must not make
	 * "Clients owe" look like it is only what has been scrolled to.
	 */
	const isBookEmpty = $derived(
		read !== undefined && !read.isNarrowedToOverdue && invoices.items.length === 0
	);

	const summary = $derived(
		read
			? [
					{ label: 'Clients owe', value: formatAmount(read.outstandingCents) },
					{ label: 'Unpaid invoices', value: String(read.outstandingCount) },
					{ label: 'Paid', value: formatAmount(read.paidCents) },
					// #1009: a refunded Invoice stays 'paid' and still counts in
					// Paid in full, so the money that went back is its own figure
					// right beside it -- always shown, $0.00 included, so the block
					// has one shape whatever the book holds.
					{ label: 'Returned to Clients', value: formatAmount(read.refundedCents) },
					// #768: the aging figure sits beside the owed one it is a
					// slice of, so "who owes me, and for how long" is answered by
					// the same block rather than by opening a second screen.
					{ label: 'Overdue', value: formatAmount(read.overdueCents) },
					{ label: 'Overdue invoices', value: String(read.overdueCount) }
				]
			: [
					'Clients owe',
					'Unpaid invoices',
					'Paid',
					'Returned to Clients',
					'Overdue',
					'Overdue invoices'
				].map((label) => ({ label, value: '' }))
	);

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
		// #2122: Overdue once the BFF counts the row late, through the one
		// label the Engagement's Invoices section reads for every role.
		{ label: 'Status', accessor: (invoice: PracticeInvoice) => invoiceRowStatusLabel(invoice) },
		{
			// #271: local and Stripe Invoices share one figure, so the rail
			// is shown per row rather than as a second book.
			label: 'Billed via',
			accessor: (invoice: PracticeInvoice) => (invoice.billingMode === 'by_hand' ? 'By hand' : 'Stripe')
		},
		{
			// #2146: the day raised, in the Practice's zone -- the Raised
			// day the Engagement's own Invoices section shows.
			label: 'Billed',
			accessor: (invoice: PracticeInvoice) => formatCalendarDay(invoice.raisedOn)
		},
		{
			// #768. "Payment due", not "Due date": a due date in this domain
			// is the pregnancy's (ADR-0015), and the two must not share a
			// word. The day and the lateness are the BFF's, in the
			// Practice's own timezone (#1965), so the row agrees with the
			// overdue figure above it.
			label: 'Payment due',
			accessor: (invoice: PracticeInvoice) => dueLabel(invoice)
		},
		{
			// An em dash rather than a blank, so an unpaid row reads as
			// "nothing here yet" instead of a cell that failed to render.
			label: 'Paid',
			accessor: (invoice: PracticeInvoice) =>
				invoice.paidOn ? formatCalendarDay(invoice.paidOn) : '—'
		}
	];

	function engagementHref(invoice: PracticeInvoice): string {
		return resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId: invoice.engagementId
		});
	}
</script>

{#snippet intro()}
	<Text
		text="Every Invoice this Practice has billed, newest first. Open one to reach the Engagement it belongs to."
		tone="muted"
	/>
{/snippet}

{#snippet pendingFigure()}
	<Skeleton variant="text" lines={1} />
{/snippet}

{#snippet content()}
	<!--
		#2015: a book with no invoice in it has nothing to total and nothing
		to narrow, so six zeros and two chips would answer no question. The
		narrowed list keeps both even when it is empty: the chips are its way
		back to the whole book.
	-->
	{#if !isBookEmpty}
		<SummaryList rows={summary} value={read ? undefined : pendingFigure} />

		<!-- One cluster, so the two filters sit side by side as one control
		     rather than as two rows of ListPage's stack. -->
		<cluster-l space="var(--space-3)">
			<Link href={invoicesHref} label="All invoices" variant="chip" current={!isNarrowedToOverdue} />
			<Link
				href="{invoicesHref}?overdue=true"
				label={read ? `Overdue (${read.overdueCount})` : 'Overdue'}
				variant="chip"
				current={isNarrowedToOverdue}
			/>
		</cluster-l>
	{/if}

	<!--
		#270: an empty book looks the same whether nobody has billed yet or
		Clients cannot pay this Practice at all; this tells the two apart.
		From the session, so it is drawn on the first frame and the rows
		never move under it (#2293), unless the read finds the session out of
		date (#2311).
	-->
	{#if isWaitingOnStripe && cardPaymentsWaitWords}
		<stack-l space="var(--space-related)">
			<Notice variant="info" message={cardPaymentsWaitWords.message} />
			{#if cardPaymentsWaitWords.linkLabel}
				<Link href={paymentsSettingsHref} label={cardPaymentsWaitWords.linkLabel} />
			{/if}
		</stack-l>
	{/if}

	{#if read}
		<div class="invoices-table">
			<DataTable
				{columns}
				rows={invoices.items}
				rowHref={engagementHref}
				hasMore={invoices.hasMore}
				onLoadMore={() => invoices.loadMore()}
				isLoadingMore={invoices.isLoadingMore}
				emptyMessage={read.isNarrowedToOverdue
					? 'Nothing is overdue. Every unpaid invoice is still within its payment terms.'
					: 'No Invoices yet. One appears here as soon as a Contract is billed.'}
			/>
		</div>
	{:else}
		<Skeleton variant="row" lines={6} label="Loading your Invoices" />
	{/if}

	{#if invoices.loadMoreError}
		<Notice message={invoices.loadMoreError} variant="error" />
	{/if}
{/snippet}

<ListPage title="Invoices" {intro} {content} />

<style>
	@layer components {
		/* The invoice list's content floor (#1945, #1981): below it the table's
		   cells wrap to more lines than at the old type floors, so it
		   shows its records -- the founder's answer on #1945. Measured on
		   this route's typical pilot screen, not its hostile one -- the
		   founder's answer of 2026-10-25 on #1981: a very long name or
		   address wraps one more line, and it does not change the layout.
		   Measured 2026-10-25 on the drag surface: its natural width fits
		   its column from 1125px on macOS/Chromium and from 1141px in CI's Linux/Chromium, the canonical environment, with no margin beyond it.
		   #2122 made the late row's Status read "Overdue" in place of
		   "Open", and CI's floor check measured 1156px (72.25rem) on
		   2026-10-26, so the floor moved up to it.
		   On the hostile fixture it was 109.75rem.
		   `DataTable.svelte`'s floor comment says why the floor is the
		   route's own. */
		.invoices-table {
			container: invoices-table / inline-size;
		}

		.invoices-table > :global(*) {
			font-size: var(--text-body-size);
			--data-table-table-display: none;
			--data-table-records-display: block;
		}

		@container invoices-table (min-width: 72.25rem) {
			.invoices-table > :global(*) {
				--data-table-table-display: table;
				--data-table-records-display: none;
			}
		}
	}
</style>
