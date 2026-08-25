<script lang="ts">
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import MembershipFields from '#lib/components/molecules/MembershipFields.svelte';

	let email = $state('');
	let roles = $state<string[]>(['doula']);
	let employmentType = $state<'employee' | 'contractor'>('employee');
	let error = $state('');
	let isSubmitting = $state(false);
	let invitedAddress = $state('');

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		invitedAddress = '';
		if (roles.length === 0) {
			error = 'Choose at least one role.';
			return;
		}
		isSubmitting = true;
		try {
			const response = await apiFetchWithSession(
				`/api/practices/${page.params.practiceId}/staff/invitations`,
				{
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ email, roles, employmentType })
				}
			);
			if (!response.ok) {
				error = await response.text();
				return;
			}

			invitedAddress = email;
			email = '';
			roles = ['doula'];
			employmentType = 'employee';
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Invite failed';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<Heading level={1} text="Invite a Staff member" />

<!-- No name field: the Invitation carries an address and a Membership,
     and the person names herself when she accepts. -->
<form onsubmit={handleSubmit}>
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
	<MembershipFields
		{roles}
		{employmentType}
		onRolesChange={(next) => (roles = next)}
		onEmploymentTypeChange={(next) => (employmentType = next)}
	/>
	<Button type="submit" label="Send invite" loading={isSubmitting} />
	{#if error}
		<Notice variant="error" message={error} />
	{/if}
</form>

<!-- The accept link is not shown here and never reaches this response:
     it goes to the invited address only, so accepting is proof she
     controls that mailbox. -->
{#if invitedAddress}
	<Text text="Invited. An email with a link to join is on its way to {invitedAddress}." />
{/if}
