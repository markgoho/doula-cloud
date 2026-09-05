<script lang="ts">
	/*
	 * GOV.UK's "one time passcode" pattern (ADR-0021): a single numeric
	 * text field, never split into six boxes. #606 reuses this markup at
	 * two call sites that never render together -- confirming a freshly
	 * scanned authenticator during enrolment, and the sign-in challenge on
	 * the login screen -- so it is composed once here rather than typed
	 * twice, the same reason WorkStateField wraps a field this app asks
	 * for more than once.
	 *
	 * `totpCodeRefusal` (#lib/formErrors.js) owns what a wrong or expired
	 * code is called; this owns only the control that asks for one.
	 */
	import LabeledField from './LabeledField.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';

	interface Properties {
		/**
		 * Fixed by the route where the error summary has to link to this
		 * control (#467).
		 */
		id: string;
		value: string;
		onInput: (value: string) => void;
		error?: string;
	}

	let { id, value, onInput, error }: Properties = $props();
</script>

<LabeledField {id} {error} label="Authenticator app code">
	{#snippet children({ id: controlId, describedBy, invalid })}
		<TextInput
			id={controlId}
			{describedBy}
			{invalid}
			{value}
			{onInput}
			inputmode="numeric"
			autocomplete="one-time-code"
			maxlength={6}
			required
		/>
	{/snippet}
</LabeledField>
