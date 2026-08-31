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

	const clients: Client[] = [
		{ name: 'Ada Lovelace', status: 'Active' },
		{ name: 'Grace Hopper', status: 'Prospective' }
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
		{ origin: 'Purchase', quantity: 20 },
		{ origin: 'Engagement started', quantity: -1 }
	];

	let hasMore = $state(true);

	function onLoadMore() {
		hasMore = false;
	}

	const noop = () => {};

	/*
	 * The demo #508 says this page was missing: six columns, and values at
	 * the longest a real Practice produces rather than a polite length
	 * (ADR-0025). This shows the defect, it does not fix it -- the fix is
	 * #508's, and the drag surface exists to make the break watchable.
	 */
	interface Member {
		name: string;
		email: string;
		roles: string;
		employment: string;
		worksFrom: string;
		joined: string;
	}

	const memberColumns = [
		{ label: 'Name', accessor: (member: Member) => member.name },
		{ label: 'Email', accessor: (member: Member) => member.email },
		{ label: 'Roles', accessor: (member: Member) => member.roles },
		{ label: 'Employment type', accessor: (member: Member) => member.employment },
		{ label: 'Works from', accessor: (member: Member) => member.worksFrom },
		{ label: 'Joined', accessor: (member: Member) => member.joined }
	];

	const members: Member[] = [
		{
			name: 'Persephone Adeyemi-Wollstonecraft',
			email: 'persephone.adeyemi-wollstonecraft@highland-midwifery-group.example.org',
			roles: 'Practice owner, Birth doula, Postpartum doula',
			employment: 'Independent contractor',
			worksFrom: 'Highland Midwifery Group, Rochester',
			joined: '1/1/2026'
		},
		{
			name: 'Ada Lovelace',
			email: 'ada@example.org',
			roles: 'Birth doula',
			employment: 'Employee',
			worksFrom: 'Rochester',
			joined: '2/14/2026'
		}
	];
</script>

{#snippet removeAction()}
	<Button label="Remove" variant="destructive" size="sm" onClick={noop} />
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
			emptyMessage="No clients yet."
		/>
	</section>

	<section>
		<h2>Load more</h2>
		<DataTable {columns} rows={clients} {hasMore} {onLoadMore} emptyMessage="No clients yet." />
	</section>

	<section>
		<h2>Empty</h2>
		<DataTable {columns} rows={[]} emptyMessage="No clients yet." />
	</section>

	<section>
		<h2>Numeric column</h2>
		<DataTable columns={creditColumns} rows={credits} emptyMessage="No credit history yet." />
	</section>

	<section>
		<h2>Six columns, longest realistic values</h2>
		<DataTable columns={memberColumns} rows={members} emptyMessage="No staff yet." />
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
