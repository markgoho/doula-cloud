<script lang="ts">
	import { createUserWithEmailAndPassword, signInWithEmailAndPassword, signOut } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch, apiFetchWithSession } from '#lib/api.js';
	import { decideLanding, type Membership, type SessionInfo } from '#lib/landing.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';

	const modeOptions: { value: 'signup' | 'login'; label: string }[] = [
		{ value: 'signup', label: "I'm new here -- create an account" },
		{ value: 'login', label: 'I already have an account -- log in' }
	];

	const inviteToken = page.url.searchParams.get('token') ?? '';

	let email = $state('');
	let password = $state('');
	let mode = $state<'signup' | 'login'>('signup');
	let error = $state('');
	let isSubmitting = $state(false);
	let picker = $state<Membership[] | undefined>();

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

			const acceptResponse = await apiFetch('/api/staff/accept-invite', idToken, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ inviteToken })
			});
			if (!acceptResponse.ok) {
				error = await acceptResponse.text();
				await signOut(getFirebaseAuth());
				return;
			}

			// AcceptInviteHandler already set the session cookie on its own
			// response (#145) -- no separate exchange needed, just drop the
			// JS SDK credential before the session probe reads the cookie.
			await signOut(getFirebaseAuth());

			const sessionResponse = await apiFetchWithSession('/api/staff/session');
			if (!sessionResponse.ok) {
				error = await sessionResponse.text();
				return;
			}
			const session: SessionInfo = await sessionResponse.json();
			const landing = decideLanding(session);
			if (landing.type === 'redirect') {
				await goto(resolve('/practices/[practiceId]', { practiceId: landing.practiceId }));
			} else {
				picker = landing.memberships;
			}
		} catch {
			// A clear, non-technical failure message -- not whatever Identity
			// Platform's SDK throws (e.g. "Firebase: Error
			// (auth/invalid-credential).") -- covers both account-creation
			// and sign-in errors, since this form handles both modes.
			error = 'Accept invite failed';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<h1>Accept your Staff invite</h1>

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
		<RadioGroup
			legend="Account mode"
			name="mode"
			options={modeOptions}
			value={mode}
			onChange={(value) => (mode = value)}
		/>
		<Button type="submit" label="Accept invite" loading={isSubmitting} />
		{#if error}
			<Notice variant="error" message={error} />
		{/if}
	</form>
{/if}

{#if picker}
	<h2>Choose a Practice</h2>
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
