<script lang="ts">
	import { signInWithEmailAndPassword, signOut } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
	import { decideLanding, type Membership, type SessionInfo } from '#lib/landing.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let isSubmitting = $state(false);
	let picker = $state<Membership[] | undefined>();

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		picker = undefined;
		isSubmitting = true;
		try {
			const credential = await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
			const idToken = await credential.user.getIdToken();

			// Exchange the Identity Platform ID token for the session cookie
			// before signing out of the JS SDK -- the Staff session probe
			// right below this authenticates by cookie, so the exchange must
			// land first (#149). A plain, one-off fetch: this token makes one
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

			const response = await apiFetchWithSession('/api/staff/session');
			if (!response.ok) {
				error = await response.text();
				return;
			}

			const session: SessionInfo = await response.json();
			const landing = decideLanding(session);
			if (landing.type === 'redirect') {
				await goto(resolve('/practices/[practiceId]', { practiceId: landing.practiceId }));
			} else {
				picker = landing.memberships;
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
	<h2>Choose a Practice</h2>
	{#if picker.length === 0}
		<p>You don't belong to any Practice yet. Ask an Owner to invite you.</p>
	{:else}
		<ul>
			{#each picker as membership (membership.practiceId)}
				<li>
					<Link
						href={resolve('/practices/[practiceId]', { practiceId: membership.practiceId })}
						label={membership.practiceName}
					/>
				</li>
			{/each}
		</ul>
	{/if}
{/if}
