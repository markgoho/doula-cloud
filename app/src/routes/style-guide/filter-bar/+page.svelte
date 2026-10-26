<script lang="ts">
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
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

{#snippet dates()}
	<LabeledField label="From">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput {id} {describedBy} {invalid} type="date" name="from" value="2026-10-26" onInput={ignore} />
		{/snippet}
	</LabeledField>
	<LabeledField label="To">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput {id} {describedBy} {invalid} type="date" name="to" value="2026-11-25" onInput={ignore} />
		{/snippet}
	</LabeledField>
{/snippet}

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Filter bar" />
	<Text
		text="The bar that narrows a list: a GET form with a quiet legend, its fields at their own width in a row that wraps, one Apply beside the last field (none where every control acts at once), and the result as its status line, read out when it changes."
		tone="variant"
	/>

	<stack-l space="var(--space-4)">
		<h2>Two dates</h2>
		<FilterBar {action} onSubmit={stay} legend="Choose the days" status="10 births on call, 1 needing cover.">
			{@render dates()}
		</FilterBar>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>Two dates and a select</h2>
		<FilterBar {action} onSubmit={stay} legend="Narrow the schedule" status="Showing 25 scheduled visits. Load more to see the rest.">
			{@render dates()}
			<LabeledField label="Doula" width={20}>
				{#snippet children({ id, describedBy, invalid, width })}
					<Select {id} {describedBy} {invalid} {width} name="staffId" options={doulas} value="staff-1" />
				{/snippet}
			</LabeledField>
		</FilterBar>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>A toggle that acts at once</h2>
		<Text
			text="Every control takes effect the moment it changes, so there is no Apply. The Clients list's See everyone."
			tone="variant"
		/>
		<FilterBar
			{action}
			onSubmit={stay}
			legend="Choose who to show"
			status="Showing 12 Clients with work."
			isAppliedOnChange
		>
			<LabeledField label="See everyone" orientation="inline">
				{#snippet children(control)}
					<Checkbox variant="toggle" checked={false} onChange={ignore} {...control} />
				{/snippet}
			</LabeledField>
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
</stack-l>
