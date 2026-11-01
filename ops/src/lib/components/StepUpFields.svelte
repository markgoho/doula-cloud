<script lang="ts">
	import type { FormError } from '../authRefusal.ts';
	import { STEP_UP_CODE_FIELD, STEP_UP_PASSWORD_FIELD } from '../stepUp.ts';
	import TextField from './TextField.svelte';

	// The fresh code an act's check page asks for, above its button. The
	// page owns the ErrorSummary and calls stepUp() with what these hold.
	let {
		errors,
		password = $bindable(''),
		code = $bindable('')
	}: { errors: FormError[]; password?: string; code?: string } = $props();

	const errorFor = (field: string) => errors.find((error) => error.field === field)?.message;
</script>

<fieldset>
	<legend>Confirm it is you</legend>
	<stack-l>
		<p>This act needs a new code, even though you are signed in.</p>
		<TextField
			id={STEP_UP_PASSWORD_FIELD}
			label="Password"
			type="password"
			autocomplete="current-password"
			bind:value={password}
			error={errorFor(STEP_UP_PASSWORD_FIELD)}
		/>
		<TextField
			id={STEP_UP_CODE_FIELD}
			label="Authenticator app code"
			hint="The 6-digit code your app shows for DoulaCloud operations."
			inputmode="numeric"
			autocomplete="one-time-code"
			spellcheck="false"
			bind:value={code}
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
