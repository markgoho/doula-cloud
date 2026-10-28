<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isLoading?: boolean;
	}

	/*
	 * The record before it loads (#2248): its title is the Client's name,
	 * the data itself, so a heading-shaped bar holds its line.
	 */
	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Record detail, loading', props: { isLoading: true } }
	];
</script>

<script lang="ts">
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import EmptyState from '#lib/components/molecules/EmptyState.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
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
			date: 'Feb 18, 2027',
			doula: 'Persephone Adeyemi-Wollstonecraft',
			kind: 'Prenatal visit at home'
		},
		{
			date: 'Mar 4, 2027',
			doula: 'Renata Chiamaka Okonkwo-Adeyemi',
			kind: 'Prenatal visit at home'
		},
		{ date: 'Mar 21, 2027', doula: 'Unassigned', kind: 'Postpartum visit at home' }
	];

	const summaryItems = [
		{ label: 'Due date', value: 'Apr 2, 2027' },
		{ label: 'Lead doula', value: 'Persephone Adeyemi-Wollstonecraft' },
		{ label: 'Package', value: 'Full birth support, $4,250.00' }
	];

	const noop = () => {};

	let { isLoading = false }: Properties = $props();
</script>

{#snippet summary()}
	<SummaryList rows={summaryItems} />
{/snippet}

{#snippet actions()}
	<Badge label="Engagement active since Jan 12, 2027" variant="success" />
	<Button label="Message" variant="secondary" size="sm" onClick={noop} />
	<Button label="Send this invoice to the Client" size="sm" onClick={noop} />
{/snippet}

{#snippet visitsSection()}
	<DataTable columns={visitColumns} rows={visits} emptyMessage="No visits booked yet." />
{/snippet}

{#snippet birthPlanSection()}
	<SummaryList
		rows={[
			{
				label: 'Pain relief',
				value: 'Wants to try without an epidural, and to be asked rather than offered'
			},
			{ label: 'Support people', value: 'My partner and my mother, for the whole labor' }
		]}
	/>
{/snippet}

{#snippet contractSection()}
	<stack-l space="var(--space-3)">
		<Text
			text="Signed Jan 12, 2027 by Anne-Marie Ochieng-Whitfield."
			step="body-sm"
			tone="variant"
		/>
		<cluster-l space="var(--space-3)">
			<Button label="View contract" variant="secondary" size="sm" onClick={noop} />
		</cluster-l>
	</stack-l>
{/snippet}

{#snippet invoicesSection()}
	<!-- An empty section keeps its heading and places its own EmptyState
	     (#2015). -->
	<EmptyState message="No Invoices yet." />
{/snippet}

<!--
	`isContentsShown` is on here because the contents region is the part
	worth looking at; with it off this is the same page without the rail.
	A second instance would put a second <h1> on one demo page.
-->
<RecordDetail
	title={isLoading ? '' : 'Anne-Marie Ochieng-Whitfield'}
	loading={isLoading ? 'Loading the Client' : undefined}
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
