<script lang="ts">
	import { signInWithEmailAndPassword } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import { decidePortalLanding, type Engagement, type PortalSessionInfo } from '#lib/portalLanding.js';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let submitting = $state(false);
	let picker = $state<Engagement[] | null>(null);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		picker = null;
		submitting = true;
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
					resolve('/portal/engagements/[engagementId]', { engagementId: landing.engagementId })
				);
			} else {
				picker = landing.engagements;
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
	<h2>Choose an Engagement</h2>
	{#if picker.length === 0}
		<p>You don't have an Engagement yet. Ask your Practice to set one up.</p>
	{:else}
		<ul>
			{#each picker as engagement (engagement.engagementId)}
				<li>
					<a
						href={resolve('/portal/engagements/[engagementId]', {
							engagementId: engagement.engagementId
						})}
					>
						{engagement.practiceName}
					</a>
				</li>
			{/each}
		</ul>
	{/if}
{/if}
