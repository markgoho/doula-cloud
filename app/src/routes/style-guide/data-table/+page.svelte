<script lang="ts">
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
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

{#snippet removeAction(client: Client)}
	<!-- #515: a bare "Remove" reads the same on every row -- the real Staff
	     page joins each Button to a visually-hidden sibling naming the row,
	     which this demo mirrors along with the three button labels below. -->
	<Button
		label="Remove"
		variant="destructive"
		size="sm"
		describedBy="remove-{client.name}"
		onClick={noop}
	/>
	<span class="visually-hidden" id="remove-{client.name}">{client.name}</span>
{/snippet}

{#snippet staffActions(member: Member)}
	<Button
		label="Edit membership"
		variant="secondary"
		size="sm"
		describedBy="{member.email}-edit"
		onClick={noop}
	/>
	<span class="visually-hidden" id="{member.email}-edit">{member.name}</span>
	<Button
		label="End sessions everywhere"
		variant="destructive"
		size="sm"
		describedBy="{member.email}-end-sessions"
		onClick={noop}
	/>
	<span class="visually-hidden" id="{member.email}-end-sessions">{member.name}</span>
	<Button
		label="Remove from practice"
		variant="destructive"
		size="sm"
		describedBy="{member.email}-remove"
		onClick={noop}
	/>
	<span class="visually-hidden" id="{member.email}-remove">{member.name}</span>
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
		<h2>Row actions</h2>
		<DataTable
			{columns}
			rows={clients}
			rowActions={{ label: 'Actions', content: removeAction }}
			emptyMessage="No clients yet."
		/>
	</section>
</stack-l>
