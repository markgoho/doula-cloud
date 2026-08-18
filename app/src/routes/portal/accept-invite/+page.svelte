<script lang="ts">
	import { createUserWithEmailAndPassword, signInWithEmailAndPassword } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import { decidePortalLanding, type Engagement, type PortalSessionInfo } from '#lib/portalLanding.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	const inviteToken = page.url.searchParams.get('token') ?? '';

	let email = $state('');
	let password = $state('');
	let mode = $state<'signup' | 'login'>('signup');
	let error = $state('');
	let isSubmitting = $state(false);
	let picker = $state<Engagement[] | undefined>();

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		picker = undefined;
		isSubmitting = true;
		try {
			if (!inviteToken) {
				error = 'Missing invite token';
				return;
			}

			const credential =
				mode === 'signup'
					? await createUserWithEmailAndPassword(getFirebaseAuth(), email, password)
					: await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
			const idToken = await credential.user.getIdToken();

			const acceptResponse = await apiFetch('/api/portal/accept-invite', idToken, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ inviteToken })
			});
			if (!acceptResponse.ok) {
				error = await acceptResponse.text();
				return;
			}

			const sessionResponse = await apiFetch('/api/portal/session', idToken);
			if (!sessionResponse.ok) {
				error = await sessionResponse.text();
				return;
			}
			const session: PortalSessionInfo = await sessionResponse.json();
			const landing = decidePortalLanding(session);
			if (landing.type === 'redirect') {
				await goto(
					resolve('/portal/engagements/[engagementId]', { engagementId: landing.engagementId })
				);
			} else {
				picker = landing.engagements;
			}
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Accept invite failed';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<h1>Accept your portal invite</h1>

{#if !inviteToken}
	<Notice variant="error" message="Missing invite token" />
{:else}
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
					minlength={6}
				/>
			{/snippet}
		</LabeledField>
		<LabeledField label="I'm new here -- create an account" orientation="inline">
			{#snippet children({ id })}
				<input type="radio" {id} name="mode" value="signup" bind:group={mode} />
			{/snippet}
		</LabeledField>
		<LabeledField label="I already have an account -- log in" orientation="inline">
			{#snippet children({ id })}
				<input type="radio" {id} name="mode" value="login" bind:group={mode} />
			{/snippet}
		</LabeledField>
		<Button type="submit" label="Accept invite" loading={isSubmitting} />
		{#if error}
			<Notice variant="error" message={error} />
		{/if}
	</form>
{/if}

{#if picker}
	<h2>Choose an Engagement</h2>
	{#if picker.length === 0}
		<p>You don't have an Engagement yet. Ask your Practice to set one up.</p>
	{:else}
		<ul>
			{#each picker as engagement (engagement.engagementId)}
				<li>
					<Link
						href={resolve('/portal/engagements/[engagementId]', {
							engagementId: engagement.engagementId
						})}
						label={engagement.practiceName}
					/>
				</li>
			{/each}
		</ul>
	{/if}
{/if}
