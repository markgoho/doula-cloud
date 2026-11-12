<script lang="ts">
	/*
	 * The Practice-wide Invoice list (#265, gap RA-G7). Before it, an
	 * Invoice was reachable only inside one Engagement's Contract, so
	 * "who owes us money" meant opening every Engagement in turn.
	 *
	 * The frame is `ListPage`'s (#1576), the totals a `SummaryList`, the
	 * filter a `FilterBar` and the book a `DataTable`, the pieces the
	 * Clients list is built from (#2746).
	 */
	import { untrack } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { afterNavigate, beforeNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import {
		cardPaymentsWait,
		formatAmount,
		INVOICES_RESULT_RULER,
		invoiceDateLine,
		invoicesResultSummary,
		invoiceStateBadge,
		invoiceStateWord,
		invoicesViewOf,
		invoicesViewQuery,
		loadPracticeInvoices,
		PAYMENT_ON_ITS_WAY_STATUS,
		type InvoicesView,
		type PracticeInvoice,
		type PracticeInvoicePage
	} from '#lib/invoice.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import { refreshSessionIfClientsCanPayChanged } from '#lib/clientsCanPaySession.js';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PracticeInvoiceListData } from '#lib/invoice.js';
	import type { PracticeSession } from '../+layout.js';
	import { isOwner, isOwnerOrAdmin } from '#lib/roles.js';

	/*
	 * `data` is the read once it lands (#2255). Until then the screen draws
	 * what the URL already says -- the totals' labels and the filter -- with
	 * placeholders where the figures and the rows will land.
	 */
	let { data }: { data?: PracticeInvoiceListData } = $props();

	const view = $derived(invoicesViewOf(page.url.searchParams));
	const session = $derived((page.data as { session?: PracticeSession }).session);
	// #2200: Stripe not being connected stops a card payment, never a
	// Practice that bills by hand, so that one is told nothing about it.
	const isWaitingOnStripe = $derived(session?.clientsCanPay === false && session.billingMode !== 'by_hand');
	// An Owner, an Admin and an employed Doula read this screen;
	// cardPaymentsWait words it for which one she is.
	const cardPaymentsWaitWords = $derived(
		session ? cardPaymentsWait({ isOwner: isOwner(session), isOwnerOrAdmin: isOwnerOrAdmin(session) }) : undefined
	);
	const paymentsSettingsHref = $derived(
		resolve('/practices/[practiceId]/settings/payments', { practiceId: page.params.practiceId! })
	);

	/*
	 * #2746: the typed search can be a Client's name, so it is in no URL
	 * (#1829). It rides in the history entry's state (`goto(url, { state })`),
	 * as the Clients list's Name does: Back and Forward restore it, a reload
	 * drops it. `load` never sees history state, so a searched read is this
	 * screen's own, a POST with the search in its body.
	 */
	const appliedSearch = $derived((page.state.invoicesSearch ?? '').trim());
	type ReadKey = { view: InvoicesView; search: string };
	const isSameKey = (a: ReadKey | undefined, b: ReadKey) => a?.view === b.view && a.search === b.search;
	const wanted = $derived<ReadKey>({ view, search: appliedSearch });

	/*
	 * What the rows on screen were read under, and the latest page read for
	 * them. While a newer filter loads the rows still answer to the last one,
	 * so the result line and the empty state never describe rows that are not
	 * there, and the list and Apply are marked busy (as on the Clients list).
	 *
	 * The totals are the latest page's: the BFF returns the whole book's
	 * figures on every page, so a "Load more" after an Invoice was paid
	 * brings the figures up to date with the rows, and paging never makes
	 * "Clients owe" look like it is only what has been scrolled to.
	 */
	let shown = $state.raw<ReadKey>();
	let book = $state.raw<PracticeInvoicePage>();
	// The searched read that failed, so the list is not left marked busy.
	let failedKey = $state.raw<ReadKey>();
	let isApplying = $state(false);
	const isBusy = $derived(isApplying || (shown !== undefined && !isSameKey(shown, wanted) && !isSameKey(failedKey, wanted)));

	// Grows from the landed read's first page (#446). The list owns its own
	// cursor, in-flight guard and error text.
	const invoices = new PaginatedList<PracticeInvoice>({
		first: { items: [], hasMore: false },
		// Every later page asks for the view and the search its rows were read
		// under (#768) -- otherwise "load more" on an overdue list would append
		// the whole book underneath it, and its cursor would be read in the
		// wrong order (#2746).
		loadPage: async (cursor) => {
			const under = shown!;
			const next = await loadPracticeInvoices(apiFetchWithSession, page.params.practiceId!, cursor, under.view, under.search);
			if (shown === under) book = next;
			return next;
		},
		failureMessage: 'Failed to load more invoices'
	});

	function adopt(first: PracticeInvoicePage, key: ReadKey) {
		invoices.reset(first);
		book = first;
		shown = key;
		refreshSessionIfClientsCanPayChanged(session ?? {}, first.clientsCanPay);
	}

	// #2254: a narrowing re-runs `load` without remounting this screen. The
	// load's read is the list while no search is applied, and again once the
	// search is removed, though the address and so the read are unchanged.
	let adopted: PracticeInvoiceListData | undefined;
	$effect(() => {
		const landed = data;
		if (!landed || appliedSearch !== '') return;
		untrack(() => {
			if (landed === adopted && shown?.search === '') return;
			adopted = landed;
			adopt(landed, { view: landed.view, search: '' });
		});
	});

	// A searched read, the latest asked for winning.
	let latestSearch = 0;
	$effect(() => {
		const key = wanted;
		if (key.search === '') return;
		const thisSearch = ++latestSearch;
		untrack(() => void readSearch(key, thisSearch));
	});
	async function readSearch(key: ReadKey, thisSearch: number) {
		try {
			const first = await loadPracticeInvoices(apiFetchWithSession, page.params.practiceId!, undefined, key.view, key.search);
			if (thisSearch === latestSearch) adopt(first, key);
		} catch {
			if (thisSearch === latestSearch) failedKey = key;
		}
	}
	const hasSearchFailed = $derived(isSameKey(failedKey, wanted) && !isSameKey(shown, wanted));

	const isBookEmpty = $derived(
		shown !== undefined && shown.view === 'all' && shown.search === '' && invoices.items.length === 0
	);

	const invoiceCount = (count: number) => `on ${count} ${count === 1 ? 'Invoice' : 'Invoices'}`;

	/*
	 * Four short lines read before a row (#2746): what Clients owe, the
	 * overdue part of it (#768: a narrowing of that figure, never a book
	 * beside it, so its label says so), what Clients have paid, and the part
	 * of that which went back (#1009: a refunded Invoice stays paid and still
	 * counts in full, so it is said the same way). Each is a label, then its
	 * figure, then a few words; the figures end on one edge. Before the read
	 * lands, the words hold their place unseen so nothing moves when the
	 * figures arrive.
	 */
	type Total = { label: string; value: string; figure: string; note: string };
	function totalRow(label: string, cents: number | undefined, note: string): Total {
		const figure = cents === undefined ? '' : formatAmount(cents);
		return { label, figure, note, value: `${figure} ${note}`.trim() };
	}
	const totals = $derived([
		totalRow('Clients owe', book?.outstandingCents, invoiceCount(book?.outstandingCount ?? 10)),
		totalRow('Of that, overdue', book?.overdueCents, invoiceCount(book?.overdueCount ?? 10)),
		totalRow('Clients have paid', book?.paidCents, ''),
		totalRow('Of that, returned', book?.refundedCents, 'to Clients')
	]);

	/*
	 * #768, #2746: the view is in the address, not in memory the page holds
	 * -- the URL is the state, so it survives a reload and a back button.
	 * One radio choice, since overdue is a part of unpaid and two boxes would
	 * offer a pair that means only one of them. The radios are named `show`,
	 * so the form reaches the same address with no script running.
	 */
	const invoicesHref = $derived(
		resolve('/practices/[practiceId]/invoices', { practiceId: page.params.practiceId! })
	);

	// Short on the radios, which sit under the bar's "Choose which Invoices
	// to show", so the three and Apply take two lines on a phone; whole on
	// the applied tag, which stands alone.
	const VIEW_OPTIONS: readonly { value: InvoicesView; label: string; tag: string }[] = [
		{ value: 'all', label: 'Every Invoice', tag: 'Every Invoice' },
		{ value: 'unpaid', label: 'Unpaid', tag: 'Unpaid Invoices' },
		{ value: 'overdue', label: 'Overdue', tag: 'Overdue Invoices' }
	];

	// What the radios and the field hold before Apply, put back in step with
	// the address and the history entry after every navigation. A field the
	// person typed in since the navigation began is hers: the seed leaves it.
	let draftView = $state<InvoicesView>('all');
	let draftSearch = $state('');
	let isSearchTouched = false;
	let filterBar = $state<{ focusStatus: () => void }>();
	let isFocusPending = $state(false);
	let lastKey: ReadKey | undefined;
	function seedDrafts() {
		draftView = view;
		if (!isSearchTouched) draftSearch = appliedSearch;
	}
	seedDrafts();
	beforeNavigate(() => (isSearchTouched = false));
	afterNavigate(() => {
		seedDrafts();
		// A change of filter lands on the result line once its list has
		// loaded, never on first arrival, where the route announcer speaks.
		if (lastKey !== undefined && !isSameKey(lastKey, wanted)) isFocusPending = true;
		lastKey = wanted;
	});
	$effect(() => {
		if (!isFocusPending || !isSameKey(shown, wanted)) return;
		isFocusPending = false;
		filterBar?.focusStatus();
	});

	// Every move to another filter: a page already being added is abandoned,
	// so it never lands in the new list, and Apply answers the press at once.
	async function showFilter(href: string, search: string) {
		invoices.abandon();
		isApplying = true;
		try {
			await goto(href, { state: { invoicesSearch: search } });
		} finally {
			isApplying = false;
		}
	}

	async function applyFilters(event: SubmitEvent) {
		event.preventDefault();
		await showFilter(`${invoicesHref}${invoicesViewQuery(draftView)}`, draftSearch.trim());
	}

	/*
	 * The GOV.UK "selected filters": one removable tag for the view and one
	 * for the search, each a link to the list minus that one. The search is in
	 * no address, so removing the view hands it on in the history entry's
	 * state, and removing the search hands on an empty one.
	 */
	type AppliedTag = { text: string; href: string; search: string };
	const appliedTags = $derived.by(() => {
		const tags: AppliedTag[] = [];
		const viewTag = VIEW_OPTIONS.find((option) => option.value === view && view !== 'all')?.tag;
		if (viewTag) tags.push({ text: viewTag, href: invoicesHref, search: appliedSearch });
		if (appliedSearch) {
			tags.push({ text: `Matching "${appliedSearch}"`, href: `${invoicesHref}${invoicesViewQuery(view)}`, search: '' });
		}
		return tags;
	});

	function removeTag(event: MouseEvent, tag: AppliedTag) {
		event.preventDefault();
		void showFilter(tag.href, tag.search);
	}

	const resultSummary = $derived(
		shown && book
			? invoicesResultSummary(shown.view, invoices.items.length, invoices.hasMore, book, shown.search)
			: ''
	);

	// The result line's id, which the table is described by.
	const RESULT_ID = 'invoices-result';

	const everyInvoice = $derived({ label: 'Show every Invoice', href: invoicesHref });

	/*
	 * The money said once (DESIGN.md): the amount and its state, in the
	 * Clients list's words ("owed", "12 days overdue", through
	 * `invoiceStateWord`). Owed is the amber badge and overdue the red one;
	 * every other state is a plain word (`invoiceStateBadge`).
	 * #2225: a payment on its way comes first, in the words the Engagement's
	 * Invoices section uses.
	 */
	function stateWord(invoice: PracticeInvoice): string {
		return invoice.paymentProcessing ? PAYMENT_ON_ITS_WAY_STATUS.toLowerCase() : invoiceStateWord(invoice);
	}

	// #271: local and Stripe Invoices share one book, so the rail is said per
	// row, beside the reference it decides the shape of.
	const billedLabel = (invoice: PracticeInvoice) => (invoice.billingMode === 'by_hand' ? 'By hand' : 'Stripe');
	const billedSpoken = (invoice: PracticeInvoice) => (invoice.billingMode === 'by_hand' ? ', billed ' : ', billed through ');

	const NO_PAYMENT_DUE = 'No payment due';

	function dateText(invoice: PracticeInvoice): string {
		const line = invoiceDateLine(invoice);
		return line ? `${line.lead} ${line.day}` : NO_PAYMENT_DUE;
	}

	/*
	 * Five columns. The Client takes the room the others leave and wraps
	 * there (`columnWidths="fill"`); every other column is as wide as its
	 * values on one line. The Money comes second, so a phone's record card
	 * reads the Client, then the amount and its state, then the rest. The
	 * days run in the order they happen: raised, then the one that matters
	 * for the row's state, with its kind in its own words.
	 */
	const columns = [
		{ label: 'Client', accessor: (invoice: PracticeInvoice) => invoice.clientName, grows: true },
		{
			label: 'Money',
			numeric: true,
			accessor: (invoice: PracticeInvoice) => `${formatAmount(invoice.amountCents)}, ${stateWord(invoice)}`,
			content: moneyCell
		},
		{
			// #271: a by-hand Invoice's own per-Practice sequence, or Stripe's
			// own `number` -- a check "for invoice ___" is matched against this.
			label: 'Reference',
			accessor: (invoice: PracticeInvoice) => `${invoice.reference}${billedSpoken(invoice)}${billedLabel(invoice)}`,
			content: referenceCell
		},
		{
			// #2146: the day raised, in the Practice's zone -- the Raised day
			// the Engagement's own Invoices section shows.
			label: 'Raised',
			accessor: (invoice: PracticeInvoice) => formatCalendarDay(invoice.raisedOn),
			content: raisedCell
		},
		{ label: 'Due or paid', accessor: dateText, content: dateCell }
	];

	// #2240: to the Invoice itself in its Engagement's Invoices section,
	// not the top of the Engagement.
	function engagementHref(invoice: PracticeInvoice): string {
		const engagement = resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId: invoice.engagementId
		});
		return `${engagement}#invoice-${invoice.id}`;
	}

	// Two Invoices of one Client are two links a list of links tells apart.
	const rowLinkLabel = (invoice: PracticeInvoice) => `${invoice.clientName}, ${invoice.reference}`;
</script>

{#snippet moneyCell(invoice: PracticeInvoice, view: DataTableView)}
	<!-- In the table, the amount and its state in two tracks, so amounts end at one place whatever the state says; on a card, the state follows the amount. -->
	{@const badge = invoiceStateBadge(invoice)}
	<span class={view === 'table' ? 'money' : 'record-money'}><span class="no-break">{formatAmount(invoice.amountCents)}</span><span class="state"><span class="visually-hidden">&#44;&#32;</span>{#if badge}<Badge label={stateWord(invoice)} variant={badge} />{:else}{stateWord(invoice)}{/if}</span></span>
{/snippet}

{#snippet referenceCell(invoice: PracticeInvoice)}
	<span class="no-break">{invoice.reference}<span class="visually-hidden">{billedSpoken(invoice)}</span><span class="separator" aria-hidden="true">·</span><span class="lead">{billedLabel(invoice)}</span></span>
{/snippet}

{#snippet dateCell(invoice: PracticeInvoice, view: DataTableView)}
	{@const line = invoiceDateLine(invoice)}
	{#if line}<span class="date" class:tracked={view === 'table'}><span class="lead">{line.lead}</span> <time datetime={line.on}>{line.day}</time></span>{:else}<span aria-hidden="true">—</span><span class="visually-hidden">{NO_PAYMENT_DUE}</span>{/if}
{/snippet}

{#snippet raisedCell(invoice: PracticeInvoice, view: DataTableView)}
	<!-- A card's day says its kind where the table's header does not stand over it. -->
	{#if view === 'record'}<span class="lead" aria-hidden="true">Raised&nbsp;</span>{/if}<time datetime={invoice.raisedOn}>{formatCalendarDay(invoice.raisedOn)}</time>
{/snippet}

{#snippet total(row: { figure: string; note: string })}
	{#if book}
		<span class="figure">{row.figure}</span>{#if row.note}<span class="note">{` ${row.note}`}</span>{/if}
	{:else}
		<span class="figure"><Skeleton variant="text" lines={1} /></span>{#if row.note}<span class="note reserve" aria-hidden="true">{` ${row.note}`}</span>{/if}
	{/if}
{/snippet}

<!-- The × comes first, as on the MOJ filter's tags; the name says what pressing it does. -->
{#snippet applied()}
	{#each appliedTags as tag (tag.text)}
		<li>
			<Link
				href={tag.href}
				label={tag.text}
				variant="chip"
				icon="x"
				ariaLabel={`Remove filter: ${tag.text.replaceAll('"', '')}`}
				onClick={(event) => removeTag(event, tag)}
			/>
		</li>
	{/each}
{/snippet}

<!-- Folded behind "More filters" in a narrow bar, so a phone's first screen still holds the first Invoice; beside the view where the bar is wide. Unnamed, so a Client's name never reaches the address (#1829). -->
{#snippet searchField()}
	<div class="search">
		<LabeledField label="Reference or Client" width={20}>
			{#snippet children({ id, describedBy, invalid, width })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					{width}
					type="search"
					maxlength={100}
					autocomplete="off"
					value={draftSearch}
					onInput={(value) => {
						isSearchTouched = true;
						draftSearch = value;
					}}
				/>
			{/snippet}
		</LabeledField>
	</div>
{/snippet}

{#snippet content()}
	<!--
		#2015: a book with no invoice in it has nothing to total and nothing
		to narrow, so zeros and a filter bar would answer no question. The
		narrowed list keeps both even when it is empty: the bar is its way
		back to the whole book.
	-->
	{#if !isBookEmpty}
		<div class="totals">
			<SummaryList rows={totals} value={total} isCompact />
		</div>
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

	<!-- The bar and the list it narrows are one group, held closer than the page's sections, as on the Clients list. -->
	<stack-l space="var(--space-related)">
		{#if !isBookEmpty}
			<FilterBar
				bind:this={filterBar}
				action={invoicesHref}
				onSubmit={applyFilters}
				legend="Choose which Invoices to show"
				status={resultSummary}
				statusRuler={INVOICES_RESULT_RULER}
				statusId={RESULT_ID}
				empty={shown !== undefined && invoices.items.length === 0}
				clearHref={appliedTags.length > 0 ? invoicesHref : undefined}
				applied={appliedTags.length > 0 ? applied : undefined}
				holdsAppliedRow
				folded={searchField}
				foldsFieldsOnly
				{isBusy}
			>
				<!-- Beside Apply, wrapping under it only where the row runs out. The bar's legend names the bar: these radios, then the field. -->
				<RadioGroup
					name="show"
					options={VIEW_OPTIONS}
					value={draftView}
					onChange={(chosen) => (draftView = chosen)}
					isInline
				/>
			</FilterBar>
		{/if}

		{#if hasSearchFailed}
			<Notice message="Your search did not run. Nothing you saved is lost." variant="error" />
		{/if}

		{#if shown}
			<div class="invoices-table">
				<DataTable
					{columns}
					rows={invoices.items}
					rowHref={engagementHref}
					{rowLinkLabel}
					hasMore={invoices.hasMore}
					onLoadMore={() => invoices.loadMore()}
					isLoadingMore={invoices.isLoadingMore}
					emptyMessage={isBookEmpty
						? 'No Invoices yet. One appears here as soon as a Contract is billed.'
						: resultSummary}
					emptyAction={isBookEmpty ? undefined : everyInvoice}
					caption="Invoices"
					describedBy={isBookEmpty ? undefined : RESULT_ID}
					columnWidths="fill"
					recordLayout="compact"
					{isBusy}
				/>
			</div>
		{:else}
			<Skeleton variant="row" lines={6} label="Loading your Invoices" />
		{/if}
	</stack-l>

	{#if invoices.loadMoreError}
		<Notice message={invoices.loadMoreError} variant="error" />
	{/if}
{/snippet}

<ListPage title="Invoices" {content} />

<style>
	@layer components {
		/* The figures end on one edge, the widest a four-figure book's total
		   says, and the words after them start on one edge too. */
		.figure {
			display: inline-block;
			min-inline-size: 9ch;
			font-variant-numeric: tabular-nums;
			text-align: end;
			vertical-align: bottom;
		}

		.note {
			color: var(--color-on-surface-variant);
			font-size: var(--text-meta-size);
		}

		.reserve {
			visibility: hidden;
		}

		/* The invoice list's content floor (#1945, #1981), measured on this
		   route's typical pilot screen, not its hostile one -- the founder's
		   answer of 2026-10-25 on #1981. #2746 cut its eight columns to five,
		   and the floor check measured the five at 997px on 2026-11-12 in WSL's
		   Linux/Chromium (it was 1201px with eight), so the floor moved down to
		   62.3rem. `DataTable.svelte`'s floor comment says why the floor is the
		   route's own. */
		.invoices-table {
			container: invoices-table / inline-size;
			/* The Money column's right edge, written once: the amounts end a
			   state track and a gap before the cell's inset, and the header ends
			   where they do. The track holds the longest state, "payment on its
			   way". */
			--money-state-track: 21ch;
			--money-state-gap: var(--space-1);
			--data-table-cell-inset: var(--space-2);
		}

		.invoices-table > :global(*) {
			font-size: var(--text-body-size);
			--data-table-table-display: none;
			--data-table-records-display: block;
		}

		@container invoices-table (min-width: 62.3rem) {
			.invoices-table > :global(*) {
				--data-table-table-display: table;
				--data-table-records-display: none;
			}
		}

		.invoices-table :global(th.numeric) {
			padding-inline-end: calc(var(--data-table-cell-inset) + var(--money-state-track) + var(--money-state-gap));
		}

		/* The state's badge is a pill taller than a line of text by its own
		   block padding; this margin takes that padding back, so a row with a
		   badge is as tall as a row without one. */
		.money {
			margin-block: calc(var(--space-1) / -2);
			display: inline-grid;
			grid-template-columns: auto var(--money-state-track);
			column-gap: var(--money-state-gap);
			white-space: nowrap;
		}

		/* A state with no badge is a plain word, quieter than its amount. */
		.state {
			color: var(--color-on-surface-variant);
			text-align: start;
		}

		.record-money {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1);
		}

		/* The lead word and the day in two tracks in the table, so every day
		   in the column starts at one place whichever word leads it. */
		.date.tracked {
			white-space: nowrap;
			display: inline-grid;
			grid-template-columns: 11ch auto;
			column-gap: var(--space-1);
		}

		.lead {
			color: var(--color-on-surface-variant);
		}

		.separator {
			padding-inline: var(--space-1);
			color: var(--color-on-surface-variant);
		}

		.no-break {
			white-space: nowrap;
		}
	}
</style>
