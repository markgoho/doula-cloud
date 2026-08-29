<script lang="ts">
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	interface Visit {
		date: string;
		doula: string;
		kind: string;
	}

	const visitColumns = [
		{ label: 'Date', accessor: (visit: Visit) => visit.date },
		{ label: 'Doula', accessor: (visit: Visit) => visit.doula },
		{ label: 'Kind', accessor: (visit: Visit) => visit.kind }
	];

	const visits: Visit[] = [
		{ date: '18 February', doula: 'Priya Raman', kind: 'Prenatal' },
		{ date: '4 March', doula: 'Priya Raman', kind: 'Prenatal' },
		{ date: '21 March', doula: 'Unassigned', kind: 'Postpartum' }
	];

	const summaryItems = [
		{ label: 'Due date', value: '2 April 2026' },
		{ label: 'Lead doula', value: 'Priya Raman' },
		{ label: 'Package', value: 'Full birth support' }
	];

	const noop = () => {};
</script>

{#snippet summary()}
	<DescriptionList items={summaryItems} />
{/snippet}

{#snippet actions()}
	<Badge label="Active" variant="success" />
	<Button label="Message" variant="secondary" size="sm" onClick={noop} />
	<Button label="Send invoice" size="sm" onClick={noop} />
{/snippet}

{#snippet visitsSection()}
	<DataTable columns={visitColumns} rows={visits} emptyMessage="No visits booked yet." />
{/snippet}

{#snippet birthPlanSection()}
	<DescriptionList
		items={[
			{ label: 'Pain relief', value: 'Wants to try without an epidural' },
			{ label: 'Support people', value: 'Partner and mother' }
		]}
	/>
{/snippet}

{#snippet contractSection()}
	<stack-l space="var(--space-3)">
		<Text text="Signed 12 January 2026 by Ada Lovelace." step="body-sm" tone="variant" />
		<cluster-l space="var(--space-3)">
			<Button label="View contract" variant="secondary" size="sm" onClick={noop} />
		</cluster-l>
	</stack-l>
{/snippet}

{#snippet invoicesSection()}
	<Text text="One invoice outstanding — $450, due 15 March." step="body-sm" tone="variant" />
{/snippet}

<!--
	`isContentsShown` is on here because the contents region is the part
	worth looking at; with it off this is the same page without the rail.
	A second instance would put a second <h1> on one demo page.
-->
<RecordDetail
	title="Ada Lovelace"
	{summary}
	{actions}
	isContentsShown
	sections={[
		{ heading: 'Visits', content: visitsSection },
		{ heading: 'Birth plan', content: birthPlanSection },
		{ heading: 'Contract', content: contractSection },
		{ heading: 'Invoices', content: invoicesSection }
	]}
/>
