<script lang="ts">
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';

	const columns = [
		{ label: 'Name', accessor: (row: { name: string; status: string }) => row.name },
		{ label: 'Status', accessor: (row: { name: string; status: string }) => row.status }
	];

	const rows = [
		{ name: 'Anne-Marie Ochieng-Whitfield', status: 'Stripe onboarding incomplete' },
		{ name: 'Renata Chiamaka Okonkwo-Adeyemi', status: 'Expired -- invite again or revoke' },
		{ name: 'dee marchetti', status: 'Active' }
	];
</script>

<stack-l space="var(--space-6)">
	<h1>Skeleton</h1>

	<section>
		<h2>Variants</h2>
		<stack-l space="var(--space-4)">
			<Skeleton variant="row" lines={3} label="Loading the Client list" />
			<Skeleton variant="text" lines={3} label="Loading the birth plan" />
		</stack-l>
	</section>

	<section>
		<h2>Reserving the right space</h2>
		<p>
			A skeleton earns its place by occupying the space the content is about to occupy. These two
			are the same height, so the page does not move when the data lands — the point of the whole
			component.
		</p>
		<stack-l space="var(--space-4)">
			<Skeleton variant="row" lines={3} label="Loading three Clients" />
			<DataTable {columns} {rows} emptyMessage="No Clients yet." />
		</stack-l>
	</section>
</stack-l>
