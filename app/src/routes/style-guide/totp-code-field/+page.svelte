<script lang="ts">
	import TotpCodeField from '#lib/components/molecules/TotpCodeField.svelte';

	let value = $state('');
	let invalidValue = $state('000000');
</script>

<stack-l space="var(--space-6)">
	<h1>Totp code field</h1>

	<section>
		<h2>Default</h2>
		<TotpCodeField id="totp-code-default" {value} onInput={(next) => (value = next)} />
	</section>

	<section>
		<h2>Refused</h2>
		<TotpCodeField
			id="totp-code-error"
			value={invalidValue}
			onInput={(next) => (invalidValue = next)}
			error="The code is not correct. Enter the 6-digit code from your authenticator app."
		/>
	</section>
</stack-l>
