<script lang="ts">
	import Select from '#lib/components/atoms/Select.svelte';
	import LabeledField from './LabeledField.svelte';
	import { timezoneOptions } from '#lib/timezones.js';

	interface Properties {
		/**
		 * The IANA zone name, e.g. "America/New_York" -- the value the API
		 * stores, not a label. Bind it and send it as-is.
		 */
		value: string;
		/**
		 * Fixed by the route where the error summary has to link to this
		 * control (#467). Left generated everywhere else.
		 */
		id?: string;
		error?: string;
		/**
		 * The sentence beneath the label. Both screens that ask this
		 * question ask it for the same reason, but only one of them is
		 * changing an answer that already governs recorded work, so the
		 * consequence sentence belongs to the caller rather than here.
		 */
		hint: string;
	}

	const uid = $props.id();

	let { value = $bindable(''), id = uid, error, hint }: Properties = $props();

	/* The seven US zones, plus whatever this Practice already holds when
	   that is not one of them -- see timezoneOptions. Derived, not
	   computed once, because the stored zone arrives after the first
	   render on the settings screen. */
	const options = $derived(timezoneOptions(value));
</script>

<!--
	A native <select> over seven fixed options: the Rule of Least Power
	answer, and it needs no JavaScript to be usable. The full IANA list is
	deliberately not what a person is handed -- she knows she is on
	Eastern time, not that she is in America/New_York -- and the API,
	which checks the real database, is what actually decides whether a
	zone is a zone.

	Composed on LabeledField like WorkStateField beside it, so this field
	can say what is wrong with it in the one place that markup lives.
-->
<LabeledField {id} {error} label="What timezone does this Practice work in?" {hint}>
	{#snippet children({ id: controlId, describedBy, invalid })}
		<Select
			id={controlId}
			{describedBy}
			{invalid}
			{options}
			placeholder="Choose a timezone"
			bind:value
			required
		/>
	{/snippet}
</LabeledField>
