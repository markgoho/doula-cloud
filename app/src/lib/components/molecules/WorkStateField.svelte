<script lang="ts">
	import Select from '#lib/components/atoms/Select.svelte';
	import LabeledField from './LabeledField.svelte';
	import { WORK_STATE_NAMES } from '#lib/workStates.js';

	interface Properties {
		/**
		 * The full state name, e.g. "New York". Bind, then convert with
		 * workStateCode() before sending it to the API, which stores the
		 * USPS two-letter code.
		 */
		value: string;
		/**
		 * Fixed by the route where the error summary has to link to this
		 * control (#467). Left generated everywhere else.
		 */
		id?: string;
		error?: string;
	}

	const uid = $props.id();

	let { value = $bindable(''), id = uid, error }: Properties = $props();
</script>

<!--
	Asked at onboarding on every path a person joins by (#415). The reason
	is stated because we are asking a personal question for a reason that
	is not obvious, and the last sentence closes off the question it
	invites -- no, we do not want your address.

	A native <select> over 51 fixed options: the Rule of Least Power answer,
	and it needs no JavaScript to be usable.

	Composed on `LabeledField` since #467 rather than hand-rolling a label,
	a hint and their aria wiring: this field has to be able to say what is
	wrong with it like every other, and rebuilding the error message here
	would have made the second place that markup lives.
-->
<LabeledField
	{id}
	{error}
	label="Which state do you work from?"
	hint="Sales tax on your practice's credits is worked out from where its team works — so this needs to be right, and needs updating if you move. We only need the state, not your address."
>
	{#snippet children({ id: controlId, describedBy, invalid })}
		<Select
			id={controlId}
			{describedBy}
			{invalid}
			options={[...WORK_STATE_NAMES]}
			placeholder="Choose a state"
			bind:value
			required
		/>
	{/snippet}
</LabeledField>
