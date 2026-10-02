<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		hasError?: boolean;
	}

	/*
	 * The other state this page renders (#1638, ADR-0025): the error
	 * summary and the message below the field, which is the longer and the
	 * busier of the two. Its own subject for the continuum check and the
	 * drag surface.
	 */
	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Form page, with errors', props: { hasError: true } }
	];
</script>

<script lang="ts">
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * hyphenated double-barreled family name and an address on the
	 * Practice's own domain, which is what a real intake form holds.
	 */
	let firstName = $state('Anne-Marie');
	let lastName = $state('Ochieng-Whitfield');
	let email = $state('anne-marie.ochieng-whitfield@highland-midwifery-group.example.org');
	let dueDate = $state('');
	let birthPlace = $state('');

	let { hasError = false }: Properties = $props();

	const dueDateId = 'style-guide-form-page-due-date';

	const noop = () => {};
</script>

{#snippet intro()}
	<Text
		text="Everything here is visible to the Doulas on this Client's Engagement, including the ones added to it later. The Client fills in the birth plan directly."
		tone="variant"
	/>
{/snippet}

{#snippet errorSummary()}
	<ErrorSummary
		errors={[{ message: 'Enter a due date, like 2 April 2027', targetId: dueDateId }]}
	/>
{/snippet}

{#snippet coreFields()}
	<LabeledField label="First name, as it appears on the Contract">
		{#snippet children({ id })}
			<TextInput {id} value={firstName} onInput={(value) => (firstName = value)} />
		{/snippet}
	</LabeledField>

	<LabeledField label="Last name, as it appears on the Contract">
		{#snippet children({ id })}
			<TextInput {id} value={lastName} onInput={(value) => (lastName = value)} />
		{/snippet}
	</LabeledField>

	<LabeledField label="Email address we send the portal invite to">
		{#snippet children({ id })}
			<TextInput {id} type="email" value={email} onInput={(value) => (email = value)} />
		{/snippet}
	</LabeledField>

	<!-- Word for word the summary entry above, which is the point of the
	     pattern: two wordings for one refusal is the defect it prevents. -->
	<LabeledField
		id={dueDateId}
		label="Estimated due date"
		error={hasError ? 'Enter a due date, like 2 April 2027' : undefined}
	>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				placeholder="2 April 2027"
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
				placeholder="Rochester General Hospital Birthing Center"
				value={birthPlace}
				onInput={(value) => (birthPlace = value)}
			/>
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet actions()}
	<Button label="Save this Client and send the portal invite" type="submit" onClick={noop} />
	<Button label="Cancel" variant="secondary" onClick={noop} />
{/snippet}

<FormPage
	title="Add a Client to Highland Midwifery"
	{intro}
	errorSummary={hasError ? errorSummary : undefined}
	fieldsets={[
		{ legend: 'About the Client and how to reach the Client', content: coreFields },
		{ legend: 'Birth preferences', content: practiceFields }
	]}
	{actions}
/>
