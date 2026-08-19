<script lang="ts">
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';

	let email = $state('');
	let name = $state('');
	let error = $state('');
	let isSubmitting = $state(false);
	let acceptLink = $state('');

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		acceptLink = '';
		isSubmitting = true;
		try {
			const response = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/invitations`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email, name })
			});
			if (!response.ok) {
				error = await response.text();
				return;
			}

			const created: { inviteToken: string } = await response.json();
			acceptLink = `${location.origin}/accept-invite?token=${created.inviteToken}`;
			email = '';
			name = '';
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Invite failed';
		} finally {
			isSubmitting = false;
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
	<button type="submit" disabled={isSubmitting}>Send invite</button>
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
