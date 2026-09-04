<script lang="ts">
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiBaseURL } from '#lib/api.js';
	import { refusalMessage, SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';

	const token = page.url.searchParams.get('token') ?? '';
	const passwordId = 'reset-password-new';

	let newPassword = $state('');
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let hasSucceeded = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errors = [];
		if (!token) {
			errors = [{ message: 'This link has expired or was already used -- ask for a new one.' }];
			return;
		}
		if (newPassword.length < 6) {
			errors = [{ message: 'Enter a password of at least 6 characters', targetId: passwordId }];
			return;
		}

		isSubmitting = true;
		try {
			const response = await fetch(`${apiBaseURL()}/api/staff/password-reset`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token, newPassword })
			});
			if (!response.ok) {
				errors = [{ message: await refusalMessage(response) }];
				return;
			}
			hasSucceeded = true;
		} catch {
			errors = [{ message: SERVICE_PROBLEM }];
		} finally {
			isSubmitting = false;
		}
	}
</script>

<PageTitle page="Reset your password" isError={errors.length > 0} />

<ErrorSummary {errors} />

<Heading level={1} variant="page" text="Reset your password" />

{#if hasSucceeded}
	<Notice
		variant="status"
		message="Your password has been reset. Every device you were signed in on has been signed out."
	/>
	<Link href={resolve('/(signed-out)/login')} label="Continue to log in" />
{:else}
	<form onsubmit={handleSubmit} novalidate>
		<LabeledField
			id={passwordId}
			label="New password"
			error={errors.find((entry) => entry.targetId === passwordId)?.message}
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="password"
					value={newPassword}
					onInput={(value) => (newPassword = value)}
					required
					autocomplete="new-password"
				/>
			{/snippet}
		</LabeledField>
		<Button type="submit" label="Reset password" loading={isSubmitting} />
	</form>
{/if}
