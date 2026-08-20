<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	let name = $state('');
	let email = $state('');
	let error = $state('');
	let isSubmitting = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		isSubmitting = true;
		try {
			const response = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/clients`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name, email })
			});
			if (!response.ok) {
				error = await response.text();
				return;
			}

			const created: { engagementId: string } = await response.json();
			await goto(
				resolve('/practices/[practiceId]/engagements/[engagementId]', {
					practiceId: page.params.practiceId!,
					engagementId: created.engagementId
				})
			);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to add Client';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<Heading level={1} text="Add a Client" />

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
	<Button type="submit" label="Add Client" loading={isSubmitting} />
	{#if error}
		<Notice variant="error" message={error} />
	{/if}
</form>
