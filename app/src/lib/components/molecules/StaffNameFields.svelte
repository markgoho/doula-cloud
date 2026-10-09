<script lang="ts">
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from './LabeledField.svelte';

	interface Properties {
		firstName: string;
		lastName: string;
		/**
		 * Fixed by the route where the error summary has to link to a
		 * control (#467). Left generated everywhere else.
		 */
		firstId?: string;
		lastId?: string;
		firstError?: string;
		lastError?: string;
	}

	const uid = $props.id();

	let {
		firstName = $bindable(''),
		lastName = $bindable(''),
		firstId = `${uid}-first`,
		lastId = `${uid}-last`,
		firstError,
		lastError
	}: Properties = $props();
</script>

<!--
	A Staff member's name, asked as two fields because the product
	addresses her by her first name and one field cannot supply it (#1537;
	docs/design/govuk-alignment.md, Names). The labels and the autocomplete
	tokens are GOV.UK's Names pattern for two fields.

	One component for the three places that ask it -- signup, Invitation
	acceptance and /account -- so the labels, tokens and refusals are written
	once. It renders the two fields as siblings with no wrapper, so the
	parent's stack (StackedForm, FormPage) spaces them like any other field
	and a screen moves the pair by moving this one element.
-->
<LabeledField id={firstId} label="First name" error={firstError}>
	{#snippet children({ id, describedBy, invalid })}
		<TextInput
			{id}
			{describedBy}
			{invalid}
			value={firstName}
			onInput={(value) => (firstName = value)}
			required
			autocomplete="given-name"
		/>
	{/snippet}
</LabeledField>
<LabeledField id={lastId} label="Last name" error={lastError}>
	{#snippet children({ id, describedBy, invalid })}
		<TextInput
			{id}
			{describedBy}
			{invalid}
			value={lastName}
			onInput={(value) => (lastName = value)}
			required
			autocomplete="family-name"
		/>
	{/snippet}
</LabeledField>
