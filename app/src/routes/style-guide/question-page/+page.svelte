<script lang="ts">
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import type { JourneyStep } from '#lib/components/organisms/StepRail.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): the
	 * question, the caption and every step name the Client in full, and the
	 * hint is the longest one this journey asks -- together they size both
	 * the rail and the question column.
	 */
	const steps: JourneyStep[] = [
		{
			label: 'Who Anne-Marie Ochieng-Whitfield is',
			href: '/style-guide/question-page#name',
			status: 'completed'
		},
		{
			label: 'How to reach Anne-Marie Ochieng-Whitfield',
			href: '/style-guide/question-page#reach',
			status: 'current',
			questions: [
				{
					label: 'Email address we send the portal invite to',
					href: '/style-guide/question-page#email'
				},
				{ label: 'Phone number', href: '/style-guide/question-page#phone' }
			]
		},
		{
			label: 'Where Anne-Marie Ochieng-Whitfield lives',
			href: '/style-guide/question-page#lives',
			status: 'todo'
		},
		{ label: 'Birth preferences', href: '/style-guide/question-page#prefs', status: 'todo' },
		{ label: 'Check your answers', href: '/style-guide/question-page#check', status: 'todo' }
	];

	let email = $state('');
	let month = $state('');
	let day = $state('');
	let year = $state('');

	let isLabelMode = $state(true);
	let hasError = $state(false);

	const noop = () => {};
</script>

{#snippet errorSummary()}
	<Notice
		message="There is a problem. Enter an email address in the correct format, like name@example.com."
		variant="error"
	/>
{/snippet}

{#snippet emailInput({ describedBy }: { describedBy: string | undefined })}
	<TextInput
		id="client-email"
		type="email"
		{describedBy}
		invalid={hasError}
		value={email}
		onInput={(value) => (email = value)}
	/>
{/snippet}

{#snippet dateInputs()}
	<cluster-l space="var(--space-4)">
		<LabeledField label="Month">
			{#snippet children({ id })}
				<TextInput {id} inputmode="numeric" value={month} onInput={(value) => (month = value)} />
			{/snippet}
		</LabeledField>
		<LabeledField label="Day">
			{#snippet children({ id })}
				<TextInput {id} inputmode="numeric" value={day} onInput={(value) => (day = value)} />
			{/snippet}
		</LabeledField>
		<LabeledField label="Year">
			{#snippet children({ id })}
				<TextInput {id} inputmode="numeric" value={year} onInput={(value) => (year = value)} />
			{/snippet}
		</LabeledField>
	</cluster-l>
{/snippet}

{#snippet actions()}
	<Button label="Continue to where she lives" type="submit" onClick={noop} />
	<Link href="/style-guide/question-page" label="Save and come back later" />
{/snippet}

<div class="controls">
	<cluster-l space="var(--space-3)">
		<Button
			label={isLabelMode ? 'Show the legend-as-h1 page' : 'Show the label-as-h1 page'}
			variant="secondary"
			size="sm"
			onClick={() => (isLabelMode = !isLabelMode)}
		/>
		<Button
			label={hasError ? 'Hide the error summary' : 'Show the error summary'}
			variant="secondary"
			size="sm"
			onClick={() => (hasError = !hasError)}
		/>
	</cluster-l>
</div>

{#if isLabelMode}
	<QuestionPage
		journey="Adding a Client to Highland Midwifery"
		{steps}
		backHref="/style-guide/question-page"
		errorSummary={hasError ? errorSummary : undefined}
		caption="How to reach Anne-Marie Ochieng-Whitfield"
		question={{
			as: 'label',
			text: "What is Anne-Marie Ochieng-Whitfield's email address?",
			for: 'client-email'
		}}
		hint="Optional. Without an email address Anne-Marie Ochieng-Whitfield cannot be invited to the Client portal and cannot be invoiced."
		content={emailInput}
		{actions}
	/>
{:else}
	<QuestionPage
		journey="Adding a Client to Highland Midwifery"
		{steps}
		backHref="/style-guide/question-page"
		errorSummary={hasError ? errorSummary : undefined}
		caption="Who Anne-Marie Ochieng-Whitfield is"
		question={{ as: 'legend', text: 'What is her date of birth?' }}
		hint="For example, 4 2 1990."
		content={dateInputs}
		{actions}
	/>
{/if}

<style>
	@layer components {
		/* Not part of the Template -- switches so both h1 shapes and the
		   error region can be seen without editing this file. */
		.controls {
			padding: var(--space-3) var(--space-4);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-container);
		}
	}
</style>
