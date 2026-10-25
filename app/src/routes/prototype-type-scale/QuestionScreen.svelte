<script lang="ts">
	/*
	 * PROTOTYPE (#1926) -- a question page after a refused answer: the
	 * question at `heading-lg`, the hint and error at `body-sm`, the label
	 * and the field at `body`, the step rail and its caption beside them.
	 */
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { journeySteps, stay } from './fixtures.js';

	let { prefix }: { prefix: string } = $props();

	const givenId = $derived(`${prefix}-given-name`);
	let given = $state('');
	let family = $state('Ochieng-Whitfield');
	let preferred = $state('');

	const noop = () => {};
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={[{ message: 'Enter the given name', targetId: givenId }]} />
{/snippet}

{#snippet content()}
	<stack-l space="var(--space-5)">
		<LabeledField
			id={givenId}
			label="Given name"
			error={given === '' ? 'Enter the given name' : undefined}
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput {id} {describedBy} {invalid} value={given} onInput={(value) => (given = value)} />
			{/snippet}
		</LabeledField>
		<LabeledField id="{prefix}-family-name" label="Family name (optional)">
			{#snippet children({ id, describedBy })}
				<TextInput {id} {describedBy} value={family} onInput={(value) => (family = value)} />
			{/snippet}
		</LabeledField>
		<LabeledField
			id="{prefix}-preferred-name"
			label="Preferred name (optional)"
			hint="What the Client is called day to day, if different"
		>
			{#snippet children({ id, describedBy })}
				<TextInput {id} {describedBy} value={preferred} onInput={(value) => (preferred = value)} />
			{/snippet}
		</LabeledField>
	</stack-l>
{/snippet}

{#snippet actions()}
	<Button label="Continue" type="submit" onClick={noop} />
	<Link href={stay('later')} label="Save and come back later" />
{/snippet}

<QuestionPage
	journey="Adding a Client to Highland Midwifery"
	steps={journeySteps}
	backHref={stay('back')}
	errorSummary={given === '' ? errorSummary : undefined}
	caption="Who the Client is"
	question={{ as: 'legend', text: "What is the Client's name?" }}
	hint="Only the given name is needed to save the record. The rest can be added at any time."
	{content}
	{actions}
/>
