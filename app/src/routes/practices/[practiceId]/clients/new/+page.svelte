<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { createClient } from '#lib/client.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import { SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';

	const nameId = 'new-client-name';
	const emailId = 'new-client-email';

	let name = $state('');
	let email = $state('');
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);

	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}

	// No pronouns, per #463: the domain noun until the name is known.
	function findRefusals(): FormError[] {
		const found: FormError[] = [];
		if (name.trim() === '')
			found.push({ message: "Enter the Client's name", targetId: nameId });
		if (email.trim() === '')
			found.push({ message: "Enter the Client's email address", targetId: emailId });
		return found;
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errors = [];

		const refusals = findRefusals();
		if (refusals.length > 0) {
			errors = refusals;
			return;
		}

		isSubmitting = true;
		try {
			await createClient(apiFetchWithSession, page.params.practiceId!, { givenName: name, email });

			await goto(
				resolve('/practices/[practiceId]/clients', {
					practiceId: page.params.practiceId!
				})
			);
		} catch (error_) {
			// `createClient` throws the server's own words for a refusal it
			// alone knows the reason for; anything else is ours to own.
			errors = [{ message: error_ instanceof Error && error_.message ? error_.message : SERVICE_PROBLEM }];
		} finally {
			isSubmitting = false;
		}
	}
</script>

<PageTitle page="Add a Client" isError={errors.length > 0} />

<ErrorSummary {errors} />

<Heading level={1} text="Add a Client" />

<!--
	`novalidate`: the page refuses the submit, not the browser (#467).
	`autocomplete="off"` on both fields below: this asks for the Client's
	name and email, not the signed-in doula's own, and offering her stored
	values here would be both a data-entry hazard and a privacy one on a
	shared device (#469).
-->
<form onsubmit={handleSubmit} novalidate>
	<LabeledField id={nameId} label="Their name" error={errorFor(nameId)}>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				value={name}
				onInput={(value) => (name = value)}
				required
				autocomplete="off"
			/>
		{/snippet}
	</LabeledField>
	<LabeledField id={emailId} label="Their email" error={errorFor(emailId)}>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				type="email"
				value={email}
				onInput={(value) => (email = value)}
				required
				autocomplete="off"
			/>
		{/snippet}
	</LabeledField>
	<Button type="submit" label="Add Client" loading={isSubmitting} />
</form>
