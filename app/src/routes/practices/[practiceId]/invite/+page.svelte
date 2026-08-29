<script lang="ts">
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import MembershipFields from '#lib/components/molecules/MembershipFields.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';

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

<!-- No name field: the Invitation carries an address and a Membership,
     and the person names herself when she accepts. -->
{#snippet who()}
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
{/snippet}

{#snippet membership()}
	<MembershipFields
		{roles}
		{employmentType}
		onRolesChange={(next) => (roles = next)}
		onEmploymentTypeChange={(next) => (employmentType = next)}
	/>
{/snippet}

{#snippet errorNotice()}
	<Notice variant="error" message={error} />
{/snippet}

{#snippet actions()}
	<Button type="submit" label="Send invite" loading={isSubmitting} />
	<!-- The accept link is not shown here and never reaches this response:
	     it goes to the invited address only, so accepting is proof she
	     controls that mailbox.

	     The confirmation sits beside the button she just pressed rather
	     than in a banner at the top, the same call `account` made; unlike
	     `account` it stays inside the Template, which owns the page
	     measure and gutters. -->
	{#if invitedAddress}
		<Notice
			variant="status"
			message="Invited. An email with a link to join is on its way to {invitedAddress}."
		/>
	{/if}
{/snippet}

<!--
	Two groups, neither named. GOV.UK's grouping test is whether the fields
	answer one question: "who are you inviting" is the address, and "what
	will she be here" is the Membership. Naming them would print two
	legends the Owner does not need -- the h1 already says what the page
	is -- so they are unnamed groups, which is the case #425 asked this
	Template to handle. Two groups rather than one because the brief puts
	28px between labelled field groups and 20px between consecutive
	fields, and `FormPage`'s outer stack is the 28px one.
-->
<form onsubmit={handleSubmit}>
	<FormPage
		title="Invite a Staff member"
		fieldsets={[{ content: who }, { content: membership }]}
		error={error ? errorNotice : undefined}
		{actions}
	/>
</form>
