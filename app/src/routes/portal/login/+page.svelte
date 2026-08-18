<script lang="ts">
	import { signInWithEmailAndPassword } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import { decidePortalLanding, type Engagement, type PortalSessionInfo } from '#lib/portalLanding.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let isSubmitting = $state(false);
	let picker = $state<Engagement[] | undefined>();

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		picker = undefined;
		isSubmitting = true;
		try {
			const credential = await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
			const idToken = await credential.user.getIdToken();

			const response = await apiFetch('/api/portal/session', idToken);
			if (!response.ok) {
				error = await response.text();
				return;
			}

			const session: PortalSessionInfo = await response.json();
			const landing = decidePortalLanding(session);
			if (landing.type === 'redirect') {
				await goto(
					resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: landing.engagementId })
				);
			} else {
				picker = landing.engagements;
			}
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Login failed';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<h1>Log in</h1>

<form onsubmit={handleSubmit}>
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
			/>
		{/snippet}
	</LabeledField>
	<Button type="submit" label="Log in" loading={isSubmitting} />
	{#if error}
		<Notice variant="error" message={error} />
	{/if}
</form>

{#if picker}
	<h2>Choose an Engagement</h2>
	{#if picker.length === 0}
		<p>You don't have an Engagement yet. Ask your Practice to set one up.</p>
	{:else}
		<ul>
			{#each picker as engagement (engagement.engagementId)}
				<li>
					<Link
						href={resolve('/portal/(authenticated)/engagements/[engagementId]', {
							engagementId: engagement.engagementId
						})}
						label={engagement.practiceName}
					/>
				</li>
			{/each}
		</ul>
	{/if}
{/if}
