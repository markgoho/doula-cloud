<script lang="ts">
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';

	let firstName = $state('Ada');
	let lastName = $state('Lovelace');
	let email = $state('ada@example.com');
	let dueDate = $state('');
	let birthPlace = $state('');

	let hasError = $state(false);

	const noop = () => {};
</script>

{#snippet intro()}
	<Text
		text="Everything here is visible to the doulas on this client's engagement. The client fills in their own birth plan later."
		tone="variant"
	/>
{/snippet}

{#snippet error()}
	<Notice message="Enter a due date before saving this client." variant="error" />
{/snippet}

{#snippet coreFields()}
	<LabeledField label="First name">
		{#snippet children({ id })}
			<TextInput {id} value={firstName} onInput={(value) => (firstName = value)} />
		{/snippet}
	</LabeledField>

	<LabeledField label="Last name">
		{#snippet children({ id })}
			<TextInput {id} value={lastName} onInput={(value) => (lastName = value)} />
		{/snippet}
	</LabeledField>

	<LabeledField label="Email">
		{#snippet children({ id })}
			<TextInput {id} type="email" value={email} onInput={(value) => (email = value)} />
		{/snippet}
	</LabeledField>

	<LabeledField label="Due date" error={hasError ? 'Enter a due date.' : undefined}>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				placeholder="2 April 2026"
				value={dueDate}
				onInput={(value) => (dueDate = value)}
			/>
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet practiceFields()}
	<Text
		text="Fields this Practice added itself, per ADR-0017. Each Practice-defined section is one more fieldset appended below the structural core."
		step="body-sm"
		tone="variant"
	/>

	<LabeledField label="Planned place of birth">
		{#snippet children({ id })}
			<TextInput
				{id}
				placeholder="Rochester General"
				value={birthPlace}
				onInput={(value) => (birthPlace = value)}
			/>
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet actions()}
	<Button label="Save client" type="submit" onClick={noop} />
	<Button label="Cancel" variant="secondary" onClick={noop} />
{/snippet}

<div class="controls">
	<Button
		label={hasError ? 'Hide the error state' : 'Show the error state'}
		variant="secondary"
		size="sm"
		onClick={() => (hasError = !hasError)}
	/>
</div>

<FormPage
	title="New client"
	{intro}
	error={hasError ? error : undefined}
	fieldsets={[
		{ legend: 'About the client', content: coreFields },
		{ legend: 'Birth preferences', content: practiceFields }
	]}
	{actions}
/>

<style>
	@layer components {
		/* Not part of the Template -- a switch so the error region can be seen
		   without editing this file. */
		.controls {
			padding: var(--space-3) var(--space-4);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-container);
		}
	}
</style>
