<script lang="ts">
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';

	let name = $state('');
	let email = $state('');
</script>

<stack-l space="var(--space-6)">
	<h1>Labeled field</h1>

	<section>
		<h2>Default</h2>
		<LabeledField label="Client name">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput {id} {describedBy} {invalid} value={name} onInput={(value) => (name = value)} />
			{/snippet}
		</LabeledField>
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any control on this page to see its focus outline.</p>
	</section>

	<section>
		<h2>Error</h2>
		<LabeledField label="Client email" error="Enter an email address in the correct format, like name@example.com">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="email"
					value={email}
					onInput={(value) => (email = value)}
				/>
			{/snippet}
		</LabeledField>
	</section>

	<section>
		<h2>Inline orientation</h2>
		<LabeledField label="Client name" orientation="inline">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput {id} {describedBy} {invalid} value={name} onInput={(value) => (name = value)} />
			{/snippet}
		</LabeledField>
	</section>
</stack-l>
