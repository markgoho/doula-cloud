<script lang="ts">
	/*
	 * PROTOTYPE (#1926) -- the Clients list: the densest screen, where the
	 * table rows read `body-sm` and the timestamp column reads `meta`.
	 */
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import { clients, stay, type ClientRow } from './fixtures.js';

	let isShowingEveryone = $state(false);

	const columns = [
		{ label: 'Name', accessor: (row: ClientRow) => row.name },
		{ label: 'Email', accessor: (row: ClientRow) => row.email },
		{ label: 'Engagements', accessor: (row: ClientRow) => row.engagement },
		{ label: 'Due date', accessor: (row: ClientRow) => row.due },
		{ label: 'Portal invite', accessor: (row: ClientRow) => row.invite },
		{ label: 'Last change', accessor: (row: ClientRow) => row.updated, variant: 'meta' as const }
	];
</script>

{#snippet actions()}
	<Link href={stay('find')} label="Find or add a Client" />
{/snippet}

{#snippet content()}
	<label>
		<Checkbox
			variant="toggle"
			checked={isShowingEveryone}
			onChange={(checked) => (isShowingEveryone = checked)}
		/>
		See everyone
	</label>
	<DataTable
		{columns}
		rows={clients}
		hasMore={false}
		rowHref={(row) => stay(`client-${row.id}`)}
		emptyMessage="No Clients yet."
	/>
{/snippet}

<ListPage title="Clients" {actions} {content} />
