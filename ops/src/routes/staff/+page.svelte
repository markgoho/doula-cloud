<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Button from '$ui/atoms/Button.svelte';
	import TextInput from '$ui/atoms/TextInput.svelte';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import LabeledField from '$ui/molecules/LabeledField.svelte';
	import type { FormError } from '#lib/authRefusal.js';
		import { findStaff, LOOKUP_EMAIL_FIELD, lookupErrors, type FoundStaff } from '#lib/staffLookup.js';

		let email = $state('');
	let errors = $state<FormError[]>([]);
	let isSending = $state(false);
	let found = $state<FoundStaff[] | undefined>();

	const errorFor = (field: string) => errors.find((error) => error.targetId === field)?.message;

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}


	async function lookUp(event: SubmitEvent) {
		event.preventDefault();
		found = undefined;
		errors = lookupErrors(email);
		if (errors.length > 0) return;
		isSending = true;
		try {
			const outcome = await findStaff(email);
			if (outcome.kind === 'signed-out') return signedOut();
			if (outcome.kind === 'ok') found = outcome.value;
			else errors = outcome.kind === 'refused' ? outcome.errors : [];
		} finally {
			isSending = false;
		}
	}
</script>

<svelte:head><title>Reset two-factor authentication – DoulaCloud operations</title></svelte:head>

{#if page.data.operator}
	<ErrorSummary {errors} />
	<h1>Reset two-factor authentication</h1>
	<p>Ask the person on the call for the email address they sign in with.</p>
	<form novalidate onsubmit={lookUp}>
		<stack-l>
			<LabeledField id={LOOKUP_EMAIL_FIELD} label="Sign-in email address" error={errorFor(LOOKUP_EMAIL_FIELD)}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="email"
						autocomplete="off"
						value={email}
						onInput={(next) => (email = next)}
					/>
				{/snippet}
			</LabeledField>
			<Button label="Find" type="submit" loading={isSending} />
		</stack-l>
	</form>
	{#if found}
		<section role="status">
			<stack-l>
				{#if found.length === 0}
					<h2>No one signs in with that email address</h2>
					<p>Check the spelling with the person. A Staff member who was deleted is not found.</p>
				{:else}
					<h2>Who signs in with {email.trim()}</h2>
					<ul class="found">
						{#each found as staff (staff.staffId)}
							<li>
								<a href={resolve('/staff/[staffId]/two-factor', { staffId: staff.staffId })}>{staff.name}</a>,
								{staff.email}.
								{staff.practiceNames.length === 0 ? 'At no Practice.' : `At ${staff.practiceNames.join(', ')}.`}
							</li>
						{/each}
					</ul>
				{/if}
			</stack-l>
		</section>
	{/if}
{/if}

<style>
	.found {
		padding-inline-start: var(--space-4);
		overflow-wrap: anywhere;
	}
</style>
