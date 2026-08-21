<script lang="ts">
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Button from '#lib/components/atoms/Button.svelte';

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

	let hasMore = $state(true);

	function onLoadMore() {
		hasMore = false;
	}

	const noop = () => {};
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
		<h2>Row actions</h2>
		<DataTable
			{columns}
			rows={clients}
			rowActions={{ label: 'Actions', content: removeAction }}
			emptyMessage="No clients yet."
		/>
	</section>
</stack-l>
