<script lang="ts">
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

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

<Heading level={1} text="Invite a Staff member" />

<form onsubmit={handleSubmit}>
	<LabeledField label="Their name">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				value={name}
				onInput={(value) => (name = value)}
				required
			/>
		{/snippet}
	</LabeledField>
	<LabeledField label="Their email">
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
	<Button type="submit" label="Send invite" loading={isSubmitting} />
	{#if error}
		<Notice variant="error" message={error} />
	{/if}
</form>

{#if acceptLink}
	<Text text="Invited. There is no email sending yet, so share this link with them directly:" />
	<!-- Raw exception (#189): the link is inline literal data, not prose --
	     Text's string-only API can't carry a <code> child, and one consumer
	     doesn't justify widening it. -->
	<div><code>{acceptLink}</code></div>
{/if}
