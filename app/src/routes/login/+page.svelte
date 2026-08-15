<script lang="ts">
	import { signInWithEmailAndPassword } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '$lib/firebase';
	import { apiFetch } from '$lib/api';
	import { decideLanding, type Membership } from '$lib/landing';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let submitting = $state(false);
	let picker = $state<Membership[] | null>(null);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		picker = null;
		submitting = true;
		try {
			const credential = await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
			const idToken = await credential.user.getIdToken();

			const response = await apiFetch('/api/staff/session', idToken);
			if (!response.ok) {
				error = await response.text();
				return;
			}

			const session: { memberships: Membership[]; lastPracticeId: string | null } =
				await response.json();
			const landing = decideLanding(session);
			if (landing.type === 'redirect') {
				await goto(resolve('/practices/[practiceId]', { practiceId: landing.practiceId }));
			} else {
				picker = landing.memberships;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Login failed';
		} finally {
			submitting = false;
		}
	}
</script>

<h1>Log in</h1>

<form onsubmit={handleSubmit}>
	<label>
		Email
		<input type="email" bind:value={email} required />
	</label>
	<label>
		Password
		<input type="password" bind:value={password} required />
	</label>
	<button type="submit" disabled={submitting}>Log in</button>
	{#if error}
		<p role="alert">{error}</p>
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
					<a href={resolve('/practices/[practiceId]', { practiceId: membership.practiceId })}>
						{membership.practiceName}
					</a>
				</li>
			{/each}
		</ul>
	{/if}
{/if}
