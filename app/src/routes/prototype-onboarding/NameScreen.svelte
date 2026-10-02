<script lang="ts">
	/*
	 * PROTOTYPE -- #1496, screen 3: intake is one question (#1516 position
	 * 3, #1611). The given name is needed; the family name and the preferred
	 * name are optional. One button, "Save and continue": it saves the
	 * Client and opens the Start work form. No "Save and come back later".
	 *
	 * `QuestionPage` always draws the journey rail. With one question the
	 * rail reads "Step 1 of 1", which says nothing, so this screen hides it.
	 * The build (#1611) needs `QuestionPage` to draw no rail for one step.
	 */
	import Button from '#lib/components/atoms/Button.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import type { FormError } from '#lib/formErrors.js';
	import { words } from './fixtures.js';
	import { prototype, to } from './model.svelte.js';

	const givenNameId = 'intake-given-name';
	const familyNameId = 'intake-family-name';
	const preferredNameId = 'intake-preferred-name';

	let givenName = $state('');
	let familyName = $state('');
	let preferredName = $state('');
	let errors = $state<FormError[]>([]);

	let filled = prototype.fill;
	$effect(() => {
		if (prototype.fill === filled) return;
		filled = prototype.fill;
		({ givenName, familyName, preferredName } = prototype.sample.client);
	});

	function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errors = givenName.trim() === '' ? [{ message: "Enter the Client's given name", targetId: givenNameId }] : [];
		if (errors.length > 0) return;
		prototype.saveClient({
			givenName: givenName.trim(),
			familyName: familyName.trim(),
			preferredName: preferredName.trim()
		});
	}
</script>

<!-- stacked-form:ignore: #1108 -- this form wraps a Template, which stacks its own regions; the `<form>` owns the submit and arranges nothing. -->
<form class="one-question" onsubmit={handleSubmit} novalidate>
	<QuestionPage
		journey="Add a Client"
		steps={[{ label: 'Name', href: to('name'), status: 'current' }]}
		backHref={to('overview')}
		question={{ as: 'legend', text: "What is the Client's name?" }}
		hint={words.nameHint}
	>
		{#snippet errorSummary()}
			{#if errors.length > 0}
				<ErrorSummary {errors} />
			{/if}
		{/snippet}

		{#snippet content()}
			<stack-l space="var(--space-5)">
				<LabeledField id={givenNameId} label="Given name" error={errors[0]?.message}>
					{#snippet children({ id, describedBy, invalid })}
						<TextInput
							{id}
							{describedBy}
							{invalid}
							value={givenName}
							onInput={(value) => (givenName = value)}
							autocomplete="off"
						/>
					{/snippet}
				</LabeledField>
				<LabeledField id={familyNameId} label="Family name (optional)">
					{#snippet children({ id, describedBy })}
						<TextInput
							{id}
							{describedBy}
							value={familyName}
							onInput={(value) => (familyName = value)}
							autocomplete="off"
						/>
					{/snippet}
				</LabeledField>
				<LabeledField
					id={preferredNameId}
					label="Preferred name (optional)"
					hint="What the Client is called day to day, if different"
				>
					{#snippet children({ id, describedBy })}
						<TextInput
							{id}
							{describedBy}
							value={preferredName}
							onInput={(value) => (preferredName = value)}
							autocomplete="off"
						/>
					{/snippet}
				</LabeledField>
			</stack-l>
		{/snippet}

		{#snippet actions()}
			<Button type="submit" label="Save and continue" />
		{/snippet}
	</QuestionPage>
</form>

<style>
	.one-question :global(nav) {
		display: none;
	}
</style>
