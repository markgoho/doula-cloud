<script lang="ts">
	import { signInWithEmailAndPassword, signOut } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
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

			// Exchange the Identity Platform ID token for the session cookie
			// before signing out of the JS SDK -- the portal session probe
			// right below this authenticates by cookie, so the exchange must
			// land first (#150). A plain, one-off fetch: this token makes one
			// trip and is never carried around the way apiFetchWithSession's
			// cookie is (#150 deleted the shared ID-token helper).
			const exchangeResponse = await fetch(`${apiBaseURL()}/api/session`, {
				method: 'POST',
				headers: { Authorization: `Bearer ${idToken}` }
			});
			if (!exchangeResponse.ok) {
				error = 'Login failed';
				await signOut(getFirebaseAuth());
				return;
			}
			await signOut(getFirebaseAuth());

			const response = await apiFetchWithSession('/api/portal/session');
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
		} catch {
			// A clear, non-technical failure message -- not whatever Identity
			// Platform's SDK throws (e.g. "Firebase: Error
			// (auth/invalid-credential).").
			error = 'Login failed';
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
