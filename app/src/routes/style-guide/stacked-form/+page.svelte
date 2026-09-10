<script lang="ts">
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';

	const emailId = 'stacked-form-email';
	const passwordId = 'stacked-form-password';

	let email = $state('');
	let password = $state('');

	function noop() {}
</script>

<stack-l space="var(--space-6)">
	<h1>Stacked form</h1>

	<section>
		<h2>Default</h2>
		<p>
			The gap between the two fields, and between the last field and the button, is what this
			component exists for. Without it every child sits flush against the one above.
		</p>
		<StackedForm onSubmit={(event) => event.preventDefault()}>
			<LabeledField id={emailId} label="Email">
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="email"
						value={email}
						onInput={(value) => (email = value)}
						required
						autocomplete="username"
					/>
				{/snippet}
			</LabeledField>
			<LabeledField id={passwordId} label="Password" hint="Must be 6 characters or more">
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="password"
						value={password}
						onInput={(value) => (password = value)}
						required
						autocomplete="current-password"
					/>
				{/snippet}
			</LabeledField>
			<Button type="submit" label="Log in" onClick={noop} />
		</StackedForm>
	</section>
</stack-l>
