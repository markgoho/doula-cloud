<script lang="ts">
	/*
	 * PROTOTYPE (#1926) -- a Client's record: the title at `heading-lg`,
	 * the sections at `heading`, a card heading at `subheading`, body and
	 * body-sm text, and the record page's own `label` step.
	 */
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import { visits } from './fixtures.js';

	type Visit = (typeof visits)[number];

	const visitColumns = [
		{ label: 'Date', accessor: (visit: Visit) => visit.date },
		{ label: 'Doula', accessor: (visit: Visit) => visit.doula },
		{ label: 'Kind', accessor: (visit: Visit) => visit.kind }
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
	<DataTable hasMore={false} columns={visitColumns} rows={visits} emptyMessage="No visits booked yet." />
{/snippet}

{#snippet birthPlanSection()}
	<stack-l space="var(--space-4)">
		<Text
			text="The birth plan the Client filled in on the portal on 3 February 2027. Ask before changing anything here; the Client sees every change."
			measure
		/>
		<Heading level={3} variant="card" text="Pain relief" />
		<DescriptionList
			items={[
				{
					label: 'Preference',
					value: 'Wants to try without an epidural, and to be asked rather than offered'
				},
				{ label: 'Support people', value: 'My partner and my mother, for the whole labor' }
			]}
		/>
	</stack-l>
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
	<Text text="One invoice outstanding: $4,250.00, due 15 March 2027." step="body-sm" tone="variant" />
	<Text text="Last reminder sent 1 March 2027, 9:14am." step="meta" tone="muted" />
{/snippet}

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
