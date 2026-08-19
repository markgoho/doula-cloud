<script lang="ts">
	import { createUserWithEmailAndPassword, signOut } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL } from '#lib/api.js';
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

			// A plain, one-off fetch: this token makes one trip and is never
			// carried around the way apiFetchWithSession's cookie is (#150
			// deleted the shared ID-token helper).
			const response = await fetch(`${apiBaseURL()}/api/staff/signup`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${idToken}` },
				body: JSON.stringify({ practiceName, staffName, staffEmail: email })
			});
			if (!response.ok) {
				error = await response.text();
				await signOut(getFirebaseAuth());
				return;
			}

			const created: { practiceId: string } = await response.json();

			// SignupHandler already set the session cookie on its own
			// response (#145) -- just drop the JS SDK credential before
			// landing (#149).
			await signOut(getFirebaseAuth());

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
