<script lang="ts">
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import { formatSignedQuantity } from '#lib/billing.js';

	interface Client {
		name: string;
		status: string;
	}

	const columns = [
		{ label: 'Name', accessor: (client: Client) => client.name },
		{ label: 'Status', accessor: (client: Client) => client.status }
	];

	/*
	 * The longest realistic value, not a representative one (ADR-0025): the
	 * two-column demos carry the same hostile names as the six-column one
	 * below, so a break shows up in the narrow table too rather than only
	 * in the wide one.
	 */
	const clients: Client[] = [
		{ name: 'Persephone Adeyemi-Wollstonecraft', status: 'Stripe onboarding incomplete' },
		{ name: 'Anne-Marie Ochieng-Whitfield', status: 'Expired -- invite again or revoke' }
	];

	interface CreditRow {
		origin: string;
		quantity: number;
	}

	const creditColumns = [
		{ label: 'Origin', accessor: (row: CreditRow) => row.origin },
		{
			label: 'Quantity',
			accessor: (row: CreditRow) => formatSignedQuantity(row.quantity),
			numeric: true
		}
	];

	const credits: CreditRow[] = [
		{ origin: 'Purchase of a twenty-credit block', quantity: 20 },
		{ origin: 'Engagement started with Anne-Marie Ochieng-Whitfield', quantity: -1 }
	];

	/*
	 * #905: the row link and the machine-readable instant on one column.
	 * A first column carrying `rowHref` used to render a bare link and
	 * drop its `datetimeAccessor` silently, so the combination is drawn
	 * here rather than only asserted in a spec -- inspect the first cell
	 * and the `<time datetime>` is around the link, with the link itself
	 * unchanged.
	 *
	 * This demonstrates the component's contract, NOT a recommended
	 * column order: a link named "12 Sep 2026, 10:30am" says nothing
	 * about where it goes (#513), which is why the real Practice-wide
	 * schedule leads with the Client and keeps When second.
	 */
	interface Occurrence {
		display: string;
		instant: string;
		client: string;
	}

	const occurrenceColumns = [
		{
			label: 'When',
			accessor: (row: Occurrence) => row.display,
			datetimeAccessor: (row: Occurrence) => row.instant
		},
		{ label: 'Client', accessor: (row: Occurrence) => row.client }
	];

	const occurrences: Occurrence[] = [
		{
			display: '12 Sep 2026, 10:30am',
			instant: '2026-09-12T14:30:00Z',
			client: 'Persephone Adeyemi-Wollstonecraft'
		},
		{
			display: '14 Sep 2026, 09:00am',
			instant: '2026-09-14T13:00:00Z',
			client: 'Anne-Marie Ochieng-Whitfield'
		}
	];

	let hasMore = $state(true);

	function onLoadMore() {
		hasMore = false;
	}

	const noop = () => {};

	/*
	 * The demo #508 says this page was missing: Staff's own Members table
	 * shape -- five columns plus its Actions column -- with values at the
	 * longest a real Practice produces rather than a polite length
	 * (ADR-0025). This shows the defect, it does not fix it -- the fix is
	 * #508's, and the drag surface exists to make the break watchable.
	 *
	 * The Actions column carries the same three button labels the real
	 * Staff page's memberActions snippet does (#508's own review found the
	 * six-column demo undercounted the content floor by leaving this
	 * column out entirely) -- plain buttons, not the real ConfirmDialogs,
	 * since only the natural width of the row matters here.
	 */
	interface Member {
		name: string;
		email: string;
		roles: string;
		employment: string;
		worksFrom: string;
	}

	const memberColumns = [
		{ label: 'Name', accessor: (member: Member) => member.name },
		{ label: 'Email', accessor: (member: Member) => member.email },
		{ label: 'Roles', accessor: (member: Member) => member.roles },
		{ label: 'Employment type', accessor: (member: Member) => member.employment },
		{ label: 'Works from', accessor: (member: Member) => member.worksFrom }
	];

	const members: Member[] = [
		{
			name: 'Persephone Adeyemi-Wollstonecraft',
			email: 'persephone.adeyemi-wollstonecraft@highland-midwifery-group.example.org',
			roles: 'Practice owner, Birth doula, Postpartum doula',
			employment: 'Independent contractor',
			worksFrom: 'Highland Midwifery Group, Rochester'
		},
		{
			name: 'Anne-Marie Ochieng-Whitfield',
			email: 'anne-marie.ochieng-whitfield@highland-midwifery-group.example.org',
			roles: 'Birth doula, Postpartum doula',
			employment: 'Employee',
			worksFrom: 'Highland Midwifery Group, Rochester'
		}
	];

	/*
	 * The one DataTable column a Practice writes without a length the
	 * component can predict (#542): a Client's history renders "Birth
	 * Engagement refused: <reason>", and the reason is free text. Every
	 * other column built today is a name, an address, an enum, a date or a
	 * quantity. Shrink-to-fit makes this column decide the whole table's
	 * width, so the page has to hold it -- with the sentence a Practice
	 * really writes and with #530's URL, which is the value a browser will
	 * not break on its own (ADR-0025).
	 */
	interface HistoryRow {
		when: string;
		who: string;
		what: string;
	}

	const historyColumns = [
		{ label: 'When', accessor: (row: HistoryRow) => row.when },
		{ label: 'Who', accessor: (row: HistoryRow) => row.who },
		{ label: 'What', accessor: (row: HistoryRow) => row.what }
	];

	/*
	 * The second `content` consumer #740 asks for, and the whole of what
	 * it proves: this page copies NO class from the Clients list, whose
	 * rollup was the seam's only consumer when the seam was written. The
	 * cell it lands in sizes and pads itself because the column declares
	 * `content`, not because the markup below carries a name DataTable
	 * recognizes -- so a route can put a list, a sentence, or a sentence
	 * and a link in a cell and get a cell that fits it.
	 *
	 * Two shapes, because they fail differently: `stateList` is several
	 * lines and is what a fixed row height would have clipped, while
	 * `stateNote` is one line and is what vertical padding would have
	 * pushed past the brief's 40px row. Values are the longest realistic
	 * ones, including #530's unbreakable URL (ADR-0025).
	 */
	interface StateRow {
		client: string;
		summary: string;
		lines: string[];
		note: string;
		noteHref?: string;
	}

	const stateColumns = [
		{ label: 'Client', accessor: (row: StateRow) => row.client },
		{
			label: 'Open Engagements',
			accessor: (row: StateRow) => row.summary,
			content: stateList
		},
		{ label: 'Portal invite', accessor: (row: StateRow) => row.note, content: stateNote }
	];

	const states: StateRow[] = [
		{
			client: 'Persephone Adeyemi-Wollstonecraft',
			summary: 'Two open Engagements',
			lines: [
				'Birth Engagement, contract signed, with Anne-Marie Ochieng-Whitfield',
				'Postpartum Engagement awaiting a contract, with Anne-Marie Ochieng-Whitfield',
				'Refused: https://highland-midwifery-group.example.org/policies/scheduling-and-availability#postpartum-capacity-window'
			],
			note: 'Undeliverable -- the address bounced',
			noteHref: '#blocked-email-addresses'
		},
		{
			client: 'Anne-Marie Ochieng-Whitfield',
			summary: 'No open Engagements',
			lines: [],
			note: 'Accepted'
		}
	];

	const history: HistoryRow[] = [
		{
			when: '31 August 2026, 09:14',
			who: 'Persephone Adeyemi-Wollstonecraft',
			what: 'Birth Engagement refused: we are already carrying two clients due the same fortnight and cannot promise attendance at a third birth without putting the other two at risk'
		},
		{
			when: '30 August 2026, 16:02',
			who: 'Anne-Marie Ochieng-Whitfield',
			what: 'Postpartum Engagement refused: https://highland-midwifery-group.example.org/policies/scheduling-and-availability#postpartum-capacity-window'
		},
		{
			when: '28 August 2026, 11:47',
			who: 'Persephone Adeyemi-Wollstonecraft',
			what: 'Record updated'
		}
	];
</script>

{#snippet stateList(row: StateRow)}
	<!--
		A plain `<ul>` under this page's OWN class name. Nothing about it
		is copied from the Clients list's rollup, and DataTable does not
		know the name: the cell it lands in is sized by the column having
		declared `content` at all (#740).
	-->
	{#if row.lines.length > 0}
		<ul class="state-list">
			{#each row.lines as line (line)}
				<li>{line}</li>
			{/each}
		</ul>
	{/if}
{/snippet}

{#snippet stateNote(row: StateRow)}
	<!-- A one-line content cell, which is the case a cell's own vertical
	     padding must not push past the brief's 40px row (#740). -->
	{row.note}
	{#if row.noteHref}
		<Link href={row.noteHref} label="Blocked email addresses" />
	{/if}
{/snippet}

{#snippet removeAction(client: Client, view: DataTableView)}
	<!-- #515: a bare "Remove" reads the same on every row -- the real Staff
	     page joins each Button to a visually-hidden sibling naming the row,
	     which this demo mirrors along with the three button labels below. -->
	<Button
		label="Remove"
		variant="destructive"
		size="sm"
		describedBy="{view}-remove-{client.name.replaceAll(' ', '-')}"
		onClick={noop}
	/>
	<span class="visually-hidden" id="{view}-remove-{client.name.replaceAll(' ', '-')}">{client.name}</span>
{/snippet}

{#snippet staffActions(member: Member, view: DataTableView)}
	<Button
		label="Edit membership"
		variant="secondary"
		size="sm"
		describedBy="{view}-{member.email}-edit"
		onClick={noop}
	/>
	<span class="visually-hidden" id="{view}-{member.email}-edit">{member.name}</span>
	<Button
		label="End sessions everywhere"
		variant="destructive"
		size="sm"
		describedBy="{view}-{member.email}-end-sessions"
		onClick={noop}
	/>
	<span class="visually-hidden" id="{view}-{member.email}-end-sessions">{member.name}</span>
	<Button
		label="Remove from practice"
		variant="destructive"
		size="sm"
		describedBy="{view}-{member.email}-remove"
		onClick={noop}
	/>
	<span class="visually-hidden" id="{view}-{member.email}-remove">{member.name}</span>
{/snippet}

<stack-l space="var(--space-6)">
	<h1>Data table</h1>

	<section>
		<h2>Default</h2>
		<DataTable {columns} rows={clients} emptyMessage="No clients yet." />
	</section>

	<section>
		<h2>Navigable rows</h2>
		<DataTable
			{columns}
			rows={clients}
			rowHref={(client) => `#${client.name}`}
			emptyMessage="No Clients yet. Add one and it appears here."
		/>
	</section>

	<section>
		<h2>Navigable rows whose first column is a timestamp</h2>
		<DataTable
			columns={occurrenceColumns}
			rows={occurrences}
			rowHref={(occurrence) => `#${occurrence.instant}`}
			emptyMessage="No Visits yet."
		/>
	</section>

	<section>
		<h2>Load more</h2>
		<DataTable {columns} rows={clients} {hasMore} {onLoadMore} emptyMessage="No clients yet." />
	</section>

	<section>
		<h2>Empty</h2>
		<DataTable
			{columns}
			rows={[]}
			emptyMessage="No Clients yet. Add one and it appears here."
		/>
	</section>

	<section>
		<h2>Numeric column</h2>
		<DataTable columns={creditColumns} rows={credits} emptyMessage="No credit history yet." />
	</section>

	<section>
		<h2>Staff's Members table shape, longest realistic values</h2>
		<DataTable
			columns={memberColumns}
			rows={members}
			rowActions={{ label: 'Actions', content: staffActions }}
			emptyMessage="No staff yet."
		/>
	</section>

	<section>
		<h2>A column a Practice writes, with no length the component can predict</h2>
		<DataTable columns={historyColumns} rows={history} emptyMessage="No history yet." />
	</section>

	<section>
		<h2>Columns whose cells a caller draws itself</h2>
		<DataTable
			columns={stateColumns}
			rows={states}
			emptyMessage="No Clients yet."
		/>
	</section>

	<section>
		<h2>Row actions</h2>
		<DataTable
			{columns}
			rows={clients}
			rowActions={{ label: 'Actions', content: removeAction }}
			emptyMessage="No clients yet."
		/>
	</section>
</stack-l>

<style>
	@layer components {
		/*
		 * This demo's own list treatment, written here rather than
		 * inherited from anywhere (#740). It is deliberately NOT the
		 * Clients list's rollup: a bordered separator there, a plain
		 * bullet-free stack here, so the two consumers visibly disagree
		 * about their content and still get the same cell.
		 */
		.state-list {
			display: grid;
			gap: var(--space-1);
			margin: 0;
			padding: 0;
			list-style: none;
		}
	}
</style>
