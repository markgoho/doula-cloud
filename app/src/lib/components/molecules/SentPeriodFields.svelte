<script lang="ts">
	/*
	 * The "Sent" select of a list that narrows to the day a row was sent
	 * (#2871, #2954), and the two typed days "Choose days" opens. The select
	 * applies when changed; the typed days wait for Apply.
	 */
	import Select from '../atoms/Select.svelte';
	import { SENT_PERIOD_OPTIONS, type SentPeriodChoice } from '../../sentPeriod.js';
	import DateRangeFields from './DateRangeFields.svelte';
	import LabeledField from './LabeledField.svelte';

	interface Properties {
		choice: SentPeriodChoice;
		from: string;
		to: string;
		fromError?: string;
		toError?: string;
	}

	let { choice = $bindable(), from = $bindable(), to = $bindable(), fromError, toError }: Properties = $props();
</script>

<div class="field sent">
	<LabeledField label="Sent" width={20}>
		{#snippet children({ id, describedBy, invalid, width })}
			<Select
				{id}
				{describedBy}
				{invalid}
				{width}
				name="sent"
				options={SENT_PERIOD_OPTIONS}
				value={choice}
				onChange={(chosen) => (choice = chosen as SentPeriodChoice)}
			/>
		{/snippet}
	</LabeledField>
</div>
{#if choice === 'choose'}
	<DateRangeFields bind:from bind:to {fromError} {toError} hint="Both days are included." />
{/if}
