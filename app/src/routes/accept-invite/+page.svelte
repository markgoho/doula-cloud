<script lang="ts">
	import { createUserWithEmailAndPassword, signInWithEmailAndPassword } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '$lib/firebase';
	import { apiFetch } from '$lib/api';
	import { decideLanding, type Membership, type SessionInfo } from '$lib/landing';

	const inviteToken = page.url.searchParams.get('token') ?? '';

	let email = $state('');
	let password = $state('');
	let mode = $state<'signup' | 'login'>('signup');
	let error = $state('');
	let submitting = $state(false);
	let picker = $state<Membership[] | null>(null);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		picker = null;
		submitting = true;
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
				return;
			}

			const sessionResponse = await apiFetch('/api/staff/session', idToken);
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
		} catch (err) {
			error = err instanceof Error ? err.message : 'Accept invite failed';
		} finally {
			submitting = false;
		}
	}
</script>

<h1>Accept your Staff invite</h1>

{#if !inviteToken}
	<p role="alert">Missing invite token</p>
{:else}
	<form onsubmit={handleSubmit}>
		<label>
			Email
			<input type="email" bind:value={email} required />
		</label>
		<label>
			Password
			<input type="password" bind:value={password} required minlength="6" />
		</label>
		<label>
			<input type="radio" name="mode" value="signup" bind:group={mode} />
			I'm new here -- create an account
		</label>
		<label>
			<input type="radio" name="mode" value="login" bind:group={mode} />
			I already have an account -- log in
		</label>
		<button type="submit" disabled={submitting}>Accept invite</button>
		{#if error}
			<p role="alert">{error}</p>
		{/if}
	</form>
{/if}

{#if picker}
	<h2>Choose a Practice</h2>
	<ul>
		{#each picker as membership (membership.practiceId)}
			<li>
				<a href={resolve('/practices/[practiceId]', { practiceId: membership.practiceId })}>
					{membership.practiceName}
				</a>
			</li>
		{/each}
	</ul>
{/if}
