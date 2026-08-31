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
</script>

{#snippet removeAction()}
	<Button label="Remove" variant="destructive" size="sm" onClick={noop} />
{/snippet}

{#snippet staffActions()}
	<Button label="Edit membership" variant="secondary" size="sm" onClick={noop} />
	<Button label="End sessions everywhere" variant="destructive" size="sm" onClick={noop} />
	<Button label="Remove from practice" variant="destructive" size="sm" onClick={noop} />
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
		<h2>Row actions</h2>
		<DataTable
			{columns}
			rows={clients}
			rowActions={{ label: 'Actions', content: removeAction }}
			emptyMessage="No clients yet."
		/>
	</section>
</stack-l>
