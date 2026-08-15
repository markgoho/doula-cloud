<script lang="ts">
	import { page } from '$app/state';
	import { getFirebaseAuth } from '$lib/firebase';
	import { apiFetch } from '$lib/api';

	let email = $state('');
	let name = $state('');
	let error = $state('');
	let submitting = $state(false);
	let acceptLink = $state('');

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		acceptLink = '';
		submitting = true;
		try {
			const user = getFirebaseAuth().currentUser;
			if (!user) {
				error = 'You must be logged in to invite staff';
				return;
			}
			const idToken = await user.getIdToken();

			const response = await apiFetch(
				`/api/practices/${page.params.practiceId}/invitations`,
				idToken,
				{
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ email, name })
				}
			);
			if (!response.ok) {
				error = await response.text();
				return;
			}

			const created: { inviteToken: string } = await response.json();
			acceptLink = `${location.origin}/accept-invite?token=${created.inviteToken}`;
			email = '';
			name = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Invite failed';
		} finally {
			submitting = false;
		}
	}
</script>

<h1>Invite a Staff member</h1>

<form onsubmit={handleSubmit}>
	<label>
		Their name
		<input type="text" bind:value={name} required />
	</label>
	<label>
		Their email
		<input type="email" bind:value={email} required />
	</label>
	<button type="submit" disabled={submitting}>Send invite</button>
	{#if error}
		<p role="alert">{error}</p>
	{/if}
</form>

{#if acceptLink}
	<p>
		Invited. There is no email sending yet, so share this link with them directly:
		<code>{acceptLink}</code>
	</p>
{/if}
