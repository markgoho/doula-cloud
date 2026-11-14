<script lang="ts">
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import DateRangeFields from '#lib/components/molecules/DateRangeFields.svelte';
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	const action = '/style-guide/filter-bar';
	const doulas = [
		{ value: '', label: 'Every doula' },
		{ value: 'staff-1', label: 'Marisol Terrazas-Whitfield' },
		{ value: 'staff-2', label: 'Bo Ng' }
	];

	// A style-guide control changes nothing beyond itself.
	function ignore() {}

	// The page stays on the page: a style-guide submit goes nowhere.
	function stay(event: SubmitEvent) {
		event.preventDefault();
	}
</script>

{#snippet switchField(label: string, name: string, checked = false, disabled = false)}
	<LabeledField {label} orientation="inline">
		{#snippet children(control)}
			<Checkbox {name} value="true" {checked} {disabled} onChange={ignore} {...control} />
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet nameField()}
	<LabeledField label="Name" width={20}>
		{#snippet children({ id, describedBy, invalid, width })}
			<TextInput {id} {describedBy} {invalid} {width} type="search" name="q" value="" onInput={ignore} />
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet doulaField(name: string, fieldWidth: 20 | 30)}
	<LabeledField label="Doula" width={fieldWidth}>
		{#snippet children({ id, describedBy, invalid, width })}
			<Select {id} {describedBy} {invalid} {width} {name} options={doulas} value="staff-1" />
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet dates()}
	<DateRangeFields from="2026-10-26" to="2026-11-25" />
{/snippet}

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Filter bar" />
	<Text
		text="The bar that narrows a list: a GET form with a quiet legend, its fields at their own width in a row that wraps, one Apply beside the last field with a Clear filters link after it (its space is held while no filter is on), and the result as its status line, read out when it changes."
		tone="variant"
	/>

	<stack-l space="var(--space-4)">
		<h2>Switches, a name and a select</h2>
		<Text
			text="The fullest bar a list builds. In a narrow column its controls collapse behind one Filters summary, which names what is on and stays closed until it is opened; in a wide one they show in full. The words are the Clients list's, which groups its first three boxes under its own Show only legend."
			tone="variant"
		/>
		<FilterBar
			{action}
			onSubmit={stay}
			clearHref={action}
			activeLabels={['Mine', 'Doula: Kimiko Nakashima']}
			legend="Choose which Clients to show"
			status="Showing 14 Clients, soonest due first."
		>
			{#snippet switches()}
				{@render switchField('Mine', 'mine', true)}
				{@render switchField('Who owe money', 'owes')}
				{@render switchField('Active, with no Contract', 'noContract')}
				{@render switchField('Also show anyone not yet in care', 'all')}
			{/snippet}
			{@render nameField()}
			{@render doulaField('doula', 30)}
		</FilterBar>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>Two dates</h2>
		<FilterBar {action} onSubmit={stay} legend="Choose the days" status="10 births on call, 1 needing cover.">
			{@render dates()}
		</FilterBar>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>Two dates and a select</h2>
		<FilterBar {action} onSubmit={stay} clearHref={action} legend="Narrow the schedule" status="Showing 25 scheduled visits. Load more to see the rest.">
			{@render dates()}
			{@render doulaField('staffId', 20)}
		</FilterBar>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>Nothing found</h2>
		<Text
			text="The list's own empty state says so on screen, so the status line is there for a screen reader only."
			tone="variant"
		/>
		<FilterBar {action} onSubmit={stay} legend="Choose the days" status="No births are on call in these days." empty>
			{@render dates()}
		</FilterBar>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>Switches kept in view, fields folded</h2>
		<Text
			text="For a bar whose switches are the filters most often wanted. In a narrow column the switches, the main field and Apply stay in view and only the rarely wanted fields fold behind More filters; in a wide one the fields sit beside Apply."
			tone="variant"
		/>
		<Text
			text="The Clients and Invoices lists draw this arrangement, so applying a filter never moves Apply or the list: the Clear filters link and a switch not drawn right now hold their space unseen, and the count has one place."
			tone="variant"
		/>
		<FilterBar {action} onSubmit={stay} legend="Choose which Clients to show" status="Showing 14 Clients, soonest due first." foldsFieldsOnly>
			{#snippet switches()}
				{@render switchField('Who owe money', 'owes')}
				<div data-holds-space aria-hidden="true" inert>
					{@render switchField('Also show anyone not yet in care', 'all', false, true)}
				</div>
			{/snippet}
			{@render nameField()}
			{#snippet folded()}
				{@render doulaField('staffId', 20)}
			{/snippet}
		</FilterBar>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>An order the reader chooses</h2>
		<Text
			text="An Order select is the last field before Apply, at every width and never folded: it is chosen with the rest and applies with Apply, and the list's own order is the empty choice, labeled with the words the list is in."
			tone="variant"
		/>
		<FilterBar
			{action}
			onSubmit={stay}
			legend="Choose which Visits to show"
			status="Showing 2 Visits, latest first."
			foldsFieldsOnly
			order={{
				options: [
					{ value: '', label: 'Soonest first' },
					{ value: 'latest', label: 'Latest first' }
				],
				value: 'latest'
			}}
		>
			{@render nameField()}
			{#snippet folded()}
				{@render doulaField('staffId', 20)}
			{/snippet}
		</FilterBar>
	</stack-l>
</stack-l>
