<script lang="ts">
	import { createUserWithEmailAndPassword } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';

	let practiceName = $state('');
	let staffName = $state('');
	let email = $state('');
	let password = $state('');
	let error = $state('');
	let submitting = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		submitting = true;
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
		} catch (err) {
			error = err instanceof Error ? err.message : 'Signup failed';
		} finally {
			submitting = false;
		}
	}
</script>

<h1>Sign up your Practice</h1>

<form onsubmit={handleSubmit}>
	<label>
		Practice name
		<input type="text" bind:value={practiceName} required />
	</label>
	<label>
		Your name
		<input type="text" bind:value={staffName} required />
	</label>
	<label>
		Email
		<input type="email" bind:value={email} required />
	</label>
	<label>
		Password
		<input type="password" bind:value={password} required minlength="6" />
	</label>
	<button type="submit" disabled={submitting}>Create Practice</button>
	{#if error}
		<p role="alert">{error}</p>
	{/if}
</form>
