<script lang="ts">
	import LabeledField from '$ui/molecules/LabeledField.svelte';
	import TotpCodeField from '$ui/molecules/TotpCodeField.svelte';
	import TextInput from '$ui/atoms/TextInput.svelte';
	import type { FormError } from '../authRefusal.ts';
	import { STEP_UP_CODE_FIELD, STEP_UP_PASSWORD_FIELD } from '../stepUp.ts';

	let {
		errors,
		password = $bindable(''),
		code = $bindable('')
	}: { errors: FormError[]; password?: string; code?: string } = $props();

	const errorFor = (field: string) => errors.find((error) => error.targetId === field)?.message;
</script>

<fieldset>
	<legend>Confirm it is you</legend>
	<stack-l>
		<p>This act needs a new code, even though you are signed in.</p>
		<LabeledField id={STEP_UP_PASSWORD_FIELD} label="Password" error={errorFor(STEP_UP_PASSWORD_FIELD)}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="password"
					autocomplete="current-password"
					value={password}
					onInput={(next) => (password = next)}
				/>
			{/snippet}
		</LabeledField>
		<TotpCodeField
			id={STEP_UP_CODE_FIELD}
			value={code}
			onInput={(next) => (code = next)}
			error={errorFor(STEP_UP_CODE_FIELD)}
		/>
	</stack-l>
</fieldset>

<style>
	fieldset {
		min-inline-size: 0;
		margin: 0;
		padding: 0;
		border: 0;
	}

	legend {
		margin-block-end: var(--space-3);
		font-size: var(--text-heading-size);
		font-weight: var(--text-heading-weight);
		line-height: var(--text-heading-leading);
	}
</style>
