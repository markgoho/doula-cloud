<script lang="ts">
	import { createUserWithEmailAndPassword } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	let practiceName = $state('');
	let staffName = $state('');
	let email = $state('');
	let password = $state('');
	let error = $state('');
	let isSubmitting = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		isSubmitting = true;
		try {
			const credential = await createUserWithEmailAndPassword(getFirebaseAuth(), email, password);
			const idToken = await credential.user.getIdToken();

			const response = await apiFetch('/api/staff/signup', idToken, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ practiceName, staffName, staffEmail: email })
			});
			if (!response.ok) {
				error = await response.text();
				return;
			}

			const created: { practiceId: string } = await response.json();
			await goto(resolve('/practices/[practiceId]', { practiceId: created.practiceId }));
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Signup failed';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<h1>Sign up your Practice</h1>

<form onsubmit={handleSubmit}>
	<LabeledField label="Practice name">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				value={practiceName}
				onInput={(value) => (practiceName = value)}
				required
			/>
		{/snippet}
	</LabeledField>
	<LabeledField label="Your name">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				value={staffName}
				onInput={(value) => (staffName = value)}
				required
			/>
		{/snippet}
	</LabeledField>
	<LabeledField label="Email">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				type="email"
				value={email}
				onInput={(value) => (email = value)}
				required
			/>
		{/snippet}
	</LabeledField>
	<LabeledField label="Password">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				type="password"
				value={password}
				onInput={(value) => (password = value)}
				required
				minlength={6}
			/>
		{/snippet}
	</LabeledField>
	<Button type="submit" label="Create Practice" loading={isSubmitting} />
	{#if error}
		<Notice variant="error" message={error} />
	{/if}
</form>
