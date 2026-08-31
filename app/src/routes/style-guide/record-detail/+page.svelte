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

	/*
	 * The longest realistic value, not a representative one (ADR-0025): the
	 * title is a Client's full legal name, the doulas are named in full, and
	 * the summary carries a four-figure package -- what a Practice's own
	 * record actually holds.
	 */
	const visits: Visit[] = [
		{
			date: '18 February 2027',
			doula: 'Persephone Adeyemi-Wollstonecraft',
			kind: 'Prenatal visit at home'
		},
		{
			date: '4 March 2027',
			doula: 'Renata Chiamaka Okonkwo-Adeyemi',
			kind: 'Prenatal visit at home'
		},
		{ date: '21 March 2027', doula: 'Unassigned', kind: 'Postpartum visit at home' }
	];

	const summaryItems = [
		{ label: 'Due date', value: '2 April 2027' },
		{ label: 'Lead doula', value: 'Persephone Adeyemi-Wollstonecraft' },
		{ label: 'Package', value: 'Full birth support, $4,250.00' }
	];

	const noop = () => {};
</script>

{#snippet summary()}
	<DescriptionList items={summaryItems} />
{/snippet}

{#snippet actions()}
	<Badge label="Engagement active since 12 January 2027" variant="success" />
	<Button label="Message" variant="secondary" size="sm" onClick={noop} />
	<Button label="Send this invoice to the Client" size="sm" onClick={noop} />
{/snippet}

{#snippet visitsSection()}
	<DataTable columns={visitColumns} rows={visits} emptyMessage="No visits booked yet." />
{/snippet}

{#snippet birthPlanSection()}
	<DescriptionList
		items={[
			{
				label: 'Pain relief',
				value: 'Wants to try without an epidural, and to be asked rather than offered'
			},
			{ label: 'Support people', value: 'Her partner and her mother, for the whole labour' }
		]}
	/>
{/snippet}

{#snippet contractSection()}
	<stack-l space="var(--space-3)">
		<Text
			text="Signed 12 January 2027 by Anne-Marie Ochieng-Whitfield."
			step="body-sm"
			tone="variant"
		/>
		<cluster-l space="var(--space-3)">
			<Button label="View contract" variant="secondary" size="sm" onClick={noop} />
		</cluster-l>
	</stack-l>
{/snippet}

{#snippet invoicesSection()}
	<Text
		text="One invoice outstanding — $4,250.00, due 15 March 2027."
		step="body-sm"
		tone="variant"
	/>
{/snippet}

<!--
	`isContentsShown` is on here because the contents region is the part
	worth looking at; with it off this is the same page without the rail.
	A second instance would put a second <h1> on one demo page.
-->
<RecordDetail
	title="Anne-Marie Ochieng-Whitfield"
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
